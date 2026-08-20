/**
 * 多会话 Tab 管理测试：SSH 支持同时开多个窗口，
 * 同一台机器也允许开多个互不干扰的会话。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Connection } from '../types'

let nextSessionID = 0
const startSession = vi.fn((_connID: string) => Promise.resolve(`sid-${++nextSessionID}`))
const closeSession = vi.fn((_sessionID: string) => Promise.resolve())

vi.mock('../../wailsjs/go/main/App.js', () => ({
  StartSession: (connID: string) => startSession(connID),
  CloseSession: (sessionID: string) => closeSession(sessionID),
  ListConnections: () => Promise.resolve([]),
  ListConnectionGroups: () => Promise.resolve([]),
  AddConnection: () => Promise.resolve(''),
  AddConnectionGroup: () => Promise.resolve({}),
  UpdateConnection: () => Promise.resolve(),
  SetDeleted: () => Promise.resolve(),
  RenameConnectionGroupByName: () => Promise.resolve(),
  AssignUngroupedToGroup: () => Promise.resolve(0),
  DeleteConnectionGroupByName: () => Promise.resolve(0),
  ExportConnections: () => Promise.resolve(''),
  MoveConnection: () => Promise.resolve(),
  ImportConnections: () => Promise.resolve({
    GroupsAdded: 0,
    ConnectionsAdded: 0,
    ConnectionsSkip: 0,
    ConnectionsUpdated: 0,
  }),
}))

const { useConnections } = await import('./useConnections')

function makeConn(id: string, name: string): Connection {
  return {
    ID: id,
    Name: name,
    Host: `10.0.0.${id}`,
    Port: 22,
    User: 'root',
    Password: '',
    GroupID: '',
    GroupName: '',
    Enabled: 1,
    Deleted: 0,
  } as Connection
}

describe('多会话 Tab', () => {
  beforeEach(() => {
    nextSessionID = 0
    startSession.mockClear()
    closeSession.mockClear()
    localStorage.clear()
  })

  it('同一个连接可以开多个独立会话', async () => {
    const api = useConnections()
    const conn = makeConn('1', 'server-a')
    const opened: string[] = []

    await api.onConnect(conn, async (sid) => {
      opened.push(sid)
    })
    await api.onConnect(conn, async (sid) => {
      opened.push(sid)
    })

    expect(api.sessions).toHaveLength(2)
    expect(opened).toEqual(['sid-1', 'sid-2'])
    // 每个 Tab 必须拿到不同的后端会话
    expect(new Set(api.sessions.map((s) => s.sessionID)).size).toBe(2)
    expect(api.activeSessionID.value).toBe('sid-2')
  })

  it('关闭当前 Tab 后激活相邻的那个，而不是最后一个', async () => {
    const api = useConnections()
    for (const c of [makeConn('1', 'a'), makeConn('2', 'b'), makeConn('3', 'c')]) {
      await api.onConnect(c, async () => {})
    }

    api.activeSessionID.value = 'sid-1'
    await api.onCloseTab('sid-1', () => {}, () => {})

    expect(api.sessions.map((s) => s.sessionID)).toEqual(['sid-2', 'sid-3'])
    expect(api.activeSessionID.value).toBe('sid-2')
  })

  it('关闭最后一个 Tab 时回落到它左边的 Tab', async () => {
    const api = useConnections()
    for (const c of [makeConn('1', 'a'), makeConn('2', 'b')]) {
      await api.onConnect(c, async () => {})
    }

    await api.onCloseTab('sid-2', () => {}, () => {})

    expect(api.activeSessionID.value).toBe('sid-1')
  })

  it('关闭非当前 Tab 不会切走当前会话', async () => {
    const api = useConnections()
    for (const c of [makeConn('1', 'a'), makeConn('2', 'b')]) {
      await api.onConnect(c, async () => {})
    }

    expect(api.activeSessionID.value).toBe('sid-2')
    await api.onCloseTab('sid-1', () => {}, () => {})

    expect(api.activeSessionID.value).toBe('sid-2')
    expect(api.sessions).toHaveLength(1)
  })

  it('关闭 Tab 会同时关掉后端会话并释放前端终端', async () => {
    const api = useConnections()
    await api.onConnect(makeConn('1', 'a'), async () => {})

    const disposed: string[] = []
    await api.onCloseTab('sid-1', (sid) => disposed.push(sid), () => {})

    expect(closeSession).toHaveBeenCalledWith('sid-1')
    expect(disposed).toEqual(['sid-1'])
    expect(api.sessions).toHaveLength(0)
    expect(api.activeSessionID.value).toBe('')
  })

  it('后端 CloseSession 失败仍然清理前端 Tab', async () => {
    closeSession.mockImplementationOnce(() => Promise.reject(new Error('session not found')))
    const api = useConnections()
    await api.onConnect(makeConn('1', 'a'), async () => {})

    const disposed: string[] = []
    await api.onCloseTab('sid-1', (sid) => disposed.push(sid), () => {})

    expect(disposed).toEqual(['sid-1'])
    expect(api.sessions).toHaveLength(0)
  })

  it('连接失败不会残留 Tab', async () => {
    startSession.mockImplementationOnce(() => Promise.reject(new Error('dial tcp: timeout')))
    const api = useConnections()

    await api.onConnect(makeConn('1', 'a'), async () => {})

    expect(api.sessions).toHaveLength(0)
    expect(api.activeSessionID.value).toBe('')
  })
})
