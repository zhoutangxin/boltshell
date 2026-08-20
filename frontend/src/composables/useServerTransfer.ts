/**
 * 跨服务器文件传送 composable
 * 临时 SFTP 连接目标服务器浏览目录并传送文件
 */

import { reactive } from 'vue'
import {
  BrowseConnectionDir,
  GetConnectionHome,
  ListConnections,
  TransferToConnection,
} from '../../wailsjs/go/main/App.js'
import type { Connection, TransferTask } from '../types'

export function useServerTransfer(
  getActiveSessionID: () => string,
  getSelectedEntry: () => { Path: string; Name: string } | undefined,
  transferTasks: TransferTask[],
  openTransferPanel: () => void,
) {
  const transfer = reactive({
    open: false,
    connections: [] as Connection[],
    targetConnID: '',
    targetPath: '/',
    targetDirs: [] as { Name: string; Path: string }[],
    browsing: false,
    srcPath: '',
    srcName: '',
    sending: false,
    error: '',
    success: '',
    logs: [] as string[],
  })

  async function openTransferDialog() {
    const entry = getSelectedEntry()
    if (!entry) return

    transfer.srcPath = entry.Path
    transfer.srcName = entry.Name
    transfer.targetConnID = ''
    transfer.targetPath = '/'
    transfer.targetDirs = []
    transfer.error = ''
    transfer.success = ''
    transfer.sending = false
    transfer.logs = []

    try {
      const conns = await ListConnections(false, '')
      transfer.connections = Array.isArray(conns) ? conns : []
      if (transfer.connections.length === 0) {
        alert('没有可用的连接')
        return
      }
      transfer.targetConnID = transfer.connections[0].ID
      transfer.open = true
      await loadTransferTargetDir()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      alert('获取连接列表失败: ' + msg)
    }
  }

  async function loadTransferTargetDir() {
    if (!transfer.targetConnID) return
    try {
      const home = await GetConnectionHome(transfer.targetConnID)
      await browseTargetDir(home || '/')
    } catch {
      await browseTargetDir('/')
    }
  }

  async function onTransferTargetChange() {
    transfer.targetPath = '/'
    transfer.targetDirs = []
    await loadTransferTargetDir()
  }

  function normalizeRemotePath(p: string) {
    const trimmed = p.trim()
    if (!trimmed) return '/'
    const withRoot = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
    return withRoot.replace(/\/+/g, '/').replace(/\/$/, '') || '/'
  }

  async function browseTargetDir(dir?: string) {
    if (!transfer.targetConnID) return
    transfer.browsing = true
    transfer.error = ''
    try {
      const p = normalizeRemotePath(dir ?? transfer.targetPath)
      transfer.targetDirs = await BrowseConnectionDir(transfer.targetConnID, p)
      transfer.targetPath = p
    } catch (e: unknown) {
      transfer.error = '浏览目录失败: ' + (e instanceof Error ? e.message : String(e))
    } finally {
      transfer.browsing = false
    }
  }

  function browseParent() {
    const p = transfer.targetPath.replace(/\/$/, '')
    if (!p || p === '/') return
    const idx = p.lastIndexOf('/')
    browseTargetDir(idx <= 0 ? '/' : p.slice(0, idx))
  }

  async function doTransfer() {
    if (!transfer.targetConnID || transfer.sending) return
    transfer.sending = true
    transfer.error = ''
    transfer.success = ''
    transfer.logs = [`[开始] 传送 ${transfer.srcName} → ${transfer.targetPath}`]

    const taskId = 'srv-' + Date.now()
    const conn = transfer.connections.find((c) => c.ID === transfer.targetConnID)
    const targetName = conn?.Name || conn?.Host || transfer.targetConnID
    const task: TransferTask = {
      id: taskId,
      sessionID: getActiveSessionID(),
      kind: 'upload',
      fileName: transfer.srcName,
      source: transfer.srcPath,
      dest: `${targetName}:${transfer.targetPath}`,
      total: 0,
      transferred: 0,
      status: 'running',
      error: '',
      updatedAt: Date.now(),
    }
    transferTasks.unshift(task)
    openTransferPanel()

    try {
      await TransferToConnection(
        getActiveSessionID(),
        transfer.srcPath,
        transfer.targetConnID,
        transfer.targetPath,
        taskId,
      )
      transfer.logs.push('[完成] 传送成功')
      transfer.success = `已传送 ${transfer.srcName} → ${transfer.targetPath}`
      task.status = 'done'
      task.updatedAt = Date.now()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      transfer.logs.push(`[错误] ${msg}`)
      transfer.error = msg
      task.status = 'error'
      task.error = msg
      task.updatedAt = Date.now()
    } finally {
      transfer.sending = false
    }
  }

  function appendLog(msg: string) {
    if (transfer.open) transfer.logs.push(msg)
  }

  function updateTaskProgress(taskID: string, total: number, transferred: number) {
    const t = transferTasks.find((x) => x.id === taskID)
    if (t) {
      t.total = total
      t.transferred = transferred
      t.updatedAt = Date.now()
      if (total > 0 && transferred >= total && t.status === 'running') {
        t.status = 'done'
      }
    }
  }

  return {
    transfer,
    openTransferDialog,
    onTransferTargetChange,
    browseTargetDir,
    browseParent,
    doTransfer,
    appendLog,
    updateTaskProgress,
  }
}
