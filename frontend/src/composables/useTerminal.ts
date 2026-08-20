/**
 * xterm.js 终端 composable
 * 负责 PTY 渲染、输出缓冲、尺寸同步与 Wails SSH 输入回传
 */

import { nextTick, type Ref } from 'vue'
import { ResizeSession, SendSessionInput } from '../../wailsjs/go/main/App.js'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import type { SessionTab } from '../types'
import { registerTerminalCsiHandlers, sanitizeTerminalChunk } from '../utils/terminalFilter'

const MAX_BUFFER_CHARS = 512 * 1024

/**
 * 绑定 xterm 输入：仅 onKey + 粘贴/IME 发往 SSH，不监听 onData
 * onData 会包含 xterm 对 CSI 查询的自动应答，发回 shell 后被 echo 成 □WWWW…
 */
function bindTerminalInput(term: Terminal, sessionID: string, hostEl: HTMLElement) {
  registerTerminalCsiHandlers(term)

  term.attachCustomKeyEventHandler((ev) => {
    if (ev.type === 'keydown' || ev.type === 'keypress') {
      if (ev.isComposing || (ev as KeyboardEvent).keyCode === 229) return true
      return false
    }
    return true
  })

  term.onKey(({ key }) => {
    if (key) SendSessionInput(sessionID, key).catch(console.error)
  })

  hostEl.addEventListener('paste', (ev) => {
    const text = ev.clipboardData?.getData('text')
    if (text) {
      ev.preventDefault()
      SendSessionInput(sessionID, text).catch(console.error)
    }
  })

  hostEl.addEventListener('compositionend', (ev) => {
    const text = (ev as CompositionEvent).data
    if (text) SendSessionInput(sessionID, text).catch(console.error)
  })
}

export function useTerminal(
  activeSessionID: Ref<string>,
  sessions: SessionTab[],
  isTypingInForm: () => boolean,
) {
  const terminalHosts = new Map<string, HTMLDivElement>()
  const termMap = new Map<string, { term: Terminal; fit: FitAddon; opened: boolean; resizeReady: boolean; inputBound: boolean }>()
  const outputBuffers = new Map<string, string>()
  const inboundTails = new Map<string, string>()

  function setTerminalHost(sessionID: string, el: unknown) {
    if (el instanceof HTMLDivElement) {
      const prev = terminalHosts.get(sessionID)
      terminalHosts.set(sessionID, el)
      if (prev === el) return

      const entry = termMap.get(sessionID)
      if (entry?.opened && prev && prev !== el) {
        entry.term.open(el)
      }
      if (sessionID === activeSessionID.value) {
        nextTick(() => openTerminal(sessionID, 0, false))
      }
    } else {
      terminalHosts.delete(sessionID)
      inboundTails.delete(sessionID)
      const entry = termMap.get(sessionID)
      if (entry) {
        entry.opened = false
        entry.resizeReady = false
      }
    }
  }

  function writeToTerminal(sessionID: string, data: string) {
    const cleaned = sanitizeTerminalChunk(sessionID, data, inboundTails)
    if (!cleaned) return
    if (!termMap.has(sessionID)) ensureTerminal(sessionID)
    const entry = termMap.get(sessionID)
    if (entry?.opened) {
      entry.term.write(cleaned)
      return
    }
    let buf = (outputBuffers.get(sessionID) ?? '') + cleaned
    if (buf.length > MAX_BUFFER_CHARS) {
      buf = buf.slice(buf.length - MAX_BUFFER_CHARS)
    }
    outputBuffers.set(sessionID, buf)
  }

  function syncTerminalSize(sessionID: string) {
    const entry = termMap.get(sessionID)
    if (!entry?.opened || !entry.resizeReady) return
    const { cols, rows } = entry.term
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

  function ensureTerminal(sessionID: string) {
    const exist = termMap.get(sessionID)
    if (exist) return exist

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      cols: 80,
      rows: 24,
      theme: {
        background: '#0b1220',
        foreground: '#e5e7eb',
        cursor: '#e5e7eb',
      },
      scrollback: 2000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    registerTerminalCsiHandlers(term)

    const entry = { term, fit, opened: false, resizeReady: false, inputBound: false }
    term.onResize(({ cols, rows }) => {
      if (!entry.resizeReady) return
      if (cols > 0 && rows > 0) {
        ResizeSession(sessionID, cols, rows).catch(() => {})
      }
    })
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

    const needsOpen = !entry.opened || entry.term.element?.parentElement !== el
    if (needsOpen) {
      if (!entry.inputBound) {
        bindTerminalInput(entry.term, sessionID, el)
        entry.inputBound = true
      }
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
        entry.resizeReady = true
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
    inboundTails.delete(sessionID)
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
