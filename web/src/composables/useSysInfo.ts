/**
 * 远端系统信息轮询 composable
 * 每 5 秒通过 GetSessionSysInfo 刷新 CPU/内存/磁盘/进程
 */

import { computed, onUnmounted, ref, type Ref } from 'vue'
import { GetSessionSysInfo } from '../../wailsjs/go/main/App.js'
import type { SessionTab, SysInfo } from '../types'
import { emptySysInfo } from '../utils/format'

export function useSysInfo(
  activeSessionID: Ref<string>,
  sessions: SessionTab[],
) {
  const sysInfo = ref<SysInfo>(emptySysInfo())
  const sysInfoLoading = ref(false)
  let sysInfoTimer: ReturnType<typeof setInterval> | null = null

  const memPercent = computed(() => {
    const s = sysInfo.value
    if (!s.MemTotal) return 0
    return Math.min(100, Math.round((s.MemUsed / s.MemTotal) * 100))
  })

  const swapPercent = computed(() => {
    const s = sysInfo.value
    if (!s.SwapTotal) return 0
    return Math.min(100, Math.round((s.SwapUsed / s.SwapTotal) * 100))
  })

  const diskPercent = computed(() => {
    const s = sysInfo.value
    if (!s.DiskTotal) return 0
    return Math.min(100, Math.round((s.DiskUsed / s.DiskTotal) * 100))
  })

  async function refreshSysInfo(sessionID?: string) {
    const sid = sessionID || activeSessionID.value
    if (!sid) {
      sysInfo.value = emptySysInfo()
      return
    }
    const tab = sessions.find((s) => s.sessionID === sid)
    if (!tab || tab.closed) return
    sysInfoLoading.value = true
    try {
      const res = await GetSessionSysInfo(sid)
      if (sid !== activeSessionID.value) return
      sysInfo.value = {
        CPUPercent: res.CPUPercent ?? 0,
        MemTotal: res.MemTotal ?? 0,
        MemUsed: res.MemUsed ?? 0,
        SwapTotal: res.SwapTotal ?? 0,
        SwapUsed: res.SwapUsed ?? 0,
        DiskTotal: res.DiskTotal ?? 0,
        DiskUsed: res.DiskUsed ?? 0,
        DiskFree: res.DiskFree ?? 0,
        DiskPath: res.DiskPath || '/',
        Processes: Array.isArray(res.Processes) ? res.Processes : [],
      }
    } catch (e) {
      console.error('[BoltShell] GetSessionSysInfo failed', e)
    } finally {
      sysInfoLoading.value = false
    }
  }

  function startSysInfoPoll() {
    stopSysInfoPoll()
    sysInfoTimer = setInterval(() => {
      refreshSysInfo().catch(console.error)
    }, 5000)
  }

  function stopSysInfoPoll() {
    if (sysInfoTimer) {
      clearInterval(sysInfoTimer)
      sysInfoTimer = null
    }
  }

  onUnmounted(stopSysInfoPoll)

  return {
    sysInfo,
    sysInfoLoading,
    memPercent,
    swapPercent,
    diskPercent,
    refreshSysInfo,
    startSysInfoPoll,
    stopSysInfoPoll,
  }
}
