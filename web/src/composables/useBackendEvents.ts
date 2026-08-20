/**
 * Wails 后端事件订阅
 *
 * 注意：EventsOnMultiple 的第三个参数是「回调触发次数上限」，不是监听器个数
 * （Wails 的 EventsOnce 就是 EventsOnMultiple(name, cb, 1)）。这些都是持续推送的
 * 流式事件，必须用 EventsOn，否则只会收到第一条。
 */

import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'
import { parseEventArgs } from './useTerminal'

export const BACKEND_EVENTS = [
  'terminal-output',
  'terminal-closed',
  'transfer-update',
  'transfer-log',
  'srv-transfer-progress',
] as const

export type BackendEventHandlers = {
  onTerminalOutput: (sessionID: string, data: string) => void
  onTerminalClosed: (sessionID: string) => void
  onTransferUpdate: (payload: Record<string, unknown>) => void
  onTransferLog: (line: string) => void
  onServerTransferProgress: (taskID: string, total: number, transferred: number) => void
}

/** 订阅全部后端事件，返回取消订阅函数 */
export function subscribeBackendEvents(handlers: BackendEventHandlers): () => void {
  // 先清掉可能残留的订阅（开发模式热更新会重复挂载）
  EventsOff(...BACKEND_EVENTS)

  const unsubscribers = [
    EventsOn('terminal-output', (...args: unknown[]) => {
      const [sessionID, data] = parseEventArgs(args)
      if (sessionID) handlers.onTerminalOutput(sessionID, data)
    }),
    EventsOn('terminal-closed', (...args: unknown[]) => {
      const [sessionID] = parseEventArgs(args)
      if (sessionID) handlers.onTerminalClosed(sessionID)
    }),
    EventsOn('transfer-update', (...args: unknown[]) => {
      const ev = args[0]
      if (ev && typeof ev === 'object') handlers.onTransferUpdate(ev as Record<string, unknown>)
    }),
    EventsOn('transfer-log', (...args: unknown[]) => {
      if (typeof args[0] === 'string') handlers.onTransferLog(args[0])
    }),
    EventsOn('srv-transfer-progress', (...args: unknown[]) => {
      const ev = args[0] as { TaskID?: string; Total?: number; Transferred?: number } | undefined
      if (ev?.TaskID) {
        handlers.onServerTransferProgress(ev.TaskID, Number(ev.Total ?? 0), Number(ev.Transferred ?? 0))
      }
    }),
  ]

  return () => {
    for (const off of unsubscribers) off?.()
    EventsOff(...BACKEND_EVENTS)
  }
}
