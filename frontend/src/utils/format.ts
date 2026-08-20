/**
 * 格式化与错误文本工具
 */

import type { Connection } from '../types'

/** 字节数 → 人类可读（B/K/M/G/T） */
export function formatBytes(n: number): string {
  if (!n || n <= 0) return '0'
  const units = ['B', 'K', 'M', 'G', 'T']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const digits = v >= 100 || i === 0 ? 0 : 1
  return `${v.toFixed(digits)}${units[i]}`
}

/** SFTP 文件列表专用大小格式 */
export function formatFileSize(n: number): string {
  if (n < 0) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

/** Unix 时间戳 → 本地日期时间字符串 */
export function formatModTime(ts: number): string {
  if (!ts) return '-'
  const d = new Date(ts * 1000)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}/${p(d.getMonth() + 1)}/${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 连接显示标题：优先名称，否则 host:port */
export function connTitle(c: Connection): string {
  if (c.Name?.trim()) return c.Name
  return `${c.Host}:${c.Port}`
}

/** 连接分组名，空则「未分组」 */
export function groupName(c: Connection): string {
  return c.GroupName?.trim() || '未分组'
}

/** 统一错误文本提取（Wails/Go error） */
export function errText(e: unknown): string {
  if (e instanceof Error && e.message) return e.message
  if (typeof e === 'string') return e
  try {
    return JSON.stringify(e)
  } catch {
    return '未知错误'
  }
}

/** 拼接本地路径 */
export function joinLocalPath(dir: string, name: string): string {
  const sep = dir.includes('\\') ? '\\' : '/'
  return dir.replace(/[/\\]+$/, '') + sep + name
}

/** 空系统信息初始值 */
export function emptySysInfo() {
  return {
    CPUPercent: 0,
    MemTotal: 0,
    MemUsed: 0,
    SwapTotal: 0,
    SwapUsed: 0,
    DiskTotal: 0,
    DiskUsed: 0,
    DiskFree: 0,
    DiskPath: '/',
    Processes: [] as { MemKB: number; CPUPct: number; Command: string }[],
  }
}
