/**
 * 回归测试：终端宿主节点的 ref 回调不能引发重复的远端 PTY resize。
 *
 * 曾经的性能故障：模板用内联箭头函数作 :ref，Vue 每次重渲染都会先以 null
 * 回调一次；setTerminalHost 若在 null 时清空记录，prev === el 的短路就会失效，
 * 系统信息每 5 秒轮询一次就会带来一轮 fit + 全屏重绘 + ResizeSession IPC。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

const resizeSession = vi.fn((_id: string, _cols: number, _rows: number) => Promise.resolve())
const sendSessionInput = vi.fn((_id: string, _data: string) => Promise.resolve())

vi.mock('../../wailsjs/go/main/App.js', () => ({
  ResizeSession: (id: string, cols: number, rows: number) => resizeSession(id, cols, rows),
  SendSessionInput: (id: string, data: string) => sendSessionInput(id, data),
}))

const { useTerminal } = await import('./useTerminal')

/** 等待 openTerminal 内部的 nextTick + 双层 rAF 调度跑完 */
function flushScheduling(): Promise<void> {
  return new Promise((done) => setTimeout(done, 50))
}

function makeHost(): HTMLDivElement {
  const el = document.createElement('div')
  // happy-dom 默认没有布局，显式给出尺寸让 fit() 能算出行列
  el.getBoundingClientRect = () => ({ width: 800, height: 400 }) as DOMRect
  document.body.appendChild(el)
  return el
}

describe('setTerminalHost', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    resizeSession.mockClear()
    sendSessionInput.mockClear()
  })

  it('重渲染的 null→元素 回调不触发重新 fit', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, getTermEntry } = useTerminal(
      activeSessionID,
      [],
      () => false,
    )

    ensureTerminal('s1')
    const host = makeHost()
    setTerminalHost('s1', host)
    await flushScheduling()

    // fit() 会做 DOM 测量，是重渲染时最主要的浪费来源
    const fitSpy = vi.spyOn(getTermEntry('s1')!.fit, 'fit')

    // 模拟 5 次重渲染：Vue 先以 null 回调旧 ref，再以同一元素回调新 ref
    for (let i = 0; i < 5; i++) {
      setTerminalHost('s1', null)
      setTerminalHost('s1', host)
    }
    await flushScheduling()

    expect(fitSpy).not.toHaveBeenCalled()
  })

  it('宿主节点真的被替换时会重新挂载', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, getTermEntry } = useTerminal(
      activeSessionID,
      [],
      () => false,
    )

    ensureTerminal('s1')
    const first = makeHost()
    setTerminalHost('s1', first)
    await flushScheduling()

    const second = makeHost()
    setTerminalHost('s1', second)
    await flushScheduling()

    expect(getTermEntry('s1')?.term.element?.parentElement).toBe(second)
  })
})

describe('尺寸同步', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    resizeSession.mockClear()
  })

  it('尺寸未变化时不重复调用 ResizeSession', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, fitActiveTerminal } = useTerminal(
      activeSessionID,
      [],
      () => false,
    )

    ensureTerminal('s1')
    setTerminalHost('s1', makeHost())
    await flushScheduling()

    const baseline = resizeSession.mock.calls.length
    fitActiveTerminal()
    fitActiveTerminal()
    fitActiveTerminal()

    expect(resizeSession.mock.calls.length).toBe(baseline)
  })

  it('每个会话各自同步自己的尺寸', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal } = useTerminal(activeSessionID, [], () => false)

    ensureTerminal('s1')
    setTerminalHost('s1', makeHost())
    await flushScheduling()

    activeSessionID.value = 's2'
    ensureTerminal('s2')
    setTerminalHost('s2', makeHost())
    await flushScheduling()

    const ids = new Set(resizeSession.mock.calls.map((c) => c[0]))
    expect(ids).toEqual(new Set(['s1', 's2']))
  })
})

describe('多会话隔离', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    resizeSession.mockClear()
    sendSessionInput.mockClear()
  })

  it('输出只写进对应会话的终端', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, writeToTerminal, getTermEntry } = useTerminal(
      activeSessionID,
      [],
      () => false,
    )

    for (const sid of ['s1', 's2']) {
      activeSessionID.value = sid
      ensureTerminal(sid)
      setTerminalHost(sid, makeHost())
      await flushScheduling()
    }

    const w1 = vi.spyOn(getTermEntry('s1')!.term, 'write')
    const w2 = vi.spyOn(getTermEntry('s2')!.term, 'write')

    writeToTerminal('s1', 'hello-one')
    writeToTerminal('s2', 'hello-two')

    expect(w1).toHaveBeenCalledWith('hello-one')
    expect(w1).not.toHaveBeenCalledWith('hello-two')
    expect(w2).toHaveBeenCalledWith('hello-two')
    expect(w2).not.toHaveBeenCalledWith('hello-one')
  })

  it('DOM 挂载前到达的输出会被缓冲，挂载后补写', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, writeToTerminal, getTermEntry } = useTerminal(
      activeSessionID,
      [],
      () => false,
    )

    // 后端在 StartSession 返回后立刻开始推送，可能早于 Tab 渲染
    writeToTerminal('s1', 'early-output')
    expect(getTermEntry('s1')).toBeDefined()

    setTerminalHost('s1', makeHost())
    await flushScheduling()

    expect(getTermEntry('s1')!.term.buffer.active.getLine(0)?.translateToString(true)).toContain(
      'early-output',
    )
  })

  it('关闭其中一个会话不影响其它会话', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, writeToTerminal, disposeTerminal, getTermEntry } =
      useTerminal(activeSessionID, [], () => false)

    for (const sid of ['s1', 's2']) {
      activeSessionID.value = sid
      ensureTerminal(sid)
      setTerminalHost(sid, makeHost())
      await flushScheduling()
    }

    disposeTerminal('s1')

    expect(getTermEntry('s1')).toBeUndefined()
    expect(getTermEntry('s2')).toBeDefined()

    const w2 = vi.spyOn(getTermEntry('s2')!.term, 'write')
    writeToTerminal('s2', 'still-alive')
    expect(w2).toHaveBeenCalledWith('still-alive')
  })

  it('会话关闭后迟到的输出不会重建终端实例', async () => {
    const activeSessionID = ref('s1')
    const { setTerminalHost, ensureTerminal, writeToTerminal, disposeTerminal, getTermEntry } =
      useTerminal(activeSessionID, [], () => false)

    ensureTerminal('s1')
    setTerminalHost('s1', makeHost())
    await flushScheduling()

    disposeTerminal('s1')

    // 后端 goroutine 可能在 CloseSession 之后还冲刷出几个数据块
    for (let i = 0; i < 50; i++) writeToTerminal('s1', `late-${i}`)

    expect(getTermEntry('s1')).toBeUndefined()
  })
})
