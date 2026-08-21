<script setup lang="ts">
/**
 * BoltShell 主应用入口（布局编排层）
 *
 * 职责：
 * - 组合 composables（连接、终端、文件、传输、监控）
 * - 挂载子组件并传递 props / 事件
 * - 订阅 Wails 后端事件（terminal-output、transfer-update 等）
 *
 * 业务逻辑已下沉至 composables/，UI 已拆分至 components/
 */
import { nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { subscribeBackendEvents } from './composables/useBackendEvents'
import { ReadRemoteFile, WriteRemoteFile, GetAnalyticsEnabled, SetAnalyticsEnabled } from '../wailsjs/go/main/App.js'

import LeftRail from './components/layout/LeftRail.vue'
import UpdatePromptModal from './components/updater/UpdatePromptModal.vue'
import SysInfoPanel from './components/sysinfo/SysInfoPanel.vue'
import SessionTabBar from './components/session/SessionTabBar.vue'
import QuickConnectPanel from './components/session/QuickConnectPanel.vue'
import FilePane from './components/file/FilePane.vue'
import TransferPanel from './components/transfer/TransferPanel.vue'
import ConnectionManager from './components/connection/ConnectionManager.vue'
import AddGroupModal from './components/connection/AddGroupModal.vue'
import ExportPickerModal from './components/connection/ExportPickerModal.vue'
import ConnectionFormModal from './components/connection/ConnectionFormModal.vue'

import { useConnections } from './composables/useConnections'
import { useSysInfo } from './composables/useSysInfo'
import { useTransferTasks } from './composables/useTransferTasks'
import { useTerminal } from './composables/useTerminal'
import { useFilePane } from './composables/useFilePane'
import { useServerTransfer } from './composables/useServerTransfer'
import { useSponsors } from './composables/useSponsors'
import { useUpdater } from './composables/useUpdater'
import type { Connection } from './types'

// —— 连接 & 会话（解构 ref 供模板自动解包）——
const connApi = useConnections()
const {
  state,
  sessions,
  activeSessionID,
  selectedQuickID,
  collapsedGroups,
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
  onCloseTab,
  onQuickKeydown,
  onMgrKeydown,
  clearRecents,
  isTypingInForm,
} = connApi

// —— 系统监控 ——
const sysApi = useSysInfo(activeSessionID, sessions)
const { sysInfo, sysInfoLoading, memPercent, swapPercent, diskPercent, refreshSysInfo, startSysInfoPoll, stopSysInfoPoll } = sysApi

// —— 本地上传/下载任务 ——
const xferApi = useTransferTasks()
const {
  downloadDir,
  transferTasks,
  transferPanelOpen,
  activeTransferCount,
  joinLocalPath,
  transferPercent,
  transferStatusText,
  upsertTransfer,
  clearFinishedTransfers,
  pickDownloadDir,
  openDownloadDir,
} = xferApi

// —— 赞助位（远程配置）——
const sponsorApi = useSponsors()
const { quickSlot, sidebarSlots, dismiss: dismissSponsor, config: sponsorConfig } = sponsorApi

// —— 版本升级 ——
const updaterApi = useUpdater()
const {
  currentVersion,
  updateInfo,
  upgrading,
  upgradeStatus,
  showUpdatePrompt,
  upgrade: applyUpgrade,
  upgradeFromPrompt,
  dismissUpdatePrompt,
  hasUpdate,
} = updaterApi

// —— 匿名统计开关 ——
const analyticsEnabled = ref(true)
async function toggleAnalytics() {
  const next = !analyticsEnabled.value
  try {
    await SetAnalyticsEnabled(next)
    analyticsEnabled.value = next
  } catch (e) {
    console.error('[BoltShell] SetAnalyticsEnabled failed', e)
  }
}

// —— xterm 终端 ——
const termApi = useTerminal(activeSessionID, sessions, isTypingInForm)
const {
  setTerminalHost,
  writeToTerminal,
  openTerminal,
  openOrSwitchTerminal,
  markSessionClosed,
  disposeTerminal,
  fitActiveTerminal,
  ensureTerminal,
} = termApi

// —— 远端文件编辑器 ——
const editor = reactive({
  open: false,
  path: '',
  content: '',
  original: '',
  saving: false,
  loading: false,
  error: '',
})

async function openFileEditor(remotePath: string) {
  const sid = activeSessionID.value
  if (!sid) return
  editor.path = remotePath
  editor.content = ''
  editor.original = ''
  editor.error = ''
  editor.saving = false
  editor.loading = true
  editor.open = true
  try {
    const text = await ReadRemoteFile(sid, remotePath)
    editor.content = text
    editor.original = text
  } catch (e: unknown) {
    editor.error = e instanceof Error ? e.message : String(e)
  } finally {
    editor.loading = false
  }
}

async function saveFileEditor() {
  const sid = activeSessionID.value
  if (!sid || editor.saving) return
  editor.saving = true
  editor.error = ''
  try {
    await WriteRemoteFile(sid, editor.path, editor.content)
    editor.original = editor.content
    editor.open = false
  } catch (e: unknown) {
    editor.error = e instanceof Error ? e.message : String(e)
  } finally {
    editor.saving = false
  }
}

function closeFileEditor() {
  if (editor.content !== editor.original) {
    if (!confirm('文件已修改但未保存，确定关闭？')) return
  }
  editor.open = false
}

// —— SFTP 文件面板 ——
const filePaneApi = useFilePane(
  activeSessionID,
  openFileEditor,
  () => { transferPanelOpen.value = true },
  downloadDir,
  joinLocalPath,
)
const {
  pathInputDraft,
  activeFileState,
  fileTreePaths,
  canDownloadRemote,
  formatFileSize,
  formatModTime,
  initFilePane,
  refreshRemoteFiles,
  selectRemoteFile,
  navigateRemote,
  onPathInputFocus,
  onPathInputBlur,
  onPathBarKeydown,
  openRemoteEntry,
  remoteGoUp,
  remoteMkdir,
  remoteDelete,
  remoteRename,
  remoteUpload,
  remoteDownload,
  removeFileState,
} = filePaneApi

// —— 跨服务器传送 ——
const srvXferApi = useServerTransfer(
  () => activeSessionID.value,
  () => {
    const st = activeFileState.value
    if (!st?.selected) return undefined
    return st.files.find((f) => f.Name === st.selected)
  },
  transferTasks,
  () => { transferPanelOpen.value = true },
)
const { transfer: serverTransfer, openTransferDialog, onTransferTargetChange, browseTargetDir, browseParent, doTransfer, appendLog, updateTaskProgress } = srvXferApi

// —— UI 布局状态 ——
const ui = reactive({
  sysCollapsed: false,
  fileCollapsed: false,
  // 首次展示时按左侧赞助区实测高度对齐；之后拖动不再改左边
  filePaneHeight: 200,
  fileTab: 'files' as 'files' | 'cmd',
})

const splitWrapRef = ref<HTMLElement | null>(null)
/** 是否已做过「默认高度 = 赞助区高度」的一次性对齐 */
const filePaneDefaultSynced = ref(false)

/** 仅默认对齐一次：文件面板高度 = 左侧 .ad-slot-group 实测高度 */
function syncFilePaneDefaultToSponsors() {
  if (filePaneDefaultSynced.value) return
  if (!sessions.length || ui.fileCollapsed || ui.sysCollapsed) return
  if (!sidebarSlots.value?.length) return

  nextTick(() => {
    requestAnimationFrame(() => {
      if (filePaneDefaultSynced.value) return
      const el = document.querySelector('.ad-slot-group') as HTMLElement | null
      if (!el) return
      const h = Math.round(el.getBoundingClientRect().height)
      if (h < 80) return
      ui.filePaneHeight = h
      filePaneDefaultSynced.value = true
    })
  })
}

async function onSessionConnected(sid: string) {
  ensureTerminal(sid)
  openTerminal(sid, 0, true)
  await initFilePane(sid)
  await refreshSysInfo(sid)
  syncFilePaneDefaultToSponsors()
}

function handleConnect(c: Connection) {
  onConnect(c, onSessionConnected)
}

function handleCloseTab(sessionID: string) {
  onCloseTab(sessionID, (id) => {
    disposeTerminal(id)
    removeFileState(id)
  }, openOrSwitchTerminal)
}

function startSplitDrag(e: MouseEvent) {
  const wrap = splitWrapRef.value
  if (!wrap) return
  const startY = e.clientY
  const startH = ui.filePaneHeight
  const onMove = (ev: MouseEvent) => {
    ui.filePaneHeight = Math.max(100, Math.min(wrap.clientHeight - 120, startH + startY - ev.clientY))
    fitActiveTerminal()
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

let unsubscribeBackendEvents: (() => void) | null = null

onMounted(async () => {
  unsubscribeBackendEvents = subscribeBackendEvents({
    onTerminalOutput: writeToTerminal,
    onTerminalClosed: markSessionClosed,
    onTransferUpdate: upsertTransfer,
    onTransferLog: appendLog,
    onServerTransferProgress: updateTaskProgress,
  })

  await refreshList()
  loadRecents()
  try {
    analyticsEnabled.value = await GetAnalyticsEnabled()
  } catch {
    /* ignore */
  }

  window.addEventListener('keydown', (e) => onMgrKeydown(e, handleConnect))
  window.addEventListener('keydown', (e) => onQuickKeydown(e, handleConnect))
  window.addEventListener('resize', fitActiveTerminal)
  startSysInfoPoll()
})

onUnmounted(() => {
  stopSysInfoPoll()
  unsubscribeBackendEvents?.()
  unsubscribeBackendEvents = null
})

watch(activeSessionID, () => {
  nextTick().then(openOrSwitchTerminal)
  const sid = activeSessionID.value
  if (sid && filePaneApi.fileBySession[sid]) refreshRemoteFiles(sid).catch(console.error)
  refreshSysInfo(sid).catch(console.error)
  syncFilePaneDefaultToSponsors()
})

watch(sidebarSlots, () => {
  syncFilePaneDefaultToSponsors()
})
</script>

<template>
  <div class="app">
    <LeftRail
      :mgr-open="state.showMgr"
      :has-update="hasUpdate()"
      :current-version="currentVersion"
      :latest-version="updateInfo?.LatestVersion"
      :upgrading="upgrading"
      :upgrade-status="upgradeStatus"
      :analytics-enabled="analyticsEnabled"
      @open-mgr="openMgr"
      @upgrade="applyUpgrade"
      @toggle-analytics="toggleAnalytics"
    />

    <SysInfoPanel
      v-if="sessions.length > 0 && !ui.sysCollapsed"
      :sys-info="sysInfo"
      :sys-info-loading="sysInfoLoading"
      :mem-percent="memPercent"
      :swap-percent="swapPercent"
      :disk-percent="diskPercent"
      :sidebar-slots="sidebarSlots"
      :pro-upgrade-url="sponsorConfig?.ProUpgradeURL"
      :surface-session="activeSessionID ? `sb-${activeSessionID}` : 'sb'"
      :config-version="sponsorConfig?.Version"
      @dismiss-sponsor="(id, days) => dismissSponsor(id, days)"
    />

    <div class="workspace">
      <SessionTabBar
        :sessions="sessions"
        :active-session-id="activeSessionID"
        :active-transfer-count="activeTransferCount"
        :sys-collapsed="ui.sysCollapsed"
        :file-collapsed="ui.fileCollapsed"
        @select="activeSessionID = $event"
        @close="handleCloseTab"
        @toggle-transfer="transferPanelOpen = !transferPanelOpen"
        @toggle-sys="ui.sysCollapsed = !ui.sysCollapsed"
        @toggle-file="ui.fileCollapsed = !ui.fileCollapsed"
      />

      <div ref="splitWrapRef" class="split-wrap">
        <div class="main-area">
          <QuickConnectPanel
            v-if="sessions.length === 0"
            :connections="quickConnections"
            :selected-id="selectedQuickID"
            :conn-title="connTitle"
            :group-name="groupName"
            :sponsor-slot="quickSlot"
            :pro-upgrade-url="sponsorConfig?.ProUpgradeURL"
            :config-version="sponsorConfig?.Version"
            @select="selectedQuickID = $event"
            @connect="handleConnect"
            @clear-recents="clearRecents"
            @dismiss-sponsor="(id, days) => dismissSponsor(id, days)"
          />
          <div
            v-for="s in sessions"
            :key="s.sessionID"
            :ref="(el) => setTerminalHost(s.sessionID, el)"
            class="terminal-host"
            v-show="activeSessionID === s.sessionID"
          />
        </div>

        <FilePane
          :visible="!!(sessions.length && activeSessionID && !ui.fileCollapsed)"
          :height="ui.filePaneHeight"
          :file-tab="ui.fileTab"
          :file-state="activeFileState"
          :file-tree-paths="fileTreePaths"
          :path-input-draft="pathInputDraft"
          :can-download="canDownloadRemote"
          :format-file-size="formatFileSize"
          :format-mod-time="formatModTime"
          :editor="editor"
          :server-transfer="serverTransfer"
          @update:file-tab="ui.fileTab = $event"
          @update:path-input-draft="pathInputDraft = $event"
          @update:editor-content="editor.content = $event"
          @update:target-conn-id="serverTransfer.targetConnID = $event"
          @update:target-path="serverTransfer.targetPath = $event"
          @split-drag="startSplitDrag"
          @go-up="remoteGoUp"
          @refresh="refreshRemoteFiles()"
          @upload="remoteUpload($event)"
          @download="remoteDownload(pickDownloadDir)"
          @mkdir="remoteMkdir"
          @delete="remoteDelete"
          @rename="remoteRename"
          @open-transfer="openTransferDialog"
          @path-focus="onPathInputFocus"
          @path-blur="onPathInputBlur"
          @path-keydown="onPathBarKeydown"
          @navigate="navigateRemote"
          @select-file="selectRemoteFile"
          @open-entry="openRemoteEntry"
          @save-editor="saveFileEditor"
          @close-editor="closeFileEditor"
          @close-transfer="serverTransfer.open = false"
          @transfer-target-change="onTransferTargetChange"
          @browse-target="browseTargetDir"
          @browse-parent="browseParent"
          @do-transfer="doTransfer"
        />
      </div>
    </div>

    <TransferPanel
      v-if="sessions.length > 0"
      :open="transferPanelOpen"
      :download-dir="downloadDir"
      :tasks="transferTasks"
      :transfer-percent="transferPercent"
      :transfer-status-text="transferStatusText"
      @close="transferPanelOpen = false"
      @clear-finished="clearFinishedTransfers"
      @pick-download-dir="pickDownloadDir"
      @open-download-dir="openDownloadDir"
    />

    <ConnectionManager
      :open="state.showMgr"
      :search-text="state.searchText"
      :group-filter="state.groupFilter"
      :include-deleted="state.includeDeleted"
      :close-after-connect="state.closeAfterConnect"
      :selected-conn-id="state.selectedConnID"
      :selected-group-name="state.selectedGroupName"
      :mgr-message="state.mgrMessage"
      :mgr-message-type="state.mgrMessageType"
      :group-options="groupOptions"
      :grouped-connections="groupedConnections"
      :collapsed-groups="collapsedGroups"
      :conn-title="connTitle"
      @close="closeMgr"
      @update:search-text="state.searchText = $event"
      @update:group-filter="state.groupFilter = $event"
      @update:include-deleted="state.includeDeleted = $event"
      @update:close-after-connect="state.closeAfterConnect = $event"
      @add="openAddModal"
      @add-group="openAddGroupModal"
      @edit-group="openEditGroupModal"
      @toggle-selected="onToggleSelected"
      @refresh="refreshList"
      @import="onImportConnections"
      @export="openExportPicker"
      @toggle-group="toggleGroup"
      @select-group="selectGroup"
      @select-conn="selectConn"
      @connect="handleConnect"
      @edit="openEditModal"
      @move-conn="onMoveConnection"
    />

    <AddGroupModal
      :open="state.showAddGroup"
      :title="state.editingGroupName === '未分组' ? '为未分组指定名称' : state.editingGroupName ? '重命名分组' : '新增分组'"
      :group-name="state.newGroupName"
      @update:group-name="state.newGroupName = $event"
      @save="onSaveGroup"
      @close="closeAddGroupModal"
    />

    <ExportPickerModal
      :open="state.showExportPicker"
      :grouped-connections="exportGrouped"
      :conn-title="connTitle"
      @close="closeExportPicker"
      @confirm="onExportConnections"
    />

    <ConnectionFormModal
      :open="state.showAdd"
      :editing="!!state.editingID"
      :form="state.form"
      :show-password="showPassword"
      :show-group-menu="showGroupMenu"
      :filtered-groups="filteredGroups"
      @update:form="Object.assign(state.form, $event)"
      @update:show-password="showPassword = $event"
      @update:show-group-menu="showGroupMenu = $event"
      @pick-group="pickGroup"
      @save="onSaveConnection"
      @close="closeAddModal"
    />

    <UpdatePromptModal
      :open="showUpdatePrompt"
      :info="updateInfo"
      :upgrading="upgrading"
      :upgrade-status="upgradeStatus"
      @close="dismissUpdatePrompt"
      @upgrade="upgradeFromPrompt"
    />
  </div>
</template>

<style scoped>
.app {
  height: 100vh;
  display: flex;
  flex-direction: row;
  text-align: left;
  overflow: hidden;
}
</style>

<style src="./styles/legacy-ui.css"></style>
