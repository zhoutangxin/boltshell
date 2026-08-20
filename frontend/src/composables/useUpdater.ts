import { onMounted, ref } from 'vue'
import { ApplyUpdate, CheckForUpdate, GetAppVersion } from '../../wailsjs/go/main/App.js'
import { UPDATE_DISMISS_KEY } from '../constants/app'
import type { UpdateCheckResult } from '../types/updater'

export function useUpdater() {
  const currentVersion = ref('')
  const updateInfo = ref<UpdateCheckResult | null>(null)
  const checking = ref(false)
  const upgrading = ref(false)
  const upgradeStatus = ref('')
  const showUpdatePrompt = ref(false)

  let upgradeTimer: ReturnType<typeof setTimeout> | null = null

  function clearUpgradeTimer() {
    if (upgradeTimer) {
      clearTimeout(upgradeTimer)
      upgradeTimer = null
    }
  }

  function isPromptDismissed(version: string) {
    if (!version) return true
    try {
      return localStorage.getItem(UPDATE_DISMISS_KEY) === version
    } catch {
      return false
    }
  }

  function dismissUpdatePrompt() {
    const ver = updateInfo.value?.LatestVersion
    if (ver) {
      try {
        localStorage.setItem(UPDATE_DISMISS_KEY, ver)
      } catch {
        /* ignore */
      }
    }
    showUpdatePrompt.value = false
  }

  async function check(silent = true) {
    checking.value = true
    try {
      currentVersion.value = await GetAppVersion()
      updateInfo.value = (await CheckForUpdate()) as UpdateCheckResult
      if (!silent && updateInfo.value.CheckError) {
        console.warn('[BoltShell] CheckForUpdate:', updateInfo.value.CheckError)
      }
    } catch (e) {
      console.error('[BoltShell] CheckForUpdate failed', e)
    } finally {
      checking.value = false
    }
  }

  async function applyUpdate(downloadURL?: string) {
    if (upgrading.value) return
    upgrading.value = true
    upgradeStatus.value = '正在下载安装包，请稍候…'
    clearUpgradeTimer()
    upgradeTimer = setTimeout(() => {
      if (upgrading.value) {
        upgrading.value = false
        upgradeStatus.value = ''
        alert('下载超时，请检查网络或服务器 releases 目录')
      }
    }, 5 * 60 * 1000)

    try {
      await ApplyUpdate(downloadURL ?? updateInfo.value?.DownloadURL ?? '')
      upgradeStatus.value = '正在重启…'
    } catch (e) {
      clearUpgradeTimer()
      upgrading.value = false
      upgradeStatus.value = ''
      const err = e instanceof Error ? e.message : String(e)
      alert(`升级失败：${err}`)
      throw e
    }
  }

  async function upgrade() {
    if (upgrading.value) return

    if (!updateInfo.value?.HasUpdate) {
      await check(false)
      const info = updateInfo.value
      if (info?.CheckError) {
        alert(`检查更新失败：${info.CheckError}`)
        return
      }
      if (!info?.HasUpdate) {
        alert(`当前已是最新版本 v${info?.CurrentVersion ?? currentVersion.value}`)
        return
      }
    }

    const info = updateInfo.value
    if (!info?.HasUpdate) return

    const msg = [
      `发现新版本 v${info.LatestVersion}`,
      info.ReleaseNotes ? `\n${info.ReleaseNotes}` : '',
      '\n\n将自动下载并重启完成升级，是否继续？',
    ].join('')

    if (!confirm(msg)) return
    showUpdatePrompt.value = false
    await applyUpdate(info.DownloadURL)
  }

  async function upgradeFromPrompt() {
    if (upgrading.value || !updateInfo.value?.HasUpdate) return
    showUpdatePrompt.value = false
    await applyUpdate(updateInfo.value.DownloadURL)
  }

  onMounted(async () => {
    await check(true)
    const info = updateInfo.value
    if (info?.HasUpdate && !isPromptDismissed(info.LatestVersion)) {
      showUpdatePrompt.value = true
    }
  })

  return {
    currentVersion,
    updateInfo,
    checking,
    upgrading,
    upgradeStatus,
    showUpdatePrompt,
    check,
    upgrade,
    upgradeFromPrompt,
    dismissUpdatePrompt,
    hasUpdate: () => !!updateInfo.value?.HasUpdate,
  }
}
