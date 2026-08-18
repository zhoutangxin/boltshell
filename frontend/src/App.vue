<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import {
  AddConnection,
  CloseSession,
  ListConnections,
  SendSessionInput,
  SetDeleted,
  StartSession,
} from '../wailsjs/go/main/App'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'

type Connection = {
  ID: string
  Name: string
  Host: string
  Port: number
  User: string
  Password: string
  GroupName: string
  Enabled: number
  Deleted: number
  CreatedAt: number
}

type SessionTab = {
  sessionID: string
  connID: string
  title: string
}

const state = reactive({
  showMgr: false,
  showAdd: false,
  closeAfterConnect: true,
  includeDeleted: false,
  groupFilter: '',
  searchText: '',
  selectedConnID: '',
  mgrMessage: '',
  mgrMessageType: '' as '' | 'success' | 'error' | 'loading',
  allConnections: [] as Connection[],
  form: {
    name: '',
    host: '',
    port: '22',
    user: 'root',
    password: '',
    groupName: '',
    enabled: true,
  },
})

const collapsedGroups = reactive<Record<string, boolean>>({})
const sessions = reactive<SessionTab[]>([])
const activeSessionID = ref('')

const terminalHosts = new Map<string, HTMLDivElement>()
const termMap = new Map<string, { term: Terminal; fit: FitAddon; opened: boolean }>()

function setTerminalHost(sessionID: string, el: unknown) {
  if (el instanceof HTMLDivElement) terminalHosts.set(sessionID, el)
  else terminalHosts.delete(sessionID)
}

function connTitle(c: Connection) {
  if (c.Name?.trim()) return c.Name
  return `${c.Host}:${c.Port}`
}

function groupName(c: Connection) {
  return c.GroupName?.trim() || '未分组'
}

function errText(e: unknown): string {
  if (e instanceof Error && e.message) return e.message
  if (typeof e === 'string') return e
  try {
    return JSON.stringify(e)
  } catch {
    return '未知错误'
  }
}

function setMgrMessage(text: string, type: '' | 'success' | 'error' | 'loading' = '') {
  state.mgrMessage = text
  state.mgrMessageType = type
}

const groupOptions = computed(() => {
  const set = new Set(state.allConnections.map((c) => groupName(c)))
  return Array.from(set).sort((a, b) => a.localeCompare(b))
})

