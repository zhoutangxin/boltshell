<!-- 左侧系统信息面板：CPU/内存/磁盘/进程 + 赞助位 -->
<script setup lang="ts">
import SponsorBanner from '../sponsor/SponsorBanner.vue'
import type { SysInfo } from '../../types'
import type { SponsorSlot } from '../../types/sponsors'
import { formatBytes } from '../../utils/format'

defineProps<{
  sysInfo: SysInfo
  sysInfoLoading: boolean
  memPercent: number
  swapPercent: number
  diskPercent: number
  sidebarSlots?: SponsorSlot[]
  proUpgradeUrl?: string
  surfaceSession?: string
  configVersion?: number
}>()

defineEmits<{
  (e: 'dismiss-sponsor', slotID: string, days: number): void
}>()
</script>

<template>
  <aside class="sys-panel">
    <div class="sys-head">系统信息</div>
    <div class="sys-body">
      <div class="metric">
        <div class="metric-label">
          <span>CPU</span>
          <span class="metric-val">{{ sysInfoLoading && !sysInfo.MemTotal ? '…' : `${Math.round(sysInfo.CPUPercent)}%` }}</span>
        </div>
        <div class="bar">
          <div class="bar-fill cpu" :style="{ width: `${Math.min(100, sysInfo.CPUPercent)}%` }"></div>
        </div>
      </div>
      <div class="metric">
        <div class="metric-label">
          <span>内存</span>
          <span class="metric-val">
            {{ sysInfo.MemTotal ? `${formatBytes(sysInfo.MemUsed)} / ${formatBytes(sysInfo.MemTotal)}` : sysInfoLoading ? '…' : '—' }}
          </span>
        </div>
        <div class="bar">
          <div class="bar-fill mem" :style="{ width: `${memPercent}%` }"></div>
        </div>
      </div>
      <div class="metric">
        <div class="metric-label">
          <span>Swap</span>
          <span class="metric-val">{{ sysInfo.SwapTotal ? `${swapPercent}%` : sysInfoLoading ? '…' : '0%' }}</span>
        </div>
        <div class="bar">
          <div class="bar-fill swap" :style="{ width: `${swapPercent}%` }"></div>
        </div>
      </div>
      <table v-if="sysInfo.Processes.length" class="proc-table">
        <thead>
          <tr><th>内存</th><th>CPU</th><th>命令</th></tr>
        </thead>
        <tbody>
          <tr v-for="(p, idx) in sysInfo.Processes" :key="idx">
            <td class="num">{{ formatBytes(p.MemKB * 1024) }}</td>
            <td class="num">{{ p.CPUPct.toFixed(1) }}</td>
            <td class="cmd" :title="p.Command">{{ p.Command }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="sysInfo.DiskTotal" class="disk-item">
        <div class="disk-head">
          <span class="path">{{ sysInfo.DiskPath || '/' }}</span>
          <span class="pct">{{ diskPercent }}%</span>
        </div>
        <div class="size">可用 {{ formatBytes(sysInfo.DiskFree) }} / 总计 {{ formatBytes(sysInfo.DiskTotal) }}</div>
        <div class="bar disk-bar">
          <div class="bar-fill disk" :style="{ width: `${diskPercent}%` }"></div>
        </div>
      </div>
    </div>
    <div v-if="sidebarSlots?.length" class="ad-slot-group">
      <SponsorBanner
        v-for="s in sidebarSlots"
        :key="s.SlotID"
        :slot="s"
        variant="compact"
        :pro-upgrade-url="proUpgradeUrl"
        :surface-session="surfaceSession"
        :config-version="configVersion"
        @dismiss="(id, days) => $emit('dismiss-sponsor', id, days)"
      />
    </div>
  </aside>
</template>
