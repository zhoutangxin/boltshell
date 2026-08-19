<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import {
  AddConnection,
  CloseSession,
  DownloadFromRemote,
  GetRemoteHome,
  GetSessionSysInfo,
  ListConnections,
  ListRemoteDir,
  MkdirRemote,
  OpenLocalFolder,
  PickDownloadDir,
  PickLocalFile,
  PickLocalDir,
  PickSaveFile,
  RemoveRemote,
  RenameRemote,
  ResizeSession,
  SendSessionInput,
  SetDeleted,
  StartSession,
  UpdateConnection,
  UploadToRemote,
} from '../wailsjs/go/main/App.js'
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
  closed: boolean
}

type RemoteEntry = {
  Name: string
  Path: string
  Size: number
  IsDir: boolean
  ModTime: number
  Mode: string
  Owner: string
}

type FilePaneState = {
  path: string
  home: string
  files: RemoteEntry[]
  selected: string
  loading: boolean
  status: string
}

type SysProcInfo = {
  MemKB: number
  CPUPct: number
  Command: string
}

type SysInfo = {
  CPUPercent: number
  MemTotal: number
  MemUsed: number
  SwapTotal: number
  SwapUsed: number
  DiskTotal: number
  DiskUsed: number
  DiskFree: number
  DiskPath: string
  Processes: SysProcInfo[]
}

type TransferTask = {
  id: string
  sessionID: string
  kind: 'upload' | 'download'
  fileName: string
  source: string
  dest: string
  total: number
  transferred: number
  status: 'running' | 'done' | 'error'
  error: string
  updatedAt: number
}

const DOWNLOAD_DIR_KEY = 'ssh-go-download-dir'
const downloadDir = ref(localStorage.getItem(DOWNLOAD_DIR_KEY) || '')
const transferTasks = reactive<TransferTask[]>([])
const transferPanelOpen = ref(false)

function saveDownloadDir(dir: string) {
  downloadDir.value = dir
  if (dir) localStorage.setItem(DOWNLOAD_DIR_KEY, dir)
  else localStorage.removeItem(DOWNLOAD_DIR_KEY)
}

function joinLocalPath(dir: string, name: string) {
  const sep = dir.includes('\\') ? '\\' : '/'
  return dir.replace(/[/\\]+$/, '') + sep + name
}

function transferPercent(t: TransferTask) {
  if (t.total > 0) return Math.min(100, Math.round((t.transferred / t.total) * 100))
  if (t.status === 'done') return 100
  return t.status === 'running' ? 0 : 0
}

function transferStatusText(t: TransferTask) {
  if (t.status === 'error') return t.error || '失败'
  if (t.status === 'done') return '完成'
  if (t.total > 0) return `${formatBytes(t.transferred)} / ${formatBytes(t.total)}`
  return '传输中…'
}

function upsertTransfer(ev: Record<string, unknown>) {
  const id = String(ev.TaskID ?? '')
  if (!id) return
  const prev = transferTasks.find((t) => t.id === id)
  const total = Number(ev.Total ?? 0)
  const transferred = Number(ev.Transferred ?? 0)
  const item: TransferTask = {
    id,
    sessionID: String(ev.SessionID ?? ''),
    kind: ev.Kind === 'download' ? 'download' : 'upload',
    fileName: String(ev.FileName ?? 'file'),
    source: String(ev.Source ?? ''),
    dest: String(ev.Dest ?? ''),
    total: total > 0 ? total : prev?.total ?? 0,
    transferred: transferred > 0 ? transferred : prev?.transferred ?? 0,
    status: ev.Status === 'done' ? 'done' : ev.Status === 'error' ? 'error' : 'running',
    error: String(ev.Error ?? ''),
    updatedAt: Date.now(),
  }
  if (item.status === 'done' && item.total > 0) item.transferred = item.total
  const idx = transferTasks.findIndex((t) => t.id === id)
  if (idx >= 0) transferTasks[idx] = item
  else transferTasks.unshift(item)
  if (transferTasks.length > 50) transferTasks.length = 50
  transferPanelOpen.value = true
}

function clearFinishedTransfers() {
  for (let i = transferTasks.length - 1; i >= 0; i--) {
    if (transferTasks[i].status !== 'running') transferTasks.splice(i, 1)
  }
}

async function pickDownloadDir() {
  try {
    const dir = await PickDownloadDir()
    if (dir) saveDownloadDir(dir)
  } catch (e) {
    console.error('[ssh-go] PickDownloadDir failed', e)
  }
}

async function openDownloadDir() {
  if (!downloadDir.value) {
    await pickDownloadDir()
    return
  }
  try {
    await OpenLocalFolder(downloadDir.value)
  } catch (e) {
    console.error('[ssh-go] OpenLocalFolder failed', e)
  }
}

function emptySysInfo(): SysInfo {
  return {
    CPUPercent: 0,
    MemTotal: 0,
    MemUsed: 0,
    SwapTotal: 0,
    SwapUsed: 0,
    DiskTotal: 0,
    DiskUsed: 0,
    DiskFree: 0,
    DiskPath: '/',
    Processes: [],
  }
}

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

const RECENT_KEY = 'ssh-go-quick-connect'
const EMPTY_GROUPS_KEY = 'ssh-go-empty-groups'
const recentIDs = ref<string[]>([])
const emptyGroups = ref<string[]>([])
const selectedQuickID = ref('')

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
  if (!g) return
  if (!emptyGroups.value.includes(g)) return
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
  const list = recentIDs.value.map((id) => map.get(id)).filter((c): c is Connection => Boolean(c))
  return list
})

const collapsedGroups = reactive<Record<string, boolean>>({})
const sessions = reactive<SessionTab[]>([])
const activeSessionID = ref('')
const fileBySession = reactive<Record<string, FilePaneState>>({})
const splitWrapRef = ref<HTMLElement | null>(null)
const sysInfo = ref<SysInfo>(emptySysInfo())
const sysInfoLoading = ref(false)
let sysInfoTimer: ReturnType<typeof setInterval> | null = null

const memPercent = computed(() => {
  const s = sysInfo.value
  if (!s.MemTotal) return 0
  return Math.min(100, Math.round((s.MemUsed / s.MemTotal) * 100))
})

const swapPercent = computed(() => {
  const s = sysInfo.value
  if (!s.SwapTotal) return 0
  return Math.min(100, Math.round((s.SwapUsed / s.SwapTotal) * 100))
})

const diskPercent = computed(() => {
  const s = sysInfo.value
  if (!s.DiskTotal) return 0
  return Math.min(100, Math.round((s.DiskUsed / s.DiskTotal) * 100))
})

function formatBytes(n: number): string {
  if (!n || n <= 0) return '0'
  const units = ['B', 'K', 'M', 'G', 'T']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const digits = v >= 100 || i === 0 ? 0 : 1
  return `${v.toFixed(digits)}${units[i]}`
}

async function refreshSysInfo(sessionID?: string) {
  const sid = sessionID || activeSessionID.value
  if (!sid) {
    sysInfo.value = emptySysInfo()
    return
  }
  const tab = sessions.find((s) => s.sessionID === sid)
  if (!tab || tab.closed) return
  sysInfoLoading.value = true
  try {
    const res = await GetSessionSysInfo(sid)
    if (sid !== activeSessionID.value) return
    sysInfo.value = {
      CPUPercent: res.CPUPercent ?? 0,
      MemTotal: res.MemTotal ?? 0,
      MemUsed: res.MemUsed ?? 0,
      SwapTotal: res.SwapTotal ?? 0,
      SwapUsed: res.SwapUsed ?? 0,
      DiskTotal: res.DiskTotal ?? 0,
      DiskUsed: res.DiskUsed ?? 0,
      DiskFree: res.DiskFree ?? 0,
      DiskPath: res.DiskPath || '/',
      Processes: Array.isArray(res.Processes) ? res.Processes : [],
    }
  } catch (e) {
    console.error('[ssh-go] GetSessionSysInfo failed', e)
  } finally {
    sysInfoLoading.value = false
  }
}

