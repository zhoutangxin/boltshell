/**
 * 连接管理器 + 会话 Tab composable
 * 负责 SQLite 连接 CRUD、快速连接、SSH 会话建立与关闭
 */

import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  AddConnection,
  AddConnectionGroup,
  AssignUngroupedToGroup,
  CloseSession,
  DeleteConnectionGroupByName,
  ExportConnections,
  ImportConnections,
  ListConnectionGroups,
  ListConnections,
  MoveConnection,
  RenameConnectionGroupByName,
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
    editingGroupName: '',
    showExportPicker: false,
  })

  const recentIDs = ref<string[]>([])
  const dbGroups = ref<string[]>([])
  let localGroupsMigrated = false
  const selectedQuickID = ref('')
  const collapsedGroups = reactive<Record<string, boolean>>({})
  const sessions = reactive<SessionTab[]>([])
  const activeSessionID = ref('')
  const showGroupMenu = ref(false)
  const showPassword = ref(false)

  // —— 本地存储：快速连接 ——

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

  async function migrateLocalEmptyGroups() {
    if (localGroupsMigrated) return
    localGroupsMigrated = true
    try {
      const raw = localStorage.getItem(EMPTY_GROUPS_KEY)
      if (!raw) return
      const parsed = JSON.parse(raw)
      const names = Array.isArray(parsed)
        ? parsed.filter((g: unknown) => typeof g === 'string' && g.trim())
        : []
      for (const name of names as string[]) {
        try {
          await AddConnectionGroup(name.trim())
        } catch {
          // 已存在或不可用，忽略
        }
      }
      localStorage.removeItem(EMPTY_GROUPS_KEY)
    } catch {
      localStorage.removeItem(EMPTY_GROUPS_KEY)
    }
  }

  async function refreshGroups() {
    try {
      const res = await ListConnectionGroups(false)
      dbGroups.value = Array.isArray(res)
        ? res.map((g) => g.Name).filter((n) => typeof n === 'string' && n.trim())
        : []
    } catch (e) {
      console.error('[BoltShell] ListConnectionGroups failed', e)
      dbGroups.value = []
    }
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
    for (const g of dbGroups.value) set.add(g)
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
    for (const g of dbGroups.value) {
      if (q) continue
      if (state.groupFilter && g !== state.groupFilter) continue
      if (!map.has(g)) map.set(g, [])
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]))
  })

  const exportGrouped = computed(() => {
    const map = new Map<string, Connection[]>()
    for (const c of state.allConnections) {
      if (!state.includeDeleted && c.Deleted === 1) continue
      const g = groupName(c)
      if (!map.has(g)) map.set(g, [])
      map.get(g)!.push(c)
    }
    for (const g of dbGroups.value) {
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
    await migrateLocalEmptyGroups()
    try {
      const [res] = await Promise.all([ListConnections(state.includeDeleted, ''), refreshGroups()])
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
    state.editingGroupName = ''
    state.newGroupName = ''
    state.showAddGroup = true
  }

  function openEditGroupModal(g?: string) {
    const name = (g || state.selectedGroupName).trim()
    if (!name) {
      setMgrMessage('请先选择分组', 'error')
      return
    }
    state.editingGroupName = name
    state.newGroupName = name === '未分组' ? '' : name
    state.showAddGroup = true
  }

  function closeAddGroupModal() {
    state.showAddGroup = false
    state.newGroupName = ''
    state.editingGroupName = ''
  }

  async function onSaveGroup() {
    const name = state.newGroupName.trim()
    if (!name) {
      setMgrMessage('请输入分组名称', 'error')
      return
    }
    if (name === '未分组') {
      setMgrMessage('不能使用「未分组」作为分组名', 'error')
      return
    }
    const renaming = state.editingGroupName.trim()
    if (renaming !== '未分组' && !renaming && groupOptions.value.some((g) => g === name)) {
      setMgrMessage('分组已存在', 'error')
      return
    }
    if (renaming && renaming !== '未分组' && name !== renaming && groupOptions.value.some((g) => g === name)) {
      setMgrMessage('分组已存在', 'error')
      return
    }
    try {
      if (renaming === '未分组') {
        const moved = await AssignUngroupedToGroup(name)
        state.selectedGroupName = name
        setMgrMessage(moved > 0 ? `已将 ${moved} 条连接归入「${name}」` : `分组「${name}」已就绪`, 'success')
      } else if (renaming) {
        await RenameConnectionGroupByName(renaming, name)
        if (collapsedGroups[renaming] !== undefined) {
          collapsedGroups[name] = collapsedGroups[renaming]
          delete collapsedGroups[renaming]
        }
        if (state.groupFilter === renaming) state.groupFilter = name
        state.selectedGroupName = name
        setMgrMessage(`分组已重命名为「${name}」`, 'success')
      } else {
        await AddConnectionGroup(name)
        state.selectedGroupName = name
        setMgrMessage(`分组「${name}」已创建`, 'success')
      }
      state.selectedConnID = ''
      closeAddGroupModal()
      await refreshList()
    } catch (e) {
      setMgrMessage(`保存分组失败: ${errText(e)}`, 'error')
    }
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
    if (state.selectedConnID) {
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
      return
    }
    const g = state.selectedGroupName.trim()
    if (!g || g === '未分组') return
    try {
      const moved = await DeleteConnectionGroupByName(g)
      state.selectedGroupName = ''
      await refreshList()
      setMgrMessage(moved > 0 ? `已删除分组，${moved} 条连接已移至未分组` : '已删除分组', 'success')
    } catch (e) {
      setMgrMessage(`删除分组失败: ${errText(e)}`, 'error')
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
      closeAddModal()
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

  function openExportPicker() {
    state.showExportPicker = true
  }

  function closeExportPicker() {
    state.showExportPicker = false
  }

  async function onMoveConnection(id: string, groupNameValue: string) {
    const conn = state.allConnections.find((c) => c.ID === id)
    if (!conn) return
    const target = (groupNameValue || '').trim()
    const current = (conn.GroupName || '').trim()
    if (target === current) return
    try {
      await MoveConnection(id, target)
      await refreshList()
      const label = target || '未分组'
      setMgrMessage(`已移动到「${label}」`, 'success')
    } catch (e) {
      setMgrMessage(`移动失败: ${errText(e)}`, 'error')
    }
  }

  function isCancelDialog(e: unknown) {
    const t = errText(e)
    return t.includes('已取消')
  }

  async function onExportConnections(ids: string[]) {
    if (!ids?.length) {
      setMgrMessage('请选择要导出的连接', 'error')
      return
    }
    try {
      const path = await ExportConnections(ids)
      closeExportPicker()
      setMgrMessage(`已导出 ${ids.length} 条到 ${path}`, 'success')
    } catch (e) {
      if (isCancelDialog(e)) return
      setMgrMessage(`导出失败: ${errText(e)}`, 'error')
    }
  }

  async function onImportConnections() {
    try {
      const res = await ImportConnections()
      await refreshList()
      const parts = [
        res.GroupsAdded ? `新增分组 ${res.GroupsAdded}` : '',
        res.ConnectionsAdded ? `新增连接 ${res.ConnectionsAdded}` : '',
        res.ConnectionsUpdated ? `更新连接 ${res.ConnectionsUpdated}` : '',
        res.ConnectionsSkip ? `跳过 ${res.ConnectionsSkip}` : '',
      ].filter(Boolean)
      setMgrMessage(parts.length ? `导入完成：${parts.join('，')}` : '导入完成，没有变更', 'success')
    } catch (e) {
      if (isCancelDialog(e)) return
      setMgrMessage(`导入失败: ${errText(e)}`, 'error')
    }
  }

  watch(
    () => state.includeDeleted,
    () => refreshList().catch(console.error),
  )

  return {
    state,
    recentIDs,
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
    exportGrouped,
    connTitle,
    groupName,
    loadRecents,
    refreshList,
    openMgr,
    closeMgr,
    toggleGroup,
    selectGroup,
    selectConn,
    pickGroup,
    openAddModal,
    openAddGroupModal,
    openEditGroupModal,
    closeAddGroupModal,
    onSaveGroup,
    openEditModal,
    closeAddModal,
    onToggleSelected,
    onSaveConnection,
    onExportConnections,
    onImportConnections,
    openExportPicker,
    closeExportPicker,
    onMoveConnection,
    onConnect,
    onConnDblClick,
    onCloseTab,
    onQuickKeydown,
    onMgrKeydown,
    clearRecents,
    isTypingInForm,
  }
}
