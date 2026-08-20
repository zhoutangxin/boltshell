/**
 * 回归测试：xterm 内部辅助 DOM 必须被 xterm.css 隐藏。
 *
 * 曾经的线上故障：main.ts 漏引 'xterm/css/xterm.css'，导致
 *   - .xterm-char-measure-element（内容为 32 个 W 的测量元素）
 *   - .xterm-helper-textarea（承接键盘输入的真实 textarea）
 * 失去隐藏样式，直接渲染成终端里的 "WWWW…" 和一个白色方框。
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { beforeEach, describe, expect, it } from 'vitest'
import { Terminal } from 'xterm'

const XTERM_CSS_SPECIFIER = 'xterm/css/xterm.css'

/** vitest 的 cwd 是 frontend 目录 */
function readProjectFile(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8')
}

function loadXtermCss(): string {
  return readProjectFile('node_modules/xterm/css/xterm.css')
}

describe('xterm 样式表接线', () => {
  it('应用入口必须引入 xterm.css', () => {
    expect(readProjectFile('src/main.ts')).toContain(XTERM_CSS_SPECIFIER)
  })

  it('xterm.css 隐藏了字符测量元素与输入 textarea', () => {
    const css = loadXtermCss()
    expect(css).toMatch(/\.xterm-char-measure-element\s*\{[^}]*visibility:\s*hidden/)
    expect(css).toMatch(/\.xterm-helper-textarea\s*\{[^}]*opacity:\s*0/)
  })
})

describe('xterm 挂载后的 DOM', () => {
  let host: HTMLDivElement
  let term: Terminal

  beforeEach(() => {
    document.head.innerHTML = `<style>${loadXtermCss()}</style>`
    document.body.innerHTML = ''
    host = document.createElement('div')
    document.body.appendChild(host)
    term = new Terminal({ cols: 80, rows: 24 })
    term.open(host)
  })

  it('测量元素含 32 个 W，且被样式隐藏', () => {
    const measure = document.querySelector('.xterm-char-measure-element')
    expect(measure).not.toBeNull()
    // 这串 W 正是故障时肉眼看到的内容，确认来源
    expect(measure!.textContent).toBe('W'.repeat(32))
    expect(getComputedStyle(measure!).visibility).toBe('hidden')

    term.dispose()
  })

  it('输入 textarea 不可见', () => {
    const textarea = host.querySelector('.xterm-helper-textarea')
    expect(textarea).not.toBeNull()
    expect(getComputedStyle(textarea!).opacity).toBe('0')

    term.dispose()
  })

  it('渲染出的文本行里不含测量用的 W 串', () => {
    term.write('Last login: Thu Aug 20 11:05:39 2026\r\n')

    // 测量元素与 <style> 同为 .xterm-screen 的子节点（这正是缺样式时会显形的原因），
    // 因此只断言真正承载可见文本的 .xterm-rows
    const rows = host.querySelector('.xterm-rows')
    expect(rows).not.toBeNull()
    expect(rows!.textContent ?? '').not.toMatch(/W{12,}/)

    term.dispose()
  })
})