function startSysInfoPoll() {
  stopSysInfoPoll()
  sysInfoTimer = setInterval(() => {
    refreshSysInfo().catch(console.error)
  }, 5000)
}

function stopSysInfoPoll() {
  if (sysInfoTimer) {
    clearInterval(sysInfoTimer)
    sysInfoTimer = null
  }
}

const ui = reactive({
  sysCollapsed: false,
  fileCollapsed: false,
  filePaneHeight: 240,
  fileTab: 'files' as 'files' | 'cmd',
})

const activeTransferCount = computed(() => transferTasks.filter((t) => t.status === 'running').length)

const terminalHosts = new Map<string, HTMLDivElement>()
const termMap = new Map<string, { term: Terminal; fit: FitAddon; opened: boolean }>()
/** SSH 输出可能在 xterm 挂载前到达，先缓冲再写入 */
const outputBuffers = new Map<string, string>()
const MAX_BUFFER_CHARS = 512 * 1024

function isTypingInForm() {
  if (state.showAdd || state.showAddGroup || state.showMgr) return true
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (el as HTMLElement).isContentEditable
}

function setTerminalHost(sessionID: string, el: unknown) {
  if (el instanceof HTMLDivElement) {
    const prev = terminalHosts.get(sessionID)
    terminalHosts.set(sessionID, el)
    // 函数 ref 每次重绘都会进来；同一节点不要再 open/focus，否则输入框焦点会被抢走
    if (prev === el) return
    if (sessionID === activeSessionID.value) {
      nextTick(() => openTerminal(sessionID, 0, false))
    }
  } else {
    terminalHosts.delete(sessionID)
  }
}

function writeToTerminal(sessionID: string, data: string) {
  if (!data) return
  if (!termMap.has(sessionID)) ensureTerminal(sessionID)
  const entry = termMap.get(sessionID)
  if (entry?.opened) {
    entry.term.write(data)
    return
  }
  let buf = (outputBuffers.get(sessionID) ?? '') + data
  if (buf.length > MAX_BUFFER_CHARS) {
    buf = buf.slice(buf.length - MAX_BUFFER_CHARS)
  }
  outputBuffers.set(sessionID, buf)
}

function syncTerminalSize(sessionID: string) {
  const entry = termMap.get(sessionID)
  if (!entry?.opened) return
  const cols = entry.term.cols
  const rows = entry.term.rows
  if (cols > 0 && rows > 0) {
    ResizeSession(sessionID, cols, rows).catch(() => {})
  }
}

function flushTerminalBuffer(sessionID: string) {
  const pending = outputBuffers.get(sessionID)
  if (!pending) return
  const entry = termMap.get(sessionID)
  if (entry?.opened) {
    entry.term.write(pending)
    outputBuffers.delete(sessionID)
  }
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

function ensureFileState(sessionID: string): FilePaneState {
  if (!fileBySession[sessionID]) {
    fileBySession[sessionID] = {
      path: '/',
      home: '/',
      files: [],
      selected: '',
      loading: false,
      status: '',
    }
  }
  return fileBySession[sessionID]
}

const activeFileState = computed(() => {
  if (!activeSessionID.value) return null
  return fileBySession[activeSessionID.value] ?? null
})

function formatFileSize(n: number) {
  if (n < 0) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function formatModTime(ts: number) {
  if (!ts) return '-'
  const d = new Date(ts * 1000)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}/${p(d.getMonth() + 1)}/${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

const fileTreePaths = computed(() => {
  const st = activeFileState.value
  if (!st) return ['/']
  const paths = ['/']
  const parts = st.path.split('/').filter(Boolean)
  let acc = ''
  for (const part of parts) {
    acc += `/${part}`
    paths.push(acc)
  }
  return paths
})

async function initFilePane(sessionID: string) {
  const st = ensureFileState(sessionID)
  try {
    const home = await GetRemoteHome(sessionID)
    st.home = home || '/'
    st.path = home || '/'
    await refreshRemoteFiles(sessionID)
  } catch (e) {
    st.status = errText(e)
  }
}

async function refreshRemoteFiles(sessionID?: string) {
  const sid = sessionID || activeSessionID.value
  if (!sid) return
  const st = ensureFileState(sid)
  st.loading = true
  try {
    st.files = await ListRemoteDir(sid, st.path)
    st.status = '就绪'
  } catch (e) {
    st.status = errText(e)
  } finally {
    st.loading = false
  }
}

function selectRemoteFile(name: string) {
  const st = activeFileState.value
  if (!st) return
  st.selected = name
}

async function navigateRemote(target: string) {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st) return
  if (target === '..') {
    const parts = st.path.split('/').filter(Boolean)
    parts.pop()
    st.path = parts.length ? `/${parts.join('/')}` : '/'
  } else {
    st.path = target
  }
  st.selected = ''
  await refreshRemoteFiles(sid)
}

async function openRemoteEntry(name: string) {
  const st = activeFileState.value
  if (!st || !activeSessionID.value) return
  if (name === '..') {
    await navigateRemote('..')
    return
  }
  const entry = st.files.find((f) => f.Name === name)
  if (entry?.IsDir) {
    st.path = entry.Path
    st.selected = ''
    await refreshRemoteFiles(activeSessionID.value)
  }
}

async function remoteGoUp() {
  await navigateRemote('..')
}

async function remoteMkdir() {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st) return
  const name = prompt('新建文件夹名称', 'new-folder')
  if (!name?.trim()) return
  const remotePath = st.path === '/' ? `/${name.trim()}` : `${st.path}/${name.trim()}`
  try {
    await MkdirRemote(sid, remotePath)
    st.status = '已创建文件夹'
    await refreshRemoteFiles(sid)
  } catch (e) {
    st.status = errText(e)
  }
}

async function remoteDelete() {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st || !st.selected || st.selected === '..') return
  const entry = st.files.find((f) => f.Name === st.selected)
  if (!entry) return
  if (!confirm(`删除 ${entry.Name}？`)) return
  try {
    await RemoveRemote(sid, entry.Path)
    st.selected = ''
    st.status = '已删除'
    await refreshRemoteFiles(sid)
  } catch (e) {
    st.status = errText(e)
  }
}

async function remoteRename() {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st || !st.selected || st.selected === '..') return
  const entry = st.files.find((f) => f.Name === st.selected)
  if (!entry) return
  const newName = prompt('重命名为', entry.Name)
  if (!newName?.trim() || newName === entry.Name) return
  const dir = st.path === '/' ? '' : st.path
  const newPath = `${dir}/${newName.trim()}`.replace('//', '/')
  try {
    await RenameRemote(sid, entry.Path, newPath)
    st.selected = ''
    st.status = '已重命名'
    await refreshRemoteFiles(sid)
  } catch (e) {
    st.status = errText(e)
  }
}

async function remoteUpload(pickDir = false) {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st) return
  try {
    const local = pickDir ? await PickLocalDir() : await PickLocalFile()
    if (!local) return
    const base = local.split(/[/\\]/).pop() || (pickDir ? 'folder' : 'upload')
    const remotePath = st.path === '/' ? `/${base}` : `${st.path}/${base}`
    st.status = pickDir ? '上传文件夹中…' : '上传中…'
    transferPanelOpen.value = true
    await UploadToRemote(sid, local, remotePath)
    st.status = pickDir ? '文件夹上传完成' : '上传完成'
    await refreshRemoteFiles(sid)
  } catch (e) {
    st.status = errText(e)
  }
}

