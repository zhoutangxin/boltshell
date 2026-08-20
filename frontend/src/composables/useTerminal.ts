/**
 * xterm.js 终端 composable
 * 负责 PTY 渲染、输出缓冲、尺寸同步与 Wails SSH 输入回传
 */

import { nextTick, type Ref } from 'vue'
import { ResizeSession, SendSessionInput } from '../../wailsjs/go/main/App.js'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import type { SessionTab } from '../types'

const MAX_BUFFER_CHARS = 512 * 1024

type TermEntry = {
  term: Terminal
  fit: FitAddon
  opened: boolean
  inputBound: boolean
  /** 已同步给远端 PTY 的尺寸，用于避免重复下发 WindowChange */
  syncedCols: number
  syncedRows: number
}

export function useTerminal(
  activeSessionID: Ref<string>,
  sessions: SessionTab[],
  isTypingInForm: () => boolean,
) {
  const terminalHosts = new Map<string, HTMLDivElement>()
  const termMap = new Map<string, TermEntry>()
  /** SSH 输出可能在 xterm DOM 挂载前到达，先缓冲 */
  const outputBuffers = new Map<string, string>()
  /** 已关闭的会话，用于丢弃后端迟到的输出，避免重建出永不释放的 xterm 实例 */
  const disposedSessions = new Set<string>()

  function setTerminalHost(sessionID: string, el: unknown) {
    // 模板里用的是内联箭头函数 ref，Vue 每次重渲染都会先以 null 再以元素回调一次。
    // 这里必须忽略 null，否则下面的 prev === el 短路失效，会话每次渲染都要重新 fit +
    // 下发一次远端 PTY resize。宿主节点由 v-show 保活，清理走 disposeTerminal。
    if (!(el instanceof HTMLDivElement)) return

    const prev = terminalHosts.get(sessionID)
    if (prev === el) return
    terminalHosts.set(sessionID, el)
    if (sessionID === activeSessionID.value) {
      nextTick(() => openTerminal(sessionID, 0, false))
    }
  }

  function writeToTerminal(sessionID: string, data: string) {
    if (!data) return
    if (disposedSessions.has(sessionID)) return
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
    const { cols, rows } = entry.term
    if (cols < 1 || rows < 1) return
    if (entry.syncedCols === cols && entry.syncedRows === rows) return
    entry.syncedCols = cols
    entry.syncedRows = rows
    ResizeSession(sessionID, cols, rows).catch(() => {})
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

  function ensureTerminal(sessionID: string) {
    const exist = termMap.get(sessionID)
    if (exist) return exist

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      theme: {
        background: '#0b1220',
        foreground: '#e5e7eb',
        cursor: '#e5e7eb',
      },
      scrollback: 2000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)

    const entry: TermEntry = {
      term,
      fit,
      opened: false,
      inputBound: false,
      syncedCols: 0,
      syncedRows: 0,
    }
    term.onResize(() => syncTerminalSize(sessionID))
    termMap.set(sessionID, entry)
    return entry
  }

  function openTerminal(sessionID: string, attempt = 0, wantFocus = false) {
    const el = terminalHosts.get(sessionID)
    if (!el) {
      if (attempt < 24) requestAnimationFrame(() => openTerminal(sessionID, attempt + 1, wantFocus))
      return
    }
    const entry = termMap.get(sessionID)
    if (!entry) return

    // 宿主节点被替换时（首次挂载 / DOM 重建）才需要重新 open
    const remounted = !entry.opened || entry.term.element?.parentElement !== el
    if (remounted) {
      entry.term.open(el)
      entry.opened = true
      if (!entry.inputBound) {
        entry.term.onData((data) => {
          SendSessionInput(sessionID, data).catch(console.error)
        })
        entry.inputBound = true
      }
      flushTerminalBuffer(sessionID)
    }

    let tries = 0
    const tryFit = () => {
      tries++
      const rect = el.getBoundingClientRect()
      if (rect.width < 5 || rect.height < 5) {
        if (tries < 60) requestAnimationFrame(tryFit)
        return
      }
      try {
        entry.fit.fit()
        // 全屏重绘只在重新挂载后需要，常规切换交给 fit 触发的增量渲染
        if (remounted) entry.term.refresh(0, entry.term.rows - 1)
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

  function markSessionClosed(sessionID: string) {
    const tab = sessions.find((s) => s.sessionID === sessionID)
    if (!tab) return
    tab.closed = true
    if (!tab.title.includes('(已断开)')) {
      tab.title += ' (已断开)'
    }
  }

  function disposeTerminal(sessionID: string) {
    const entry = termMap.get(sessionID)
    if (entry) {
      entry.term.dispose()
      termMap.delete(sessionID)
    }
    terminalHosts.delete(sessionID)
    outputBuffers.delete(sessionID)
    disposedSessions.add(sessionID)
  }

  function fitActiveTerminal() {
    if (!activeSessionID.value) return
    const entry = termMap.get(activeSessionID.value)
    if (!entry?.opened) return
    const el = terminalHosts.get(activeSessionID.value)
    if (!el) return
    const rect = el.getBoundingClientRect()
    if (rect.width < 5 || rect.height < 5) return
    try {
      entry.fit.fit()
      syncTerminalSize(activeSessionID.value)
    } catch {
      /* ignore */
    }
  }

  function getTermEntry(sessionID: string) {
    return termMap.get(sessionID)
  }

  return {
    setTerminalHost,
    writeToTerminal,
    openTerminal,
    openOrSwitchTerminal,
    markSessionClosed,
    disposeTerminal,
    fitActiveTerminal,
    ensureTerminal,
    getTermEntry,
  }
}

/** 解析 Wails EventsOn 回调参数（兼容多种传参格式） */
export function parseEventArgs(args: unknown[]): [string, string] {
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
