/**
 * xterm 集成测试：复现 echo 乱码并验证 CSI 处理器
 */
import { beforeEach, describe, expect, it } from 'vitest'
import { Terminal } from 'xterm'
import {
  isXtermAutoResponse,
  registerTerminalCsiHandlers,
  sanitizeTerminalOutput,
} from './terminalFilter'

function mountTerminal(handlers = true): { term: Terminal; host: HTMLDivElement; sent: string[] } {
  const host = document.createElement('div')
  host.style.width = '800px'
  host.style.height = '400px'
  document.body.appendChild(host)

  const term = new Terminal({ cols: 80, rows: 24 })
  const sent: string[] = []

  if (handlers) registerTerminalCsiHandlers(term)

  term.onData((data) => sent.push(data))
  term.open(host)
  return { term, host, sent }
}

function bufferText(term: Terminal): string {
  let out = ''
  for (let y = 0; y < term.rows; y++) {
    out += term.buffer.active.getLine(y)?.translateToString(true) ?? ''
    if (y < term.rows - 1) out += '\n'
  }
  return out
}

function hasLongWRun(text: string): boolean {
  return /[wW]{12,}/.test(text)
}

describe('xterm CSI handlers', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('does not emit onData when server sends primary DA query', () => {
    const { term, host, sent } = mountTerminal()
    term.write('\x1b[c')
    expect(sent.filter((s) => isXtermAutoResponse(s))).toEqual([])
    expect(hasLongWRun(bufferText(term))).toBe(false)
    term.dispose()
    host.remove()
  })

  it('swallows window size report CSI 8 (echo of \\x1b[18t response)', () => {
    const { term, host } = mountTerminal(true)
    term.write('\x1b[8;24;120t')
    expect(hasLongWRun(bufferText(term))).toBe(false)
    term.dispose()
    host.remove()
  })

  it('login-like output stays clean after stripping', () => {
    const { term, host } = mountTerminal()
    const garbage = 'W'.repeat(120)
    const raw = `\x1b[?1;2c${garbage}\x1b[8;24;120tLast login: Thu Aug 20 10:33:40 2026\n`
    term.write(sanitizeTerminalOutput(raw))
    const text = bufferText(term)
    expect(hasLongWRun(text)).toBe(false)
    expect(text).toContain('Last login')
    term.dispose()
    host.remove()
  })

  it('handles CSI split across two writes with handlers', () => {
    const { term, host } = mountTerminal()
    term.write('\x1b[8;24;')
    term.write('120t\nOK')
    expect(hasLongWRun(bufferText(term))).toBe(false)
    expect(bufferText(term)).toContain('OK')
    term.dispose()
    host.remove()
  })

  it('regression: forwarding onData back into terminal pollutes screen', () => {
    const { term, host } = mountTerminal(false)
    term.onData((data) => {
      if (isXtermAutoResponse(data)) term.write(data)
    })
    term.write('\x1b[c')
    expect(term.buffer.active.getLine(0)?.translateToString(true) ?? '').not.toMatch(/^[wW]{12,}$/)
    term.dispose()
    host.remove()
  })
})