async function remoteDownload() {
  const sid = activeSessionID.value
  const st = activeFileState.value
  if (!sid || !st || !st.selected) return
  const entry = st.files.find((f) => f.Name === st.selected)
  if (!entry) return
  try {
    let local = ''
    if (entry.IsDir) {
      let baseDir = downloadDir.value
      if (!baseDir) {
        baseDir = await PickDownloadDir()
      }
      if (!baseDir) return
      local = joinLocalPath(baseDir, entry.Name)
    } else if (downloadDir.value) {
      local = joinLocalPath(downloadDir.value, entry.Name)
    } else {
      local = await PickSaveFile(entry.Name)
    }
    if (!local) return
    st.status = entry.IsDir ? '下载文件夹中…' : '下载中…'
    transferPanelOpen.value = true
    await DownloadFromRemote(sid, entry.Path, local)
    st.status = entry.IsDir ? '文件夹下载完成' : '下载完成'
  } catch (e) {
    st.status = errText(e)
  }
}

const canDownloadRemote = computed(() => {
  const st = activeFileState.value
  if (!st?.selected) return false
  return Boolean(st.files.find((f) => f.Name === st.selected))
})

function startSplitDrag(e: MouseEvent) {
  const wrap = splitWrapRef.value
  if (!wrap) return
  const startY = e.clientY
  const startH = ui.filePaneHeight
  const onMove = (ev: MouseEvent) => {
    const delta = startY - ev.clientY
    const maxH = wrap.clientHeight - 120
    ui.filePaneHeight = Math.max(100, Math.min(maxH, startH + delta))
    if (activeSessionID.value) {
      const entry = termMap.get(activeSessionID.value)
      if (entry?.opened) {
        try {
          entry.fit.fit()
        } catch {
          /* ignore */
        }
      }
    }
  }
  const onUp = () => {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
  }
  document.body.style.cursor = 'row-resize'
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

function setMgrMessage(text: string, type: '' | 'success' | 'error' | 'loading' = '') {
  state.mgrMessage = text
  state.mgrMessageType = type
}

const groupOptions = computed(() => {
  const set = new Set(state.allConnections.map((c) => groupName(c)))
  for (const g of emptyGroups.value) set.add(g)
  return Array.from(set).sort((a, b) => a.localeCompare(b))
})

const showGroupMenu = ref(false)
const showPassword = ref(false)
const filteredGroups = computed(() => {
  const q = state.form.groupName.trim().toLowerCase()
  if (!q) return groupOptions.value
  return groupOptions.value.filter((g) => g.toLowerCase().includes(q))
})

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
  if (g && g !== '未分组') {
    state.form.groupName = g
  }
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
  const exists = groupOptions.value.some((g) => g === name)
  if (exists) {
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

function ensureTerminal(sessionID: string) {
  const exist = termMap.get(sessionID)
  if (exist) return exist

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    cols: 120,
    rows: 32,
    theme: {
      background: '#0b1220',
      foreground: '#e5e7eb',
      cursor: '#e5e7eb',
    },
    scrollback: 2000,
  })
  const fit = new FitAddon()
  term.loadAddon(fit)
  term.onData((data) => {
    SendSessionInput(sessionID, data).catch(console.error)
  })
  term.onResize(({ cols, rows }) => {
    if (cols > 0 && rows > 0) {
      ResizeSession(sessionID, cols, rows).catch(() => {})
    }
  })
  termMap.set(sessionID, { term, fit, opened: false })
  return { term, fit, opened: false }
}

function openTerminal(sessionID: string, attempt = 0, wantFocus = false) {
  const el = terminalHosts.get(sessionID)
  if (!el) {
    if (attempt < 24) requestAnimationFrame(() => openTerminal(sessionID, attempt + 1, wantFocus))
    return
  }
  const entry = termMap.get(sessionID)
  if (!entry) return

  if (!entry.opened) {
    entry.term.open(el)
    entry.opened = true
    flushTerminalBuffer(sessionID)
  }

  let tries = 0
  const tryFit = () => {
    tries++
    const rect = el.getBoundingClientRect()
    if (rect.width < 5 || rect.height < 5) {
      if (tries < 24) requestAnimationFrame(tryFit)
      return
    }
    try {
      entry.fit.fit()
      entry.term.refresh(0, entry.term.rows - 1)
      flushTerminalBuffer(sessionID)
      syncTerminalSize(sessionID)
      if (wantFocus && sessionID === activeSessionID.value && !isTypingInForm()) {
        entry.term.focus()
      }
    } catch {
      /* ignore */
    }
  }
  requestAnimationFrame(() => requestAnimationFrame(tryFit))
}

function openOrSwitchTerminal() {
  if (!activeSessionID.value) return
  ensureTerminal(activeSessionID.value)
  openTerminal(activeSessionID.value, 0, true)
}

