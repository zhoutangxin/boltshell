/**
 * 文件传输任务 composable
 * 管理本地上传/下载进度列表，订阅 Wails transfer-update 事件
 */

import { computed, reactive, ref } from 'vue'
import { OpenLocalFolder, PickDownloadDir } from '../../wailsjs/go/main/App.js'
import { DOWNLOAD_DIR_KEY } from '../constants/app'
import type { TransferTask } from '../types'
import { formatBytes, joinLocalPath } from '../utils/format'

export function useTransferTasks() {
  const downloadDir = ref(localStorage.getItem(DOWNLOAD_DIR_KEY) || '')
  const transferTasks = reactive<TransferTask[]>([])
  const transferPanelOpen = ref(false)

  const activeTransferCount = computed(
    () => transferTasks.filter((t) => t.status === 'running').length,
  )

  function saveDownloadDir(dir: string) {
    downloadDir.value = dir
    if (dir) localStorage.setItem(DOWNLOAD_DIR_KEY, dir)
    else localStorage.removeItem(DOWNLOAD_DIR_KEY)
  }

  function transferPercent(t: TransferTask) {
    if (t.total > 0) return Math.min(100, Math.round((t.transferred / t.total) * 100))
    if (t.status === 'done') return 100
    return 0
  }

  function transferStatusText(t: TransferTask) {
    if (t.status === 'error') return t.error || '失败'
    if (t.status === 'done') return '完成'
    if (t.total > 0) return `${formatBytes(t.transferred)} / ${formatBytes(t.total)}`
    return '传输中…'
  }

  /** 合并 Go 端 transfer-update 事件到任务列表 */
  function upsertTransfer(ev: Record<string, unknown>) {
    const id = String(ev.TaskID ?? ev.taskID ?? ev.taskId ?? '')
    if (!id) return
    const prev = transferTasks.find((t) => t.id === id)
    const total = Number(ev.Total ?? ev.total ?? 0)
    const transferred = Number(ev.Transferred ?? ev.transferred ?? 0)
    const statusRaw = String(ev.Status ?? ev.status ?? 'running').toLowerCase()
    let status: TransferTask['status'] =
      statusRaw === 'done' ? 'done' : statusRaw === 'error' ? 'error' : 'running'
    // 迟到的 running 事件不应把已完成任务打回进行中（否则角标一直不清）
    if (prev && prev.status !== 'running' && status === 'running') {
      status = prev.status
    }
    const item: TransferTask = {
      id,
      sessionID: String(ev.SessionID ?? ev.sessionID ?? ev.sessionId ?? ''),
      kind: (ev.Kind ?? ev.kind) === 'download' ? 'download' : 'upload',
      fileName: String(ev.FileName ?? ev.fileName ?? 'file'),
      source: String(ev.Source ?? ev.source ?? ''),
      dest: String(ev.Dest ?? ev.dest ?? ''),
      total: total > 0 ? total : prev?.total ?? 0,
      transferred: transferred > 0 ? transferred : prev?.transferred ?? 0,
      status,
      error: String(ev.Error ?? ev.error ?? ''),
      updatedAt: Date.now(),
    }
    if (item.status === 'done' && item.total > 0) item.transferred = item.total
    const idx = transferTasks.findIndex((t) => t.id === id)
    if (idx >= 0) transferTasks[idx] = item
    else transferTasks.unshift(item)
    if (transferTasks.length > 50) transferTasks.length = 50
    if (item.status === 'running') transferPanelOpen.value = true
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
      console.error('[BoltShell] PickDownloadDir failed', e)
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
      console.error('[BoltShell] OpenLocalFolder failed', e)
    }
  }

  return {
    downloadDir,
    transferTasks,
    transferPanelOpen,
    activeTransferCount,
    saveDownloadDir,
    joinLocalPath,
    transferPercent,
    transferStatusText,
    upsertTransfer,
    clearFinishedTransfers,
    pickDownloadDir,
    openDownloadDir,
  }
}
