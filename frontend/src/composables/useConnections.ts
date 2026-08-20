/**
 * 连接管理器 + 会话 Tab composable
 * 负责 SQLite 连接 CRUD、快速连接、SSH 会话建立与关闭
 */

import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  AddConnection,
  CloseSession,
  ListConnections,
  SetDeleted,
  StartSession,
  UpdateConnection,
} from '../../wailsjs/go/main/App.js'
import { EMPTY_GROUPS_KEY, RECENT_KEY } from '../constants/app'
import type { Connection, SessionTab } from '../types'
import { connTitle, errText, groupName } from '../utils/format'

export function useConnections() {
  const state = reactive({
    showMgr: false,
    showAdd: false,
    closeAfterConnect: true,
    includeDeleted: false,
    groupFilter: '',
    searchText: '',
    selectedConnID: '',
    selectedGroupName: '',
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
    editingID: '',
    showAddGroup: false,
    newGroupName: '',
  })

  const recentIDs = ref<string[]>([])
  const emptyGroups = ref<string[]>([])
  const selectedQuickID = ref('')
  const collapsedGroups = reactive<Record<string, boolean>>({})
  const sessions = reactive<SessionTab[]>([])
  const activeSessionID = ref('')
  const showGroupMenu = ref(false)
  const showPassword = ref(false)

  // —— 本地存储：快速连接 & 空分组 ——

  function loadRecents() {
    try {
      const raw = localStorage.getItem(RECENT_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      recentIDs.value = Array.isArray(parsed) ? parsed.filter((id) => typeof id === 'string') : []
    } catch {
      recentIDs.value = []
    }
  }

  function saveRecents() {
    localStorage.setItem(RECENT_KEY, JSON.stringify(recentIDs.value))
  }

  function loadEmptyGroups() {
    try {
      const raw = localStorage.getItem(EMPTY_GROUPS_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      emptyGroups.value = Array.isArray(parsed)
        ? parsed.filter((g) => typeof g === 'string' && g.trim())
        : []
    } catch {
      emptyGroups.value = []
    }
  }

  function saveEmptyGroups() {
    localStorage.setItem(EMPTY_GROUPS_KEY, JSON.stringify(emptyGroups.value))
  }

  function removeEmptyGroupIfUsed(name: string) {
    const g = name.trim()
    if (!g || !emptyGroups.value.includes(g)) return
    emptyGroups.value = emptyGroups.value.filter((x) => x !== g)
    saveEmptyGroups()
  }

  function seedRecentsIfEmpty() {
    if (localStorage.getItem(RECENT_KEY) !== null) return
    const ids = state.allConnections
      .filter((c) => c.Deleted === 0 && c.Enabled === 1)
      .map((c) => c.ID)
    if (ids.length === 0) return
    recentIDs.value = ids
    saveRecents()
  }

  function rememberRecent(id: string) {
    recentIDs.value = [id, ...recentIDs.value.filter((x) => x !== id)].slice(0, 40)
    saveRecents()
  }

  function clearRecents() {
    recentIDs.value = []
    selectedQuickID.value = ''
    saveRecents()
  }

  const quickConnections = computed(() => {
    const map = new Map(
      state.allConnections
        .filter((c) => c.Deleted === 0 && c.Enabled === 1)
        .map((c) => [c.ID, c]),
    )
    return recentIDs.value.map((id) => map.get(id)).filter((c): c is Connection => Boolean(c))
  })

  const groupOptions = computed(() => {
    const set = new Set(state.allConnections.map((c) => groupName(c)))
    for (const g of emptyGroups.value) set.add(g)
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  })

  const filteredGroups = computed(() => {
    const q = state.form.groupName.trim().toLowerCase()
    if (!q) return groupOptions.value
    return groupOptions.value.filter((g) => g.toLowerCase().includes(q))
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
    const q = state.searchText.trim()
    for (const g of emptyGroups.value) {
      if (q) continue
      if (state.groupFilter && g !== state.groupFilter) continue
      if (!map.has(g)) map.set(g, [])
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]))
  })

  function setMgrMessage(text: string, type: '' | 'success' | 'error' | 'loading' = '') {
    state.mgrMessage = text
    state.mgrMessageType = type
  }

  function isTypingInForm() {
    if (state.showAdd || state.showAddGroup || state.showMgr) return true
    const el = document.activeElement
    if (!el) return false
    const tag = el.tagName
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (el as HTMLElement).isContentEditable
  }

  async function refreshList() {
    try {
      const res = await ListConnections(state.includeDeleted, '')
      state.allConnections = Array.isArray(res) ? res : []
      seedRecentsIfEmpty()
    } catch (e) {
      console.error('[BoltShell] ListConnections failed', e)
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
    state.selectedGroupName = ''
  }

  function toggleGroup(g: string) {
    collapsedGroups[g] = !collapsedGroups[g]
  }

  function selectGroup(g: string) {
    state.selectedGroupName = g
    state.selectedConnID = ''
  }

  function selectConn(id: string) {
    state.selectedConnID = id
    state.selectedGroupName = ''
  }

  function pickGroup(g: string) {
    state.form.groupName = g
    showGroupMenu.value = false
  }

  function resetForm() {
    state.editingID = ''
    state.form.name = ''
    state.form.host = ''
    state.form.port = '22'
    state.form.user = 'root'
    state.form.password = ''
    state.form.groupName = ''
    state.form.enabled = true
    showPassword.value = false
  }

  function openAddModal() {
    showGroupMenu.value = false
    resetForm()
    const g = state.selectedGroupName.trim()
    if (g && g !== '未分组') state.form.groupName = g
    state.showAdd = true
  }

  function openAddGroupModal() {
    state.newGroupName = ''
    state.showAddGroup = true
  }

  function closeAddGroupModal() {
    state.showAddGroup = false
    state.newGroupName = ''
  }

  function onSaveGroup() {
    const name = state.newGroupName.trim()
    if (!name) {
      setMgrMessage('请输入分组名称', 'error')
      return
    }
    if (name === '未分组') {
      setMgrMessage('不能使用「未分组」作为分组名', 'error')
      return
    }
    if (groupOptions.value.some((g) => g === name)) {
      setMgrMessage('分组已存在', 'error')
      return
    }
    emptyGroups.value = [...emptyGroups.value, name].sort((a, b) => a.localeCompare(b))
    saveEmptyGroups()
    state.selectedGroupName = name
    state.selectedConnID = ''
    closeAddGroupModal()
    setMgrMessage(`分组「${name}」已创建`, 'success')
  }

  function fillForm(conn: Connection) {
    state.form.name = conn.Name || ''
    state.form.host = conn.Host || ''
    state.form.port = String(conn.Port || 22)
    state.form.user = conn.User || ''
    state.form.password = conn.Password || ''
    state.form.groupName = conn.GroupName || ''
    state.form.enabled = conn.Enabled === 1
  }

  function openEditModal(conn?: Connection) {
    const target = conn || state.allConnections.find((c) => c.ID === state.selectedConnID)
    if (!target) return
    showGroupMenu.value = false
    showPassword.value = false
    state.selectedConnID = target.ID
    state.editingID = target.ID
    fillForm(target)
    state.showAdd = true
  }

  function closeAddModal() {
    state.showAdd = false
    showGroupMenu.value = false
    resetForm()
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
      if (state.editingID) {
        await UpdateConnection(
          state.editingID,
          state.form.name,
          state.form.host,
          port,
          state.form.user,
          state.form.password,
          state.form.groupName,
          state.form.enabled,
        )
      } else {
        await AddConnection(
          state.form.name,
          state.form.host,
          port,
          state.form.user,
          state.form.password,
          state.form.groupName,
          state.form.enabled,
        )
      }
      const savedGroup = state.form.groupName
      closeAddModal()
      removeEmptyGroupIfUsed(savedGroup)
      setMgrMessage('保存成功', 'success')
      await refreshList()
    } catch (e) {
      setMgrMessage(`保存失败: ${errText(e)}`, 'error')
    }
  }

  /** 建立 SSH 会话；onConnected 回调用于初始化终端/文件面板/监控 */
  async function onConnect(
    conn: Connection,
    onConnected: (sessionID: string) => Promise<void>,
  ) {
    if (conn.Enabled === 0 || conn.Deleted === 1) return
    setMgrMessage('正在连接...', 'loading')
    try {
      const sid = await StartSession(conn.ID)
      sessions.push({
        sessionID: sid,
        connID: conn.ID,
        title: connTitle(conn),
        closed: false,
      })
      activeSessionID.value = sid
      await nextTick()
      await nextTick()
      await onConnected(sid)
      rememberRecent(conn.ID)
      setMgrMessage('连接成功', 'success')
      if (state.closeAfterConnect) closeMgr()
    } catch (e) {
      setMgrMessage(`连接失败: ${errText(e)}`, 'error')
      console.error('[BoltShell] onConnect failed', e)
    }
  }

  function onConnDblClick(conn: Connection, onConnected: (sessionID: string) => Promise<void>) {
    onConnect(conn, onConnected)
  }

  async function onCloseTab(
    sessionID: string,
    onDispose: (sessionID: string) => void,
    onSwitchTerminal: () => void,
  ) {
    try {
      await CloseSession(sessionID)
    } catch (e) {
      console.error('[BoltShell] CloseSession failed', e)
    }
    onDispose(sessionID)
    const idx = sessions.findIndex((s) => s.sessionID === sessionID)
    if (idx >= 0) sessions.splice(idx, 1)
    if (activeSessionID.value === sessionID) {
      // 关掉当前 Tab 后激活相邻的那个（右边优先），而不是永远跳到最后一个
      const next = sessions[idx] ?? sessions[idx - 1]
      activeSessionID.value = next ? next.sessionID : ''
      if (activeSessionID.value) {
        await nextTick()
        onSwitchTerminal()
      }
    }
  }

  function onQuickKeydown(e: KeyboardEvent, onConnected: (conn: Connection) => void) {
    if (state.showMgr || state.showAdd) return
    if (sessions.length > 0) return
    if (e.key !== 'Enter' || !selectedQuickID.value) return
    const conn = quickConnections.value.find((c) => c.ID === selectedQuickID.value)
    if (conn) onConnected(conn)
  }

  function onMgrKeydown(e: KeyboardEvent, onConnected: (conn: Connection) => void) {
    if (!state.showMgr) return
    const tag = (e.target as HTMLElement | null)?.tagName
    const inField = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
    if (e.key === 'Escape') {
      if (state.showAdd) {
        state.showAdd = false
        return
      }
      closeMgr()
      return
    }
    if (inField) return
    if (e.key === 'Enter' && state.selectedConnID) {
      const conn = state.allConnections.find((c) => c.ID === state.selectedConnID)
      if (conn) onConnected(conn)
    }
  }

  watch(
    () => state.includeDeleted,
    () => refreshList().catch(console.error),
  )

  return {
    state,
    recentIDs,
    emptyGroups,
    selectedQuickID,
    collapsedGroups,
    sessions,
    activeSessionID,
    showGroupMenu,
    showPassword,
    quickConnections,
    groupOptions,
    filteredGroups,
    groupedConnections,
    connTitle,
    groupName,
    loadRecents,
    loadEmptyGroups,
    refreshList,
    openMgr,
    closeMgr,
    toggleGroup,
    selectGroup,
    selectConn,
    pickGroup,
    openAddModal,
    openAddGroupModal,
    closeAddGroupModal,
    onSaveGroup,
    openEditModal,
    closeAddModal,
    onToggleSelected,
    onSaveConnection,
    onConnect,
    onConnDblClick,
    onCloseTab,
    onQuickKeydown,
    onMgrKeydown,
    clearRecents,
    isTypingInForm,
  }
}
