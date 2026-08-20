/**
 * SFTP 文件面板 composable
 * 每个 SSH 会话独立维护目录状态、路径栏编辑与远端文件操作
 */

import { computed, reactive, ref, watch, type Ref } from 'vue'
import {
  DownloadFromRemote,
  GetRemoteHome,
  ListRemoteDir,
  MkdirRemote,
  PickDownloadDir,
  PickLocalDir,
  PickLocalFile,
  PickSaveFile,
  RemoveRemote,
  RenameRemote,
  UploadToRemote,
} from '../../wailsjs/go/main/App.js'
import type { FilePaneState } from '../types'
import { errText, formatModTime, formatFileSize } from '../utils/format'

export function useFilePane(
  activeSessionID: Ref<string>,
  onOpenEditor: (remotePath: string) => void,
  onTransferPanelOpen: () => void,
  downloadDir: Ref<string>,
  joinLocalPath: (dir: string, name: string) => string,
) {
  const fileBySession = reactive<Record<string, FilePaneState>>({})
  const pathInputFocused = ref(false)
  const pathInputDraft = ref('/')

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

  const canDownloadRemote = computed(() => {
    const st = activeFileState.value
    if (!st?.selected) return false
    return Boolean(st.files.find((f) => f.Name === st.selected))
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

  async function refreshRemoteFiles(sessionID?: string): Promise<boolean> {
    const sid = sessionID || activeSessionID.value
    if (!sid) return false
    const st = ensureFileState(sid)
    st.loading = true
    try {
      st.files = await ListRemoteDir(sid, st.path)
      st.status = '就绪'
      return true
    } catch (e) {
      st.status = errText(e)
      return false
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

  function onPathInputFocus(e: FocusEvent) {
    pathInputFocused.value = true
    pathInputDraft.value = activeFileState.value?.path || '/'
    const input = e.target as HTMLInputElement
    input.select()
  }

  function onPathInputBlur() {
    pathInputFocused.value = false
    pathInputDraft.value = activeFileState.value?.path || '/'
  }

  async function onPathBarKeydown(e: KeyboardEvent) {
    const input = e.target as HTMLInputElement
    if (e.key === 'Enter') {
      e.preventDefault()
      const val = pathInputDraft.value.trim()
      if (!val || !activeFileState.value) return
      const target = val.startsWith('/') ? val : `/${val}`
      if (target !== activeFileState.value.path) {
        const oldPath = activeFileState.value.path
        activeFileState.value.path = target
        activeFileState.value.selected = ''
        const ok = await refreshRemoteFiles()
        if (!ok) {
          activeFileState.value.path = oldPath
          pathInputDraft.value = oldPath
          await refreshRemoteFiles()
        } else {
          pathInputDraft.value = target
        }
      }
      pathInputFocused.value = false
      input.blur()
    } else if (e.key === 'Escape') {
      pathInputDraft.value = activeFileState.value?.path || '/'
      input.blur()
    }
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
    } else if (entry) {
      onOpenEditor(entry.Path)
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
      onTransferPanelOpen()
      await UploadToRemote(sid, local, remotePath)
      st.status = pickDir ? '文件夹上传完成' : '上传完成'
      await refreshRemoteFiles(sid)
    } catch (e) {
      st.status = errText(e)
    }
  }

  async function remoteDownload(pickDownloadDirFn: () => Promise<void>) {
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
      onTransferPanelOpen()
      await DownloadFromRemote(sid, entry.Path, local)
      st.status = entry.IsDir ? '文件夹下载完成' : '下载完成'
    } catch (e) {
      st.status = errText(e)
    }
  }

  function removeFileState(sessionID: string) {
    delete fileBySession[sessionID]
  }

  watch(
    () => activeFileState.value?.path,
    (p) => {
      if (!pathInputFocused.value) {
        pathInputDraft.value = p || '/'
      }
    },
  )

  return {
    fileBySession,
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
  }
}