const filteredConnections = computed(() => {
  const q = state.searchText.trim().toLowerCase()
  return state.allConnections.filter((c) => {
    if (!state.includeDeleted && c.Deleted === 1) return false
    if (state.groupFilter && groupName(c) !== state.groupFilter) return false
    if (q) {
      const hay = `${c.Name} ${c.Host} ${c.User} ${c.GroupName}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

const groupedConnections = computed(() => {
  const map = new Map<string, Connection[]>()
  for (const c of filteredConnections.value) {
    const g = groupName(c)
    if (!map.has(g)) map.set(g, [])
    map.get(g)!.push(c)
  }
  return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]))
})

function ensureTerminal(sessionID: string) {
  const exist = termMap.get(sessionID)
  if (exist) return exist

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    theme: { background: '#0b1220' },
    scrollback: 2000,
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.onData((data) => {
    SendSessionInput(sessionID, data).catch(console.error)
  })
  termMap.set(sessionID, { term, fit, opened: false })
  return { term, fit, opened: false }
}

function openTerminal(sessionID: string) {
  const el = terminalHosts.get(sessionID)
  if (!el) return
  const entry = termMap.get(sessionID)
  if (!entry) return

  if (!entry.opened) {
    entry.term.open(el)
    entry.opened = true
  }

  let tries = 0
  const tryFit = () => {
    tries++
    const rect = el.getBoundingClientRect()
    if (rect.width < 5 || rect.height < 5) {
      if (tries < 8) requestAnimationFrame(tryFit)
      return
    }
    try {
      entry.fit.fit()
      entry.term.focus()
    } catch {
      /* ignore */
    }
  }
  requestAnimationFrame(() => requestAnimationFrame(tryFit))
}

function openOrSwitchTerminal() {
  if (!activeSessionID.value) return
  ensureTerminal(activeSessionID.value)
  openTerminal(activeSessionID.value)
}

async function refreshList() {
  try {
    const res = await ListConnections(state.includeDeleted, '')
    state.allConnections = Array.isArray(res) ? res : []
  } catch (e) {
    console.error('[ssh-go] ListConnections failed', e)
    state.allConnections = []
    setMgrMessage('获取连接列表失败', 'error')
  }
}

function openMgr() {
  state.showMgr = true
  refreshList()
}

function closeMgr() {
  state.showMgr = false
  state.selectedConnID = ''
}

function toggleGroup(g: string) {
  collapsedGroups[g] = !collapsedGroups[g]
}

function selectConn(id: string) {
  state.selectedConnID = id
}

async function onConnect(conn: Connection) {
  if (conn.Enabled === 0 || conn.Deleted === 1) return

  const existing = sessions.find((s) => s.connID === conn.ID)
  if (existing) {
    activeSessionID.value = existing.sessionID
    setMgrMessage('已切换到已有会话', 'success')
    await nextTick()
    openTerminal(existing.sessionID)
    if (state.closeAfterConnect) closeMgr()
    return
  }

  setMgrMessage('正在连接...', 'loading')
  try {
    const sid = await StartSession(conn.ID)
    sessions.push({
      sessionID: sid,
      connID: conn.ID,
      title: connTitle(conn),
    })
    activeSessionID.value = sid
    ensureTerminal(sid)
    await nextTick()
    openTerminal(sid)
    setMgrMessage('连接成功', 'success')
    if (state.closeAfterConnect) closeMgr()
  } catch (e) {
    setMgrMessage(`连接失败: ${errText(e)}`, 'error')
    console.error('[ssh-go] onConnect failed', e)
  }
}

function onConnDblClick(conn: Connection) {
  onConnect(conn)
}

async function onToggleSelected() {
  if (!state.selectedConnID) return
  const conn = state.allConnections.find((c) => c.ID === state.selectedConnID)
  if (!conn) return
  try {
    await SetDeleted(conn.ID, conn.Deleted === 0)
    await refreshList()
    if (conn.Deleted === 0) state.selectedConnID = ''
    setMgrMessage(conn.Deleted === 0 ? '已删除' : '已恢复', 'success')
  } catch (e) {
    setMgrMessage(`操作失败: ${errText(e)}`, 'error')
  }
}

async function onSaveConnection() {
  const port = parseInt(state.form.port || '0', 10) || 0
  if (!state.form.host || !state.form.user || !state.form.password) {
    setMgrMessage('缺少必填项', 'error')
    return
  }
  try {
    await AddConnection(
      state.form.name,
      state.form.host,
      port,
      state.form.user,
      state.form.password,
      state.form.groupName,
      state.form.enabled,
    )
    state.showAdd = false
    state.form.name = ''
    state.form.host = ''
    state.form.port = '22'
    state.form.user = 'root'
    state.form.password = ''
    state.form.groupName = ''
    state.form.enabled = true
    setMgrMessage('保存成功', 'success')
    await refreshList()
  } catch (e) {
    setMgrMessage(`保存失败: ${errText(e)}`, 'error')
  }
}

async function onCloseTab(sessionID: string) {
  try {
    await CloseSession(sessionID)
  } catch (e) {
    console.error('[ssh-go] CloseSession failed', e)
  }
  const entry = termMap.get(sessionID)
  if (entry) {
    entry.term.dispose()
    termMap.delete(sessionID)
  }
  terminalHosts.delete(sessionID)
  const idx = sessions.findIndex((s) => s.sessionID === sessionID)
  if (idx >= 0) sessions.splice(idx, 1)
  if (activeSessionID.value === sessionID) {
    activeSessionID.value = sessions.length ? sessions[sessions.length - 1].sessionID : ''
    if (activeSessionID.value) {
      await nextTick()
      openTerminal(activeSessionID.value)
    }
  }
}

function onMgrKeydown(e: KeyboardEvent) {
  if (!state.showMgr) return
  if (e.key === 'Escape') closeMgr()
  if (e.key === 'Enter' && state.selectedConnID) {
    const conn = state.allConnections.find((c) => c.ID === state.selectedConnID)
    if (conn) onConnect(conn)
  }
}

onMounted(async () => {
  await refreshList()

  EventsOn('terminal-output', (sessionID: string, data: string) => {
    termMap.get(sessionID)?.term.write(data)
  })

  EventsOn('terminal-closed', (sessionID: string) => {
    console.log('terminal closed', sessionID)
  })

  window.addEventListener('keydown', onMgrKeydown)
  window.addEventListener('resize', () => {
    if (!activeSessionID.value) return
    const entry = termMap.get(activeSessionID.value)
    if (!entry?.opened) return
    const el = terminalHosts.get(activeSessionID.value)
    if (!el) return
    const rect = el.getBoundingClientRect()
    if (rect.width < 5 || rect.height < 5) return
    try {
      entry.fit.fit()
    } catch {
      /* ignore */
    }
  })
})

watch(
  () => state.includeDeleted,
  () => refreshList().catch(console.error),
)

watch(activeSessionID, () => {
  nextTick().then(() => openOrSwitchTerminal())
})
</script>

<template>
  <div class="app">
    <!-- 顶部 Tab 栏 -->
    <div class="top-bar">
      <button
        class="btn-conn-mgr"
        :class="{ active: state.showMgr }"
        title="连接管理器"
        @click="openMgr"
      >
        📁
      </button>
      <div class="tabs">
        <button
          v-for="(s, i) in sessions"
          :key="s.sessionID"
          class="tab"
          :class="{ active: activeSessionID === s.sessionID }"
          @click="activeSessionID = s.sessionID"
        >
          <span class="dot" />
          <span>{{ i + 1 }} {{ s.title }}</span>
          <span class="close" @click.stop="onCloseTab(s.sessionID)">×</span>
        </button>
      </div>
    </div>

    <!-- 全屏终端区域 -->
    <div class="main-area">
      <div v-if="sessions.length === 0" class="terminal-empty">
        <span>暂无活动会话</span>
        <button class="hint-btn" @click="openMgr">打开连接管理器</button>
      </div>
      <div
        v-for="s in sessions"
        :key="s.sessionID"
        :ref="(el) => setTerminalHost(s.sessionID, el)"
        class="terminal-host"
        v-show="activeSessionID === s.sessionID"
      />
    </div>

    <!-- 连接管理器 -->
    <div v-if="state.showMgr" class="mgr-backdrop" @click.self="closeMgr">
      <div class="conn-mgr">
        <div class="mgr-titlebar">
          <span class="title">连接管理器</span>
          <button class="win-btn close" title="关闭" @click="closeMgr">✕</button>
        </div>

        <div class="mgr-toolbar">
          <button class="mgr-tool-btn" title="新建连接" @click="state.showAdd = true">➕</button>
          <button
            class="mgr-tool-btn"
            title="删除/恢复"
            :disabled="!state.selectedConnID"
            @click="onToggleSelected"
          >
            🗑
          </button>
          <span class="mgr-tool-sep" />
          <button class="mgr-tool-btn" title="刷新" @click="refreshList">🔄</button>
          <div class="mgr-search-area">
            <input v-model="state.searchText" type="text" placeholder="搜索连接..." />
            <select v-model="state.groupFilter">
              <option value="">全部</option>
              <option v-for="g in groupOptions" :key="g" :value="g">{{ g }}</option>
            </select>
            <label>
              <input type="checkbox" v-model="state.includeDeleted" />
              显示已删除
            </label>
          </div>
        </div>

        <div class="mgr-list-header">
          <span>名称</span>
          <span>主机</span>
          <span>端口</span>
          <span>用户名</span>
        </div>

        <div class="mgr-list-body">
          <div v-if="groupedConnections.length === 0" class="mgr-empty">暂无连接，点击 ➕ 新建</div>
          <template v-for="[g, items] in groupedConnections" :key="g">
            <div
              class="mgr-folder"
              :class="{ collapsed: collapsedGroups[g] }"
              @click="toggleGroup(g)"
            >
              <div class="folder-name">
                <span class="arrow">▼</span>
                <span>📁 {{ g }}</span>
                <span class="folder-count">({{ items.length }})</span>
              </div>
            </div>
            <div
              v-for="c in collapsedGroups[g] ? [] : items"
              :key="c.ID"
              class="mgr-conn"
              :class="{
                selected: state.selectedConnID === c.ID,
                deleted: c.Deleted === 1,
              }"
              @click="selectConn(c.ID)"
              @dblclick="onConnDblClick(c)"
            >
              <div class="conn-name">
                <span class="conn-icon">🖥</span>
                <span>{{ connTitle(c) }}</span>
              </div>
              <span class="col-host">{{ c.Host }}</span>
              <span class="col-port">{{ c.Port }}</span>
              <span class="col-user">{{ c.User }}</span>
            </div>
          </template>
        </div>

        <div
          class="mgr-status"
          :class="state.mgrMessageType"
          v-if="state.mgrMessage"
        >
          {{ state.mgrMessage }}
        </div>

        <div class="mgr-footer">
          <label class="close-after">
            <input type="checkbox" v-model="state.closeAfterConnect" />
            连接后关闭窗口
          </label>
        </div>
      </div>
    </div>

    <!-- 新建连接子弹窗 -->
    <div v-if="state.showAdd" class="add-backdrop" @click.self="state.showAdd = false">
      <div class="add-modal">
        <div class="add-titlebar">新建连接</div>
        <div class="add-body">
          <div class="form-grid">
            <label class="form-field">
              名称
              <input v-model="state.form.name" class="input" placeholder="可选" />
            </label>
            <label class="form-field">
              主机 *
              <input v-model="state.form.host" class="input" placeholder="192.168.1.10" />
            </label>
            <label class="form-field">
              端口
              <input v-model="state.form.port" class="input" />
            </label>
            <label class="form-field">
              用户名 *
              <input v-model="state.form.user" class="input" />
            </label>
            <label class="form-field">
              密码 *
              <input v-model="state.form.password" class="input" type="password" />
            </label>
            <label class="form-field">
              分组（文件夹）
              <input v-model="state.form.groupName" class="input" placeholder="如：aliyun" />
            </label>
            <label class="form-row">
              <input type="checkbox" v-model="state.form.enabled" />
              启用
            </label>
          </div>
          <div class="add-actions">
            <button class="btn" @click="state.showAdd = false">取消</button>
            <button class="btn btn-primary" @click="onSaveConnection">保存</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app {
  height: 100vh;
  display: flex;
  flex-direction: column;
  font-family: 'Segoe UI', 'Microsoft YaHei', Arial, sans-serif;
  overflow: hidden;
}

/* 顶部 Tab */
.top-bar {
  height: 36px;
  background: #e8eef5;
  display: flex;
  align-items: center;
  border-bottom: 1px solid #c5d0de;
  flex-shrink: 0;
}
.btn-conn-mgr {
  width: 36px;
  height: 36px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 18px;
  color: #475569;
  flex-shrink: 0;
}
.btn-conn-mgr:hover { background: #d4dde8; }
.btn-conn-mgr.active { background: #c5d5e8; }

.tabs {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  overflow-x: auto;
  flex: 1;
  height: 100%;
  padding-top: 4px;
}
.tab {
  background: #d8e3f0;
  border: 1px solid #b8c8d8;
  border-bottom: none;
  color: #334155;
  padding: 5px 12px;
  border-radius: 6px 6px 0 0;
  cursor: pointer;
  white-space: nowrap;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 30px;
}
.tab.active {
  background: #0b1220;
  color: #e5e7eb;
  border-color: #0b1220;
}
.tab .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  flex-shrink: 0;
}
.tab .close {
  opacity: 0.5;
  font-size: 13px;
}
.tab .close:hover {
  opacity: 1;
  color: #ef4444;
}

/* 全屏终端 */
.main-area {
  flex: 1;
  position: relative;
  background: #0b1220;
  overflow: hidden;
  min-height: 0;
}
.terminal-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #64748b;
  font-size: 13px;
  gap: 12px;
}
.hint-btn {
  background: #1e3a5f;
  border: 1px solid #2563eb;
  color: #93c5fd;
  padding: 8px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}
.hint-btn:hover {
  background: #2563eb;
  color: #fff;
}
.terminal-host {
  position: absolute;
  inset: 0;
  overflow: hidden;
}
.terminal-host :deep(.xterm) {
  height: 100%;
}
.terminal-host :deep(.xterm-viewport) {
  overflow-y: auto;
}

/* 连接管理器 */
.mgr-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  z-index: 200;
  display: flex;
  align-items: flex-start;
  justify-content: flex-start;
  padding: 48px 0 0 4px;
}
.conn-mgr {
  width: 720px;
  height: 520px;
  background: #f5f7fa;
  border: 1px solid #b0bec5;
  border-radius: 4px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.35);
  display: flex;
  flex-direction: column;
  color: #1e293b;
  overflow: hidden;
}
.mgr-titlebar {
  height: 32px;
  background: linear-gradient(180deg, #e8eef5, #d4dde8);
  border-bottom: 1px solid #b0bec5;
  display: flex;
  align-items: center;
  padding: 0 8px;
  flex-shrink: 0;
}
.mgr-titlebar .title {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
  color: #334155;
}
.win-btn {
  width: 24px;
  height: 20px;
  border: 1px solid #b0bec5;
  background: #e8eef5;
  cursor: pointer;
  font-size: 11px;
  color: #475569;
}
.win-btn.close:hover {
  background: #ef4444;
  color: #fff;
  border-color: #ef4444;
}

.mgr-toolbar {
  height: 36px;
  background: #eef2f7;
  border-bottom: 1px solid #d0d8e4;
  display: flex;
  align-items: center;
  padding: 0 8px;
  gap: 4px;
  flex-shrink: 0;
}
.mgr-tool-btn {
  width: 28px;
  height: 28px;
  border: 1px solid transparent;
  background: transparent;
  cursor: pointer;
  border-radius: 4px;
  font-size: 14px;
  color: #475569;
}
.mgr-tool-btn:hover:not(:disabled) {
  background: #d8e3f0;
  border-color: #b0bec5;
}
.mgr-tool-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.mgr-tool-sep {
  width: 1px;
  height: 20px;
  background: #c5d0de;
  margin: 0 4px;
}
.mgr-search-area {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}
.mgr-search-area input {
  width: 140px;
  height: 24px;
  border: 1px solid #b0bec5;
  border-radius: 3px;
  padding: 0 8px;
  font-size: 12px;
}
.mgr-search-area select {
  height: 24px;
  border: 1px solid #b0bec5;
  border-radius: 3px;
  font-size: 12px;
  padding: 0 4px;
}
.mgr-search-area label {
  font-size: 11px;
  color: #64748b;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.mgr-list-header {
  display: grid;
  grid-template-columns: 1fr 160px 60px 80px;
  background: #e8eef5;
  border-bottom: 1px solid #d0d8e4;
  font-size: 11px;
  color: #64748b;
  padding: 4px 8px;
  flex-shrink: 0;
}
.mgr-list-body {
  flex: 1;
  overflow: auto;
  background: #fff;
}
.mgr-folder {
  padding: 5px 8px;
  font-size: 12px;
  cursor: pointer;
  border-bottom: 1px solid #f0f4f8;
  user-select: none;
}
.mgr-folder:hover { background: #f0f6ff; }
.folder-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: #334155;
}
.folder-count {
  color: #94a3b8;
  font-weight: 400;
  font-size: 11px;
}
.arrow {
  font-size: 10px;
  color: #94a3b8;
  width: 12px;
  display: inline-block;
  transition: transform 0.15s;
}
.mgr-folder.collapsed .arrow {
  transform: rotate(-90deg);
}

.mgr-conn {
  display: grid;
  grid-template-columns: 1fr 160px 60px 80px;
  padding: 5px 8px 5px 28px;
  font-size: 12px;
  cursor: pointer;
  border-bottom: 1px solid #f8fafc;
  align-items: center;
  color: #334155;
}
.mgr-conn:hover { background: #f0f6ff; }
.mgr-conn.selected { background: #dbeafe; }
.mgr-conn.deleted { opacity: 0.45; }
.conn-name {
  display: flex;
  align-items: center;
  gap: 6px;
}
.conn-icon { font-size: 13px; }
.col-host,
.col-port,
.col-user {
  color: #64748b;
}

.mgr-empty {
  padding: 40px;
  text-align: center;
  color: #94a3b8;
  font-size: 13px;
}
.mgr-status {
  font-size: 11px;
  padding: 4px 8px;
  min-height: 20px;
  color: #2563eb;
}
.mgr-status.success { color: #16a34a; }
.mgr-status.error { color: #ef4444; }
.mgr-status.loading { color: #2563eb; }

.mgr-footer {
  height: 36px;
  background: #eef2f7;
  border-top: 1px solid #d0d8e4;
  display: flex;
  align-items: center;
  padding: 0 12px;
  flex-shrink: 0;
  font-size: 12px;
  color: #475569;
}
.close-after {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  margin-left: auto;
}

/* 新建连接 */
.add-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.3);
  z-index: 300;
  display: flex;
  align-items: center;
  justify-content: center;
}
.add-modal {
  width: 440px;
  background: #f5f7fa;
  border: 1px solid #b0bec5;
  border-radius: 4px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  color: #1e293b;
}
.add-titlebar {
  height: 32px;
  background: linear-gradient(180deg, #e8eef5, #d4dde8);
  border-bottom: 1px solid #b0bec5;
  display: flex;
  align-items: center;
  padding: 0 12px;
  font-size: 12px;
  font-weight: 600;
}
.add-body { padding: 16px; }
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.form-field {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 11px;
  color: #64748b;
}
.form-row {
  grid-column: span 2;
  flex-direction: row;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}
.input {
  height: 28px;
  padding: 0 8px;
  border: 1px solid #b0bec5;
  border-radius: 3px;
  font-size: 12px;
  background: #fff;
  color: #1e293b;
}
.input:focus {
  border-color: #3b82f6;
  outline: none;
}
.add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
}
.btn {
  height: 28px;
  padding: 0 16px;
  border-radius: 3px;
  font-size: 12px;
  cursor: pointer;
  border: 1px solid #b0bec5;
  background: #e8eef5;
  color: #334155;
}
.btn:hover { background: #d4dde8; }
.btn-primary {
  background: #2563eb;
  border-color: #1d4ed8;
  color: #fff;
}
.btn-primary:hover { background: #1d4ed8; }
</style>
