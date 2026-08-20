/**
 * 后端事件订阅回归测试
 *
 * 历史 bug：App.vue 用 EventsOnMultiple(name, cb, 1) 订阅流式事件，
 * 而 maxCallbacks=1 等同于 EventsOnce —— 终端只渲染第一个数据块，
 * 第二个 Tab 全空白，传输进度也只更新一次。
 *
 * 下面的假 runtime 严格实现 Wails 的 maxCallbacks 语义（达到上限自动注销），
 * 所以一旦有人改回 EventsOnMultiple(..., 1) 这些用例就会红。
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'

type Listener = { cb: (...args: unknown[]) => void; remaining: number }

const registry = new Map<string, Listener[]>()

function fakeEventsOnMultiple(name: string, cb: (...a: unknown[]) => void, maxCallbacks: number) {
  const listener: Listener = { cb, remaining: maxCallbacks }
  const list = registry.get(name) ?? []
  list.push(listener)
  registry.set(name, list)
  return () => {
    const cur = registry.get(name)
    if (!cur) return
    const i = cur.indexOf(listener)
    if (i >= 0) cur.splice(i, 1)
  }
}

/** 模拟后端推送一次事件，遵守 maxCallbacks 到期自动注销 */
function emit(name: string, ...args: unknown[]) {
  for (const listener of [...(registry.get(name) ?? [])]) {
    listener.cb(...args)
    if (listener.remaining < 0) continue
    listener.remaining -= 1
    if (listener.remaining <= 0) {
      const cur = registry.get(name)!
      const i = cur.indexOf(listener)
      if (i >= 0) cur.splice(i, 1)
    }
  }
}

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOnMultiple: (name: string, cb: (...a: unknown[]) => void, max: number) =>
    fakeEventsOnMultiple(name, cb, max),
  EventsOn: (name: string, cb: (...a: unknown[]) => void) => fakeEventsOnMultiple(name, cb, -1),
  EventsOff: (...names: string[]) => {
    for (const n of names) registry.delete(n)
  },
}))

const { subscribeBackendEvents, BACKEND_EVENTS } = await import('./useBackendEvents')

function makeHandlers() {
  return {
    onTerminalOutput: vi.fn(),
    onTerminalClosed: vi.fn(),
    onTransferUpdate: vi.fn(),
    onTransferLog: vi.fn(),
    onServerTransferProgress: vi.fn(),
  }
}

describe('subscribeBackendEvents', () => {
  beforeEach(() => registry.clear())

  it('terminal-output 是持续订阅，不会在第一条之后失效', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)

    for (let i = 0; i < 20; i++) emit('terminal-output', 'sid-1', `chunk-${i}`)

    expect(h.onTerminalOutput).toHaveBeenCalledTimes(20)
    expect(h.onTerminalOutput).toHaveBeenLastCalledWith('sid-1', 'chunk-19')
  })

  it('多个会话的输出互不干扰，都能持续收到', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)

    emit('terminal-output', 'sid-1', 'a')
    emit('terminal-output', 'sid-2', 'b')
    emit('terminal-output', 'sid-1', 'c')
    emit('terminal-output', 'sid-2', 'd')

    const bySession = (sid: string) =>
      h.onTerminalOutput.mock.calls.filter((c) => c[0] === sid).map((c) => c[1])
    expect(bySession('sid-1')).toEqual(['a', 'c'])
    expect(bySession('sid-2')).toEqual(['b', 'd'])
  })

  it('传输进度类事件同样是持续订阅', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)

    for (let i = 0; i < 5; i++) emit('transfer-update', { TaskID: 't1', Total: 100, Transferred: i })
    for (let i = 0; i < 5; i++) emit('transfer-log', `line-${i}`)
    for (let i = 0; i < 5; i++) emit('srv-transfer-progress', { TaskID: 't1', Total: 100, Transferred: i })

    expect(h.onTransferUpdate).toHaveBeenCalledTimes(5)
    expect(h.onTransferLog).toHaveBeenCalledTimes(5)
    expect(h.onServerTransferProgress).toHaveBeenCalledTimes(5)
    expect(h.onServerTransferProgress).toHaveBeenLastCalledWith('t1', 100, 4)
  })

  it('terminal-closed 能为每个会话分别触发', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)

    emit('terminal-closed', 'sid-1')
    emit('terminal-closed', 'sid-2')

    expect(h.onTerminalClosed.mock.calls.map((c) => c[0])).toEqual(['sid-1', 'sid-2'])
  })

  it('重复订阅不会导致输出翻倍（热更新场景）', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)
    subscribeBackendEvents(h)

    emit('terminal-output', 'sid-1', 'x')

    expect(h.onTerminalOutput).toHaveBeenCalledTimes(1)
  })

  it('取消订阅后不再收到事件', () => {
    const h = makeHandlers()
    const unsubscribe = subscribeBackendEvents(h)

    emit('terminal-output', 'sid-1', 'before')
    unsubscribe()
    emit('terminal-output', 'sid-1', 'after')

    expect(h.onTerminalOutput).toHaveBeenCalledTimes(1)
    for (const name of BACKEND_EVENTS) {
      expect(registry.get(name) ?? []).toHaveLength(0)
    }
  })

  it('缺少 sessionID 的事件被忽略', () => {
    const h = makeHandlers()
    subscribeBackendEvents(h)

    emit('terminal-output', '', 'data')
    emit('terminal-closed', '')
    emit('transfer-update', 'not-an-object')
    emit('transfer-log', 123)
    emit('srv-transfer-progress', { Total: 1 })

    expect(h.onTerminalOutput).not.toHaveBeenCalled()
    expect(h.onTerminalClosed).not.toHaveBeenCalled()
    expect(h.onTransferUpdate).not.toHaveBeenCalled()
    expect(h.onTransferLog).not.toHaveBeenCalled()
    expect(h.onServerTransferProgress).not.toHaveBeenCalled()
  })
})