async function refreshList() {
  try {
    const res = await ListConnections(state.includeDeleted, '')
    state.allConnections = Array.isArray(res) ? res : []
    seedRecentsIfEmpty()
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

async function onConnect(conn: Connection) {
  if (conn.Enabled === 0 || conn.Deleted === 1) return

  setMgrMessage('正在连接...', 'loading')
  try {
    const sid = await StartSession(conn.ID)
    ensureTerminal(sid)
    sessions.push({
      sessionID: sid,
      connID: conn.ID,
      title: connTitle(conn),
      closed: false,
    })
    activeSessionID.value = sid
    await nextTick()
    await nextTick()
    openTerminal(sid, 0, true)
    rememberRecent(conn.ID)
    setMgrMessage('连接成功', 'success')
    initFilePane(sid).catch(console.error)
    refreshSysInfo(sid).catch(console.error)
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
  outputBuffers.delete(sessionID)
  const idx = sessions.findIndex((s) => s.sessionID === sessionID)
  if (idx >= 0) sessions.splice(idx, 1)
  delete fileBySession[sessionID]
  if (activeSessionID.value === sessionID) {
    activeSessionID.value = sessions.length ? sessions[sessions.length - 1].sessionID : ''
    if (activeSessionID.value) {
      await nextTick()
      openTerminal(activeSessionID.value, 0, true)
    }
  }
}

function onQuickKeydown(e: KeyboardEvent) {
  if (state.showMgr || state.showAdd) return
  if (sessions.length > 0) return
  if (e.key !== 'Enter' || !selectedQuickID.value) return
  const conn = quickConnections.value.find((c) => c.ID === selectedQuickID.value)
  if (conn) onConnect(conn)
}

function onMgrKeydown(e: KeyboardEvent) {
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
    if (conn) onConnect(conn)
  }
}

function parseEventArgs(args: unknown[]): [string, string] {
  if (args.length >= 2) return [String(args[0] ?? ''), String(args[1] ?? '')]
  const first = args[0]
  if (Array.isArray(first) && first.length >= 1) {
    return [String(first[0] ?? ''), String(first[1] ?? '')]
  }
  if (first && typeof first === 'object' && 'sessionID' in (first as object)) {
    const o = first as { sessionID?: unknown; data?: unknown }
    return [String(o.sessionID ?? ''), String(o.data ?? '')]
  }
  return [String(first ?? ''), '']
}

function markSessionClosed(sessionID: string) {
  const tab = sessions.find((s) => s.sessionID === sessionID)
  if (!tab) return
  tab.closed = true
  if (!tab.title.includes('(已断开)')) {
    tab.title += ' (已断开)'
  }
}

onMounted(async () => {
  EventsOn('terminal-output', (...args: unknown[]) => {
    const [sessionID, data] = parseEventArgs(args)
    console.debug('[ssh-go] terminal-output', sessionID, data?.length)
    if (sessionID) writeToTerminal(sessionID, data)
  })

  EventsOn('terminal-closed', (...args: unknown[]) => {
    const [sessionID] = parseEventArgs(args)
    console.debug('[ssh-go] terminal-closed', sessionID)
    if (sessionID) markSessionClosed(sessionID)
  })

  EventsOn('transfer-update', (...args: unknown[]) => {
    const ev = args[0]
    if (ev && typeof ev === 'object') upsertTransfer(ev as Record<string, unknown>)
  })

  await refreshList()
  loadRecents()
  loadEmptyGroups()
  seedRecentsIfEmpty()

  window.addEventListener('keydown', onMgrKeydown)
  window.addEventListener('keydown', onQuickKeydown)
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

  startSysInfoPoll()
})

onUnmounted(() => {
  stopSysInfoPoll()
})

watch(
  () => state.includeDeleted,
  () => refreshList().catch(console.error),
)

watch(activeSessionID, () => {
  nextTick().then(() => openOrSwitchTerminal())
  const sid = activeSessionID.value
  if (sid && fileBySession[sid]) {
    refreshRemoteFiles(sid).catch(console.error)
  }
  refreshSysInfo(sid).catch(console.error)
})
</script>

<template>
  <div class="app">
    <!-- 最左侧导航栏 -->
    <aside class="left-rail">
      <button
        class="btn-conn-mgr"
        :class="{ active: state.showMgr }"
        title="连接管理器"
        @click="openMgr"
      >
        📁
      </button>
    </aside>

    <aside v-if="sessions.length > 0 && !ui.sysCollapsed" class="sys-panel">
      <div class="sys-head">系统信息</div>
      <div class="sys-body">
        <div class="metric">
          <div class="metric-label">
            <span>CPU</span>
            <span class="metric-val">{{ sysInfoLoading && !sysInfo.MemTotal ? '…' : `${Math.round(sysInfo.CPUPercent)}%` }}</span>
          </div>
          <div class="bar">
            <div class="bar-fill cpu" :style="{ width: `${Math.min(100, sysInfo.CPUPercent)}%` }"></div>
          </div>
        </div>
        <div class="metric">
          <div class="metric-label">
            <span>内存</span>
            <span class="metric-val">
              {{
                sysInfo.MemTotal
                  ? `${formatBytes(sysInfo.MemUsed)} / ${formatBytes(sysInfo.MemTotal)}`
                  : sysInfoLoading
                    ? '…'
                    : '—'
              }}
            </span>
          </div>
          <div class="bar">
            <div class="bar-fill mem" :style="{ width: `${memPercent}%` }"></div>
          </div>
        </div>
        <div class="metric">
          <div class="metric-label">
            <span>Swap</span>
            <span class="metric-val">{{ sysInfo.SwapTotal ? `${swapPercent}%` : sysInfoLoading ? '…' : '0%' }}</span>
          </div>
          <div class="bar">
            <div class="bar-fill swap" :style="{ width: `${swapPercent}%` }"></div>
          </div>
        </div>
        <table v-if="sysInfo.Processes.length" class="proc-table">
          <thead>
            <tr>
              <th>内存</th>
              <th>CPU</th>
              <th>命令</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(p, idx) in sysInfo.Processes" :key="idx">
              <td class="num">{{ formatBytes(p.MemKB * 1024) }}</td>
              <td class="num">{{ p.CPUPct.toFixed(1) }}</td>
              <td class="cmd" :title="p.Command">{{ p.Command }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="sysInfo.DiskTotal" class="disk-item">
          <div class="disk-head">
            <span class="path">{{ sysInfo.DiskPath || '/' }}</span>
            <span class="pct">{{ diskPercent }}%</span>
          </div>
          <div class="size">可用 {{ formatBytes(sysInfo.DiskFree) }} / 总计 {{ formatBytes(sysInfo.DiskTotal) }}</div>
          <div class="bar disk-bar">
            <div class="bar-fill disk" :style="{ width: `${diskPercent}%` }"></div>
          </div>
        </div>
      </div>
    </aside>

    <div class="workspace">
      <div class="top-bar">
        <div class="tabs">
          <button
            v-for="(s, i) in sessions"
            :key="s.sessionID"
            class="tab"
            :class="{ active: activeSessionID === s.sessionID, disconnected: s.closed }"
            @click="activeSessionID = s.sessionID"
          >
            <span class="dot" :class="{ off: s.closed }" />
            <span>{{ i + 1 }} {{ s.title }}</span>
            <span class="close" @click.stop="onCloseTab(s.sessionID)">×</span>
          </button>
        </div>
        <div v-if="sessions.length" class="tab-actions">
          <button type="button" title="文件传输" class="transfer-toggle" @click="transferPanelOpen = !transferPanelOpen">
            ⇅
            <span v-if="activeTransferCount" class="transfer-badge">{{ activeTransferCount }}</span>
          </button>
          <button type="button" title="折叠系统信息" @click="ui.sysCollapsed = !ui.sysCollapsed">◧</button>
          <button type="button" title="折叠文件面板" @click="ui.fileCollapsed = !ui.fileCollapsed">▤</button>
        </div>
      </div>

      <div ref="splitWrapRef" class="split-wrap">
        <div class="main-area">
          <div v-if="sessions.length === 0" class="quick-panel">
            <div class="quick-header">
              <span>快速连接</span>
              <button class="quick-clear" type="button" @click="clearRecents">清空</button>
            </div>
            <div class="quick-list">
              <div
                v-for="c in quickConnections"
                :key="c.ID"
                class="quick-row"
                :class="{ selected: selectedQuickID === c.ID }"
                @click="selectedQuickID = c.ID"
                @dblclick="onConnect(c)"
              >
                <span class="quick-icon">🖥</span>
                <span class="quick-name">{{ connTitle(c) }}</span>
                <span class="quick-path">/{{ groupName(c) === '未分组' ? '' : groupName(c) }}</span>
                <span class="quick-user">{{ c.User }}</span>
              </div>
              <div v-if="quickConnections.length === 0" class="quick-empty">
                暂无连接，点击左侧 📁 打开连接管理器添加
              </div>
            </div>
          </div>
          <div
            v-for="s in sessions"
            :key="s.sessionID"
            :ref="(el) => setTerminalHost(s.sessionID, el)"
            class="terminal-host"
            v-show="activeSessionID === s.sessionID"
          />
        </div>

        <div
          v-if="sessions.length && activeSessionID && !ui.fileCollapsed"
          class="splitter"
          @mousedown.prevent="startSplitDrag"
        />

        <div
          v-if="sessions.length && activeSessionID && !ui.fileCollapsed"
          class="file-pane"
          :style="{ height: ui.filePaneHeight + 'px' }"
        >
          <div class="file-tabs">
            <div
              class="file-tab"
              :class="{ active: ui.fileTab === 'files' }"
              @click="ui.fileTab = 'files'"
            >文件</div>
            <div
              class="file-tab"
              :class="{ active: ui.fileTab === 'cmd' }"
              @click="ui.fileTab = 'cmd'"
            >命令</div>
          </div>

          <template v-if="ui.fileTab === 'files' && activeFileState">
            <div class="file-toolbar">
              <button class="ft-btn" type="button" @click="remoteGoUp">⬆ 上级</button>
              <button class="ft-btn" type="button" @click="refreshRemoteFiles()">🔄 刷新</button>
              <span class="ft-sep" />
              <button class="ft-btn primary" type="button" @click="remoteUpload(false)">⬆ 上传</button>
              <button class="ft-btn" type="button" @click="remoteUpload(true)">📁 上传文件夹</button>
              <button class="ft-btn" type="button" :disabled="!canDownloadRemote" @click="remoteDownload">⬇ 下载</button>
              <button class="ft-btn" type="button" @click="remoteMkdir">📁 新建文件夹</button>
              <button class="ft-btn" type="button" :disabled="!activeFileState.selected" @click="remoteDelete">🗑 删除</button>
              <button class="ft-btn" type="button" :disabled="!activeFileState.selected" @click="remoteRename">✏ 重命名</button>
              <span class="ft-sep" />
              <span class="ft-tag">SFTP</span>
              <div class="path-bar" :title="activeFileState.path || '/'">
                <span class="path-display">{{ activeFileState.path || '/' }}</span>
              </div>
            </div>
            <div class="file-body">
              <div class="dir-tree">
                <div
                  v-for="p in fileTreePaths"
                  :key="p"
                  class="tree-node"
                  :class="{ active: activeFileState.path === p }"
                  :style="{ paddingLeft: 8 + (p.split('/').filter(Boolean).length) * 12 + 'px' }"
                  @click="navigateRemote(p)"
                >
                  <span class="tree-arrow">▾</span>📁 {{ p === '/' ? '/' : p.split('/').pop() }}
                </div>
              </div>
              <div class="file-list-wrap">
                <div class="file-list-header">
                  <span>名称</span><span>大小</span><span>类型</span><span>修改时间</span><span>权限</span><span>用户</span>
                </div>
                <div class="file-list-body">
                  <div v-if="activeFileState.loading" class="file-loading">加载中…</div>
                  <div
                    v-if="activeFileState.path !== '/'"
                    class="file-row dir"
                    @dblclick="navigateRemote('..')"
                  >
                    <div class="fname">📁 <span>..</span></div>
                    <span class="muted">-</span><span class="muted">上级</span><span class="muted">-</span><span class="muted">-</span><span class="muted">-</span>
                  </div>
                  <div
                    v-for="f in activeFileState.files"
                    :key="f.Path"
                    class="file-row"
                    :class="{ dir: f.IsDir, selected: activeFileState.selected === f.Name }"
                    @click="selectRemoteFile(f.Name)"
                    @dblclick="openRemoteEntry(f.Name)"
                  >
                    <div class="fname">{{ f.IsDir ? '📁' : '📄' }} <span>{{ f.Name }}</span></div>
                    <span class="muted">{{ f.IsDir ? '-' : formatFileSize(f.Size) }}</span>
                    <span class="muted">{{ f.IsDir ? '文件夹' : '文件' }}</span>
                    <span class="muted">{{ formatModTime(f.ModTime) }}</span>
                    <span class="muted">{{ f.Mode }}</span>
                    <span class="muted">{{ f.Owner }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="file-status">
              <span>协议: SFTP（与 SSH 同连接）</span>
              <span>{{ activeFileState.status || '就绪' }}</span>
            </div>
          </template>

          <div v-else class="cmd-panel active">
            <div class="cmd-hint">批量命令（后续版本支持多机执行）</div>
            <textarea readonly>systemctl status sshd
df -h</textarea>
          </div>
        </div>
      </div>
    </div>

    <!-- 右侧文件传输面板（FinalShell 风格） -->
    <aside v-if="sessions.length > 0 && transferPanelOpen" class="transfer-panel">
      <div class="transfer-head">
        <span>文件传输</span>
        <div class="transfer-head-actions">
          <button type="button" title="清除已完成" @click="clearFinishedTransfers">清除</button>
          <button type="button" title="关闭" @click="transferPanelOpen = false">×</button>
        </div>
      </div>
      <div class="transfer-dir-row">
        <span class="transfer-dir-label">下载:</span>
        <span class="transfer-dir-path" :title="downloadDir || '未设置默认目录'" @click="pickDownloadDir">
          {{ downloadDir || '点击选择目录' }}
        </span>
        <button type="button" class="transfer-icon-btn" title="选择目录" @click="pickDownloadDir">📁</button>
        <button type="button" class="transfer-icon-btn" title="打开文件夹" :disabled="!downloadDir" @click="openDownloadDir">📂</button>
      </div>
      <div class="transfer-list">
        <div v-if="transferTasks.length === 0" class="transfer-empty">暂无传输任务，上传或下载文件后在此查看进度</div>
        <div
          v-for="t in transferTasks"
          :key="t.id"
          class="transfer-item"
          :class="t.status"
        >
          <div class="transfer-item-top">
            <span class="transfer-kind" :class="t.kind">{{ t.kind === 'upload' ? '上传' : '下载' }}</span>
            <span class="transfer-name" :title="t.fileName">{{ t.fileName }}</span>
            <span class="transfer-pct">{{ transferPercent(t) }}%</span>
          </div>
          <div class="transfer-progress">
            <div
              class="transfer-progress-bar"
              :class="t.kind"
              :style="{ width: `${transferPercent(t)}%` }"
            ></div>
          </div>
          <div class="transfer-meta">{{ transferStatusText(t) }}</div>
        </div>
      </div>
    </aside>

    <!-- 连接管理器 -->
    <div v-if="state.showMgr" class="mgr-backdrop" @click.self="closeMgr">
      <div class="conn-mgr">
        <div class="mgr-titlebar">
          <span class="title">连接管理器</span>
          <button class="win-btn close" title="关闭" @click="closeMgr">✕</button>
        </div>

        <div class="mgr-toolbar">
          <button class="mgr-tool-btn" title="新建连接" @click="openAddModal">➕</button>
          <button class="mgr-tool-btn" title="新增分组" @click="openAddGroupModal">📁</button>
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
          <span class="col-action">操作</span>
        </div>

        <div class="mgr-list-body">
          <div v-if="groupedConnections.length === 0" class="mgr-empty">暂无连接，点击 ➕ 新建</div>
          <template v-for="[g, items] in groupedConnections" :key="g">
            <div
              class="mgr-folder"
              :class="{
                collapsed: collapsedGroups[g],
                selected: state.selectedGroupName === g,
              }"
              @click="selectGroup(g)"
            >
              <div class="folder-name">
                <span class="arrow" @click.stop="toggleGroup(g)">▼</span>
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
                <span class="conn-title">{{ connTitle(c) }}</span>
              </div>
              <span class="col-host">{{ c.Host }}</span>
              <span class="col-port">{{ c.Port }}</span>
              <span class="col-user">{{ c.User }}</span>
              <button
                class="row-edit-btn"
                type="button"
                title="编辑"
                @click.stop="openEditModal(c)"
              >
                ✏
              </button>
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

    <!-- 新增分组 -->
    <div v-if="state.showAddGroup" class="add-backdrop" @click.self="closeAddGroupModal">
      <div class="add-modal group-modal">
        <div class="add-titlebar">
          <span>新增分组</span>
          <button class="add-close" type="button" @click="closeAddGroupModal">×</button>
        </div>
        <div class="add-body">
          <label class="form-field">
            <span class="field-label">分组名称</span>
            <input
              v-model="state.newGroupName"
              class="input"
              placeholder="例如：公司、测试环境"
              @keydown.enter.prevent="onSaveGroup"
            />
          </label>
          <div class="add-actions">
            <button class="btn" type="button" @click="closeAddGroupModal">取消</button>
            <button class="btn btn-primary" type="button" @click="onSaveGroup">保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 新建连接子弹窗 -->
    <div v-if="state.showAdd" class="add-backdrop" @click.self="closeAddModal">
      <div class="add-modal">
        <div class="add-titlebar">
          <span>{{ state.editingID ? '编辑连接' : '新建连接' }}</span>
          <button class="add-close" type="button" @click="closeAddModal">×</button>
        </div>
        <div class="add-body">
          <div class="form-grid">
            <label class="form-field">
              <span class="field-label">名称</span>
              <input v-model="state.form.name" class="input" placeholder="可选，默认使用主机" />
            </label>
            <label class="form-field">
              <span class="field-label">主机 <span class="req">*</span></span>
              <input v-model="state.form.host" class="input" placeholder="192.168.1.10" />
            </label>
            <label class="form-field">
              <span class="field-label">端口</span>
              <input v-model="state.form.port" class="input" />
            </label>
            <label class="form-field">
              <span class="field-label">用户名 <span class="req">*</span></span>
              <input v-model="state.form.user" class="input" />
            </label>
            <label class="form-field">
              <span class="field-label">密码 <span class="req">*</span></span>
              <div class="pwd-wrap">
                <input
                  v-model="state.form.password"
                  class="input"
                  :type="showPassword ? 'text' : 'password'"
                  placeholder="必填"
                  autocomplete="off"
                />
                <button
                  class="pwd-toggle"
                  type="button"
                  tabindex="-1"
                  :title="showPassword ? '隐藏密码' : '显示密码'"
                  @click.stop="showPassword = !showPassword"
                >
                  {{ showPassword ? '🙈' : '👁' }}
                </button>
              </div>
            </label>
            <div class="form-field">
              <span class="field-label">分组（文件夹）</span>
              <div class="combo">
                <input
                  v-model="state.form.groupName"
                  class="input"
                  placeholder="输入新分组，或点选已有"
                  autocomplete="off"
                  @focus="showGroupMenu = true"
                  @input="showGroupMenu = true"
                />
                <button
                  class="combo-arrow"
                  type="button"
                  tabindex="-1"
                  @click.stop="showGroupMenu = !showGroupMenu"
                >
                  ▾
                </button>
                <div v-if="showGroupMenu" class="combo-menu">
                  <div
                    v-for="g in filteredGroups"
                    :key="g"
                    class="combo-item"
                    :class="{ active: state.form.groupName === g }"
                    @mousedown.prevent="pickGroup(g)"
                  >
                    {{ g }}
                  </div>
                  <div v-if="filteredGroups.length === 0" class="combo-empty">
                    无匹配分组，将作为新分组保存
                  </div>
                </div>
              </div>
            </div>
            <label class="form-row">
              <input type="checkbox" v-model="state.form.enabled" />
              启用
            </label>
          </div>
          <div class="add-actions">
            <button class="btn" type="button" @click="closeAddModal">取消</button>
            <button class="btn btn-primary" type="button" @click="onSaveConnection">保存</button>
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
  flex-direction: row;
  text-align: left;
  font-family: var(--font-ui);
  overflow: hidden;
  --font-ui: 'Segoe UI', 'Microsoft YaHei UI', 'PingFang SC', system-ui, sans-serif;
  --font-mono: 'Cascadia Mono', 'JetBrains Mono', Consolas, 'Courier New', monospace;
  --ui-bg: #eef2f6;
  --ui-surface: #ffffff;
  --ui-border: #c8d3df;
  --ui-border-light: #e2e8f0;
  --ui-text: #1e293b;
  --ui-text-secondary: #475569;
  --ui-text-muted: #94a3b8;
  --ui-accent: #2563eb;
  --ui-accent-soft: #eff6ff;
  --cpu-color: #16a34a;
  --mem-color: #2563eb;
  --swap-color: #7c3aed;
  --disk-color: #0284c7;
  --term-bg: #0b1220;
  --fs-xs: 12px;
  --fs-sm: 13px;
  --fs-base: 14px;
  --fs-md: 15px;
}

.left-rail {
  width: 40px;
  background: var(--ui-bg);
  border-right: 1px solid var(--ui-border);
  display: flex;
  flex-direction: column;
  align-items: stretch;
  flex-shrink: 0;
}
.btn-conn-mgr {
  width: 40px;
  height: 36px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 18px;
  color: #475569;
}
.btn-conn-mgr:hover { background: #d4dde8; }
.btn-conn-mgr.active { background: #c5d5e8; }

.workspace {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--term-bg);
}

/* 左侧系统信息 */
.sys-panel {
  width: 240px;
  background: var(--ui-surface);
  border-right: 1px solid var(--ui-border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  box-shadow: 1px 0 0 rgba(15, 23, 42, 0.04);
}
.sys-head {
  height: 32px;
  background: linear-gradient(180deg, #f8fafc 0%, #eef2f7 100%);
  border-bottom: 1px solid var(--ui-border-light);
  display: flex;
  align-items: center;
  padding: 0 10px;
  font-size: var(--fs-sm);
  color: var(--ui-text-secondary);
  font-weight: 600;
  letter-spacing: 0.02em;
}
.sys-body {
  flex: 1;
  overflow: auto;
  padding: 10px 10px 12px;
  font-size: var(--fs-sm);
  color: var(--ui-text);
}
.metric { margin-bottom: 12px; }
.metric-label {
  color: var(--ui-text-muted);
  margin-bottom: 4px;
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  font-size: var(--fs-xs);
  font-weight: 500;
  letter-spacing: 0.03em;
}
.metric-val {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--ui-text);
  font-size: var(--fs-sm);
  font-family: var(--font-mono);
}
.bar {
  height: 8px;
  background: #e8edf3;
  border-radius: 4px;
  overflow: hidden;
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.06);
}
.bar-fill {
  height: 100%;
  min-width: 0;
  border-radius: 4px;
  transition: width 0.35s ease;
}
.bar-fill.cpu { background: linear-gradient(90deg, #22c55e, var(--cpu-color)); }
.bar-fill.mem { background: linear-gradient(90deg, #60a5fa, var(--mem-color)); }
.bar-fill.swap { background: linear-gradient(90deg, #a78bfa, var(--swap-color)); }
.bar-fill.disk { background: linear-gradient(90deg, #38bdf8, var(--disk-color)); }
.proc-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--ui-border-light);
  font-size: var(--fs-xs);
}
.proc-table th {
  text-align: left;
  color: var(--ui-text-muted);
  font-weight: 600;
  padding: 4px 0 6px;
  font-size: var(--fs-xs);
  letter-spacing: 0.02em;
}
.proc-table th:nth-child(2),
.proc-table td.num:nth-child(2) { text-align: right; padding-right: 6px; }
.proc-table td {
  padding: 4px 0;
  color: var(--ui-text-secondary);
  vertical-align: middle;
}
.proc-table td.num {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  font-size: var(--fs-xs);
  color: var(--ui-text);
  white-space: nowrap;
}
.proc-table td.cmd {
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ui-text-secondary);
  font-size: var(--fs-xs);
}
.proc-table tbody tr:hover td { background: #f8fafc; }
.disk-item {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--ui-border-light);
}
.disk-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2px;
}
.disk-item .path {
  color: var(--ui-text);
  font-weight: 600;
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
}
.disk-head .pct {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  font-weight: 600;
  color: var(--disk-color);
}
.disk-item .size {
  color: var(--ui-text-muted);
  font-size: var(--fs-xs);
  margin-bottom: 5px;
}
.disk-bar { margin-top: 2px; }
.sys-hint { color: var(--ui-text-muted); font-size: var(--fs-xs); line-height: 1.4; margin-top: 8px; }

.split-wrap {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.tab-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  padding-right: 6px;
  flex-shrink: 0;
}
.tab-actions button {
  width: 26px;
  height: 26px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: #64748b;
  border-radius: 4px;
  font-size: 13px;
}
.tab-actions button:hover { background: #d4dde8; }
.tab-actions .transfer-toggle {
  position: relative;
  font-size: 14px;
  font-weight: 700;
}
.transfer-badge {
  position: absolute;
  top: 2px;
  right: 2px;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  border-radius: 7px;
  background: #2563eb;
  color: #fff;
  font-size: 10px;
  line-height: 14px;
  font-weight: 700;
}

/* 右侧文件传输面板 */
.transfer-panel {
  width: 300px;
  background: var(--ui-surface);
  border-left: 1px solid var(--ui-border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  box-shadow: -1px 0 0 rgba(15, 23, 42, 0.04);
}
.transfer-head {
  height: 34px;
  background: linear-gradient(180deg, #f8fafc 0%, #eef2f7 100%);
  border-bottom: 1px solid var(--ui-border-light);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px;
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--ui-text-secondary);
  flex-shrink: 0;
}
.transfer-head-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.transfer-head-actions button {
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--ui-text-muted);
  font-size: var(--fs-xs);
  padding: 2px 6px;
  border-radius: 4px;
}
.transfer-head-actions button:hover { background: #e2e8f0; color: var(--ui-text); }
.transfer-head-actions button:last-child { font-size: 16px; padding: 0 4px; }
.transfer-dir-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--ui-border-light);
  background: #fafbfc;
  flex-shrink: 0;
}
.transfer-dir-label {
  font-size: var(--fs-sm);
  color: var(--ui-text-secondary);
  flex-shrink: 0;
}
.transfer-dir-path {
  flex: 1;
  font-size: var(--fs-xs);
  font-family: var(--font-mono);
  color: var(--ui-accent);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}
.transfer-dir-path:hover { text-decoration: underline; }
.transfer-icon-btn {
  width: 28px;
  height: 28px;
  border: 1px solid var(--ui-border-light);
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  flex-shrink: 0;
}
.transfer-icon-btn:hover:not(:disabled) { background: var(--ui-accent-soft); border-color: #bfdbfe; }
.transfer-icon-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.transfer-list {
  flex: 1;
  overflow: auto;
  padding: 8px 10px;
  min-height: 120px;
}
.transfer-empty {
  color: var(--ui-text-muted);
  font-size: var(--fs-sm);
  line-height: 1.5;
  padding: 16px 8px;
  text-align: center;
}
.transfer-item {
  border: 1px solid var(--ui-border-light);
  border-radius: 6px;
  padding: 8px 10px;
  margin-bottom: 8px;
  background: #fff;
}
.transfer-item.running { border-color: #bfdbfe; background: #f8fbff; }
.transfer-item.done { border-color: #bbf7d0; background: #f0fdf4; }
.transfer-item.error { border-color: #fecaca; background: #fef2f2; }
.transfer-item-top {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}
.transfer-kind {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 3px;
  flex-shrink: 0;
}
.transfer-kind.upload { background: #dbeafe; color: #1d4ed8; }
.transfer-kind.download { background: #dcfce7; color: #15803d; }
.transfer-name {
  flex: 1;
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--ui-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.transfer-pct {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  font-weight: 700;
  color: var(--ui-text-secondary);
  flex-shrink: 0;
}
.transfer-progress {
  height: 6px;
  background: #e2e8f0;
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 4px;
}
.transfer-progress-bar {
  height: 100%;
  border-radius: 3px;
  transition: width 0.25s ease;
  min-width: 0;
}
.transfer-progress-bar.upload { background: linear-gradient(90deg, #60a5fa, #2563eb); }
.transfer-progress-bar.download { background: linear-gradient(90deg, #4ade80, #16a34a); }
.transfer-meta {
  font-size: var(--fs-xs);
  color: var(--ui-text-muted);
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.transfer-item.error .transfer-meta { color: #dc2626; }

/* 顶部 Tab */
.top-bar {
  height: 38px;
  background: var(--ui-bg);
  display: flex;
  align-items: center;
  justify-content: flex-start;
  border-bottom: 1px solid var(--ui-border);
  flex-shrink: 0;
}

.tabs {
  display: flex;
  align-items: flex-end;
  justify-content: flex-start;
  gap: 2px;
  overflow-x: auto;
  flex: 1;
  height: 100%;
  padding: 4px 0 0 6px;
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
  font-size: var(--fs-base);
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
}
.tab.active {
  background: var(--term-bg);
  color: #e5e7eb;
  border-color: var(--term-bg);
}
.tab .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  flex-shrink: 0;
}
.tab .dot.off {
  background: #94a3b8;
}
.tab.disconnected {
  opacity: 0.75;
}
.tab .close {
  opacity: 0.5;
  font-size: 13px;
}
.tab .close:hover {
  opacity: 1;
  color: #ef4444;
}

/* 终端 + 文件分割 */
.main-area {
  flex: 1;
  position: relative;
  background: var(--term-bg);
  overflow: hidden;
  min-height: 120px;
}
.splitter {
  height: 6px;
  background: #1e293b;
  cursor: row-resize;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-top: 1px solid #334155;
  border-bottom: 1px solid #334155;
}
.splitter::after {
  content: '';
  width: 40px;
  height: 2px;
  background: #64748b;
  border-radius: 1px;
}
.splitter:hover { background: rgba(37, 99, 235, 0.2); }

/* SFTP 文件面板 */
.file-pane {
  background: var(--ui-surface);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  border-top: 1px solid #334155;
  min-height: 100px;
  font-family: var(--font-ui);
}
.file-tabs {
  height: 36px;
  background: var(--ui-bg);
  border-bottom: 1px solid var(--ui-border-light);
  display: flex;
  align-items: flex-end;
  padding: 0 10px;
  gap: 2px;
  flex-shrink: 0;
}
.file-tab {
  padding: 6px 16px;
  font-size: var(--fs-base);
  color: var(--ui-text-muted);
  cursor: pointer;
  border-radius: 6px 6px 0 0;
  margin-bottom: -1px;
  transition: color 0.15s, background 0.15s;
  user-select: none;
}
.file-tab:hover { color: var(--ui-text-secondary); background: rgba(255, 255, 255, 0.6); }
.file-tab.active {
  background: var(--ui-surface);
  color: var(--ui-text);
  font-weight: 600;
  border: 1px solid var(--ui-border-light);
  border-bottom-color: var(--ui-surface);
}
.file-toolbar {
  height: 40px;
  background: var(--ui-surface);
  border-bottom: 1px solid var(--ui-border-light);
  display: flex;
  align-items: center;
  padding: 0 10px;
  gap: 6px;
  flex-shrink: 0;
}
.ft-btn {
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--ui-border-light);
  background: #fafbfc;
  border-radius: 5px;
  font-size: var(--fs-sm);
  font-weight: 500;
  color: var(--ui-text-secondary);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
  white-space: nowrap;
}
.ft-btn:hover:not(:disabled) {
  background: var(--ui-accent-soft);
  border-color: #bfdbfe;
  color: var(--ui-accent);
}
.ft-btn:disabled { opacity: 0.38; cursor: not-allowed; }
.ft-btn.primary {
  background: var(--ui-accent);
  border-color: var(--ui-accent);
  color: #fff;
  font-weight: 600;
}
.ft-btn.primary:hover:not(:disabled) {
  background: #1d4ed8;
  border-color: #1d4ed8;
  color: #fff;
}
.ft-sep { width: 1px; height: 20px; background: var(--ui-border-light); margin: 0 2px; flex-shrink: 0; }
.ft-tag {
  font-size: var(--fs-xs);
  font-weight: 600;
  color: var(--ui-accent);
  padding: 3px 7px;
  background: var(--ui-accent-soft);
  border-radius: 4px;
  letter-spacing: 0.04em;
  flex-shrink: 0;
}
.path-bar {
  flex: 1;
  margin-left: 4px;
  height: 30px;
  border: 1px solid var(--ui-border-light);
  border-radius: 5px;
  background: #f8fafc;
  display: flex;
  align-items: center;
  padding: 0 10px;
  font-size: var(--fs-base);
  font-family: var(--font-mono);
  overflow: hidden;
  min-width: 0;
}
.path-display {
  color: var(--ui-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path-crumb {
  color: var(--ui-accent);
  cursor: pointer;
  white-space: nowrap;
  font-weight: 500;
}
.path-crumb:hover { text-decoration: underline; }
.path-sep { color: var(--ui-text-muted); margin: 0 1px; }
.file-body { flex: 1; display: flex; min-height: 0; background: var(--ui-surface); }
.dir-tree {
  width: 176px;
  border-right: 1px solid var(--ui-border-light);
  overflow: auto;
  font-size: var(--fs-base);
  flex-shrink: 0;
  background: #fafbfc;
}
.tree-node {
  padding: 6px 10px;
  cursor: pointer;
  color: var(--ui-text-secondary);
  user-select: none;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background 0.12s;
}
.tree-node:hover { background: var(--ui-accent-soft); color: var(--ui-text); }
.tree-node.active {
  background: #dbeafe;
  color: #1d4ed8;
  font-weight: 600;
  border-right: 2px solid var(--ui-accent);
}
.tree-arrow { font-size: 10px; color: var(--ui-text-muted); margin-right: 3px; }
.file-list-wrap { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.file-list-header {
  display: grid;
  grid-template-columns: minmax(120px, 1.5fr) 80px 76px 136px 100px 56px;
  background: #f1f5f9;
  border-bottom: 1px solid var(--ui-border-light);
  font-size: var(--fs-sm);
  font-weight: 600;
  color: var(--ui-text-muted);
  padding: 7px 10px;
  letter-spacing: 0.02em;
  position: sticky;
  top: 0;
  z-index: 1;
}
.file-list-body { flex: 1; overflow: auto; }
.file-loading { padding: 20px; color: var(--ui-text-muted); font-size: var(--fs-base); }
.file-row {
  display: grid;
  grid-template-columns: minmax(120px, 1.5fr) 80px 76px 136px 100px 56px;
  padding: 6px 10px;
  font-size: var(--fs-base);
  cursor: pointer;
  border-bottom: 1px solid #f1f5f9;
  align-items: center;
  color: var(--ui-text);
  transition: background 0.1s;
}
.file-row:nth-child(even) { background: #fafbfc; }
.file-row:hover { background: var(--ui-accent-soft) !important; }
.file-row.selected { background: #dbeafe !important; }
.file-row.dir .fname { color: #1e40af; font-weight: 600; }
.fname { display: flex; align-items: center; gap: 6px; overflow: hidden; min-width: 0; }
.fname span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.muted {
  color: var(--ui-text-muted);
  font-size: var(--fs-sm);
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-status {
  height: 28px;
  background: #f8fafc;
  border-top: 1px solid var(--ui-border-light);
  padding: 0 12px;
  font-size: var(--fs-sm);
  color: var(--ui-text-muted);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}
.file-status span:last-child {
  font-family: var(--font-mono);
  color: var(--ui-text-secondary);
}
.cmd-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 12px;
  gap: 8px;
  background: #fff;
}
.cmd-panel textarea {
  flex: 1;
  min-height: 60px;
  font-family: Consolas, monospace;
  font-size: 12px;
  border: 1px solid #dbe3ee;
  border-radius: 6px;
  padding: 8px;
  resize: none;
}
.cmd-hint { font-size: 12px; color: #64748b; }
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
.quick-panel {
  position: absolute;
  inset: 0;
  background: #fff;
  display: flex;
  flex-direction: column;
  color: #1e293b;
}
.quick-header {
  height: 36px;
  background: #eceff3;
  border-bottom: 1px solid #d8dee8;
  display: flex;
  align-items: center;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}
.quick-clear {
  margin-left: auto;
  height: 24px;
  padding: 0 10px;
  border: 1px solid #c5d0de;
  background: #fff;
  border-radius: 4px;
  font-size: 12px;
  color: #334155;
  cursor: pointer;
}
.quick-clear:hover { background: #f8fafc; }
.quick-list {
  flex: 1;
  overflow: auto;
}
.quick-row {
  display: grid;
  grid-template-columns: 28px minmax(160px, 1.4fr) minmax(120px, 1fr) 100px;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  font-size: 13px;
  cursor: pointer;
  border-bottom: 1px solid #f1f5f9;
}
.quick-row:hover { background: #f8fafc; }
.quick-row.selected { background: #dbeafe; }
.quick-icon { color: #475569; text-align: center; }
.quick-name { color: #0f172a; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.quick-path { color: #334155; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.quick-user { color: #334155; }
.quick-empty {
  padding: 48px 16px;
  text-align: center;
  color: #94a3b8;
  font-size: 13px;
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
  padding-left: 4px;
}
.terminal-host :deep(.xterm-viewport) {
  overflow-y: auto;
}

/* 连接管理器 */
.mgr-backdrop {
  position: fixed;
  inset: 0;
  left: 40px;
  background: rgba(0, 0, 0, 0.35);
  z-index: 200;
  display: flex;
  align-items: flex-start;
  justify-content: flex-start;
  padding: 0;
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

.mgr-list-header,
.mgr-conn {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150px 56px 80px 36px;
  column-gap: 8px;
  align-items: center;
  padding: 5px 12px 5px 12px;
  box-sizing: border-box;
}
.mgr-list-header {
  background: #e8eef5;
  border-bottom: 1px solid #d0d8e4;
  font-size: 11px;
  color: #64748b;
  flex-shrink: 0;
}
.mgr-list-header .col-action {
  text-align: center;
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
.mgr-folder.selected { background: #dbeafe; }
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
  cursor: pointer;
  flex-shrink: 0;
}
.arrow:hover { color: #475569; }
.mgr-folder.collapsed .arrow {
  transform: rotate(-90deg);
}

.mgr-conn {
  font-size: 12px;
  cursor: pointer;
  border-bottom: 1px solid #f8fafc;
  color: #334155;
}
.mgr-conn:hover { background: #f0f6ff; }
.mgr-conn.selected { background: #dbeafe; }
.mgr-conn.deleted { opacity: 0.45; }
.conn-name {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding-left: 16px;
}
.conn-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conn-icon { font-size: 13px; flex-shrink: 0; }
.col-host,
.col-port,
.col-user {
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.col-port { text-align: right; }
.row-edit-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  border-radius: 4px;
  font-size: 13px;
  line-height: 1;
  opacity: 0.35;
}
.mgr-conn:hover .row-edit-btn,
.mgr-conn.selected .row-edit-btn {
  opacity: 1;
}
.row-edit-btn:hover {
  background: #e2e8f0;
  color: #1d4ed8;
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
  background: rgba(15, 23, 42, 0.45);
  z-index: 300;
  display: flex;
  align-items: center;
  justify-content: center;
}
.add-modal {
  width: 520px;
  max-width: calc(100vw - 32px);
  box-sizing: border-box;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.28);
  color: #1e293b;
  overflow: visible;
}
.add-modal.group-modal {
  width: 360px;
}
.add-modal.group-modal .add-actions {
  margin-top: 14px;
  padding-top: 0;
  border-top: none;
}
.add-titlebar {
  height: 44px;
  background: #fff;
  border-bottom: 1px solid #eef2f7;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}
.add-close {
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: #94a3b8;
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  border-radius: 6px;
}
.add-close:hover {
  background: #f1f5f9;
  color: #334155;
}
.add-body { padding: 18px 18px 16px; box-sizing: border-box; }
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 16px;
  align-items: start;
}
.form-field {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  color: #64748b;
  position: relative;
}
.field-label {
  display: block;
  font-size: 12px;
  line-height: 18px;
  color: #64748b;
  white-space: nowrap;
}
.req { color: #ef4444; }
.form-row {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 13px;
  color: #334155;
  cursor: pointer;
}
.input {
  box-sizing: border-box;
  height: 34px;
  padding: 0 10px;
  border: 1px solid #dbe3ee;
  border-radius: 8px;
  font-size: 13px;
  background: #f8fafc;
  color: #0f172a;
  width: 100%;
  min-width: 0;
}
.input:focus {
  border-color: #3b82f6;
  background: #fff;
  outline: none;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
}
.combo {
  position: relative;
  width: 100%;
  min-width: 0;
}
.combo .input {
  padding-right: 32px;
}
.pwd-wrap {
  position: relative;
  width: 100%;
  min-width: 0;
}
.pwd-wrap .input {
  padding-right: 36px;
}
.pwd-toggle {
  position: absolute;
  right: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 28px;
  height: 26px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 6px;
  font-size: 13px;
  line-height: 1;
  color: #64748b;
}
.pwd-toggle:hover { background: #e2e8f0; }
.combo-arrow {
  position: absolute;
  right: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 26px;
  height: 26px;
  border: none;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  border-radius: 6px;
}
.combo-arrow:hover { background: #e2e8f0; }
.combo-menu {
  position: absolute;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.14);
  max-height: 180px;
  overflow: auto;
  z-index: 20;
}
.combo-item {
  padding: 8px 12px;
  font-size: 13px;
  color: #334155;
  cursor: pointer;
}
.combo-item:hover,
.combo-item.active {
  background: #eff6ff;
  color: #1d4ed8;
}
.combo-empty {
  padding: 10px 12px;
  font-size: 12px;
  color: #94a3b8;
}
.add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid #eef2f7;
}
.btn {
  height: 34px;
  padding: 0 18px;
  border-radius: 8px;
  font-size: 13px;
  cursor: pointer;
  border: 1px solid #dbe3ee;
  background: #fff;
  color: #334155;
}
.btn:hover { background: #f8fafc; }
.btn-primary {
  background: #2563eb;
  border-color: #2563eb;
  color: #fff;
}
.btn-primary:hover { background: #1d4ed8; }
</style>
