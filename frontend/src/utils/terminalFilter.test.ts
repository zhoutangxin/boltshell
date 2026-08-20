import { describe, expect, it } from 'vitest'
import {
  drainTerminalOutbound,
  isXtermAutoResponse,
  sanitizeTerminalChunk,
  sanitizeTerminalOutput,
} from './terminalFilter'

describe('sanitizeTerminalOutput', () => {
  it('removes inline w/W garbage (no newline)', () => {
    for (const ch of ['w', 'W']) {
      const garbage = ch.repeat(120)
      const input = `\x1b[?1;2c${garbage}Last login: Thu Aug 20`
      const out = sanitizeTerminalOutput(input)
      expect(out).not.toMatch(/[wW]{12,}/)
      expect(out).toContain('Last login')
    }
  })

  it('removes full-line w garbage', () => {
    const input = `\r\n${'w'.repeat(80)}\r\nLast login:\r\n`
    expect(sanitizeTerminalOutput(input)).toBe('\r\nLast login:\r\n')
  })

  it('strips DA sequences from echo', () => {
    expect(sanitizeTerminalOutput('\x1b[?1;2chello')).toBe('hello')
    expect(sanitizeTerminalOutput('\x1b[>0;136;0chello')).toBe('hello')
  })

  it('strips window size report echo', () => {
    expect(sanitizeTerminalOutput('\x1b[8;24;120thello')).toBe('hello')
  })

  it('strips server window query', () => {
    expect(sanitizeTerminalOutput('\x1b[18tLast login')).toBe('Last login')
    expect(sanitizeTerminalOutput('\x1b[6nLast login')).toBe('Last login')
  })

  it('preserves normal output', () => {
    const normal = '[root@localhost ~]# ls\r\n'
    expect(sanitizeTerminalOutput(normal)).toBe(normal)
  })
})

describe('sanitizeTerminalChunk', () => {
  it('reassembles split CSI before strip', () => {
    const tails = new Map<string, string>()
    const a = sanitizeTerminalChunk('s1', '\x1b[8;24;', tails)
    expect(a).toBe('')
    expect(tails.get('s1')).toBe('\x1b[8;24;')
    const b = sanitizeTerminalChunk('s1', '120tHi', tails)
    expect(b).toBe('Hi')
    expect(tails.get('s1')).toBe('')
  })
})

describe('drainTerminalOutbound', () => {
  it('drops primary DA auto response', () => {
    const { toSend, remaining } = drainTerminalOutbound('\x1b[?1;2c')
    expect(toSend).toEqual([])
    expect(remaining).toBe('')
  })

  it('drops window size auto response', () => {
    const { toSend } = drainTerminalOutbound('\x1b[8;24;120t')
    expect(toSend).toEqual([])
  })

  it('drops CPR auto response', () => {
    const { toSend } = drainTerminalOutbound('\x1b[24;80R')
    expect(toSend).toEqual([])
  })

  it('forwards plain user typing', () => {
    const { toSend, remaining } = drainTerminalOutbound('ls -la\r')
    expect(toSend).toEqual(['ls -la\r'])
    expect(remaining).toBe('')
  })

  it('forwards arrow key escape sequence', () => {
    const { toSend } = drainTerminalOutbound('\x1b[A')
    expect(toSend).toEqual(['\x1b[A'])
  })

  it('handles auto response followed by user input', () => {
    const { toSend } = drainTerminalOutbound('\x1b[?1;2cls')
    expect(toSend).toEqual(['ls'])
  })

  it('buffers incomplete CSI', () => {
    const { toSend, remaining } = drainTerminalOutbound('\x1b[24;')
    expect(toSend).toEqual([])
    expect(remaining).toBe('\x1b[24;')
  })
})

describe('isXtermAutoResponse', () => {
  it('detects known auto responses', () => {
    expect(isXtermAutoResponse('\x1b[?1;2c')).toBe(true)
    expect(isXtermAutoResponse('\x1b[8;24;120t')).toBe(true)
    expect(isXtermAutoResponse('\x1b[A')).toBe(false)
    expect(isXtermAutoResponse('hello')).toBe(false)
  })
})
