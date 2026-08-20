/**
 * BoltShell 前端共享类型定义
 * 与 Wails 生成的 Go struct JSON 字段保持一致
 */

/** SSH 连接配置（对应 db.Connection） */
export type Connection = {
  ID: string
  Name: string
  Host: string
  Port: number
  User: string
  Password: string
  GroupName: string
  Enabled: number
  Deleted: number
  CreatedAt: number
}

/** 已打开的 SSH 会话 Tab */
export type SessionTab = {
  sessionID: string
  connID: string
  title: string
  closed: boolean
}

/** SFTP 远端文件/目录条目 */
export type RemoteEntry = {
  Name: string
  Path: string
  Size: number
  IsDir: boolean
  ModTime: number
  Mode: string
  Owner: string
}

/** 每个会话独立的 SFTP 文件面板状态 */
export type FilePaneState = {
  path: string
  home: string
  files: RemoteEntry[]
  selected: string
  loading: boolean
  status: string
}

/** 远端进程信息（系统监控） */
export type SysProcInfo = {
  MemKB: number
  CPUPct: number
  Command: string
}

/** 远端系统资源快照 */
export type SysInfo = {
  CPUPercent: number
  MemTotal: number
  MemUsed: number
  SwapTotal: number
  SwapUsed: number
  DiskTotal: number
  DiskUsed: number
  DiskFree: number
  DiskPath: string
  Processes: SysProcInfo[]
}

/** 本地上传/下载传输任务 */
export type TransferTask = {
  id: string
  sessionID: string
  kind: 'upload' | 'download'
  fileName: string
  source: string
  dest: string
  total: number
  transferred: number
  status: 'running' | 'done' | 'error'
  error: string
  updatedAt: number
}

/** 连接管理器 UI 状态 */
export type MgrMessageType = '' | 'success' | 'error' | 'loading'

/** 赞助/广告位配置（旧版，已由远程 sponsors.json 替代） */
export type AdSlot = {
  id: string
  badge: string
  title: string
  desc: string
  url: string
}
