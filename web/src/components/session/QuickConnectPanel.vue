<!-- 未连接时的快速连接列表 + 底部赞助 Banner -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import SponsorBanner from '../sponsor/SponsorBanner.vue'
import type { Connection } from '../../types'
import type { SponsorSlot } from '../../types/sponsors'

defineProps<{
  connections: Connection[]
  selectedId: string
  connTitle: (c: Connection) => string
  groupName: (c: Connection) => string
  sponsorSlot?: SponsorSlot
  proUpgradeUrl?: string
  configVersion?: number
}>()

defineEmits<{
  (e: 'select', id: string): void
  (e: 'connect', conn: Connection): void
  (e: 'clear-recents'): void
  (e: 'dismiss-sponsor', slotID: string, days: number): void
}>()

const surfaceSession = ref(`qc-${Date.now()}`)
onMounted(() => {
  surfaceSession.value = `qc-${Date.now()}`
})
</script>

<template>
  <div class="quick-panel">
    <div class="quick-header">
      <span>快速连接</span>
      <button class="quick-clear" type="button" @click="$emit('clear-recents')">清空</button>
    </div>
    <div class="quick-list">
      <div
        v-for="c in connections"
        :key="c.ID"
        class="quick-row"
        :class="{ selected: selectedId === c.ID }"
        @click="$emit('select', c.ID)"
        @dblclick="$emit('connect', c)"
      >
        <span class="quick-icon">🖥</span>
        <span class="quick-name">{{ connTitle(c) }}</span>
        <span class="quick-path">/{{ groupName(c) === '未分组' ? '' : groupName(c) }}</span>
        <span class="quick-user">{{ c.User }}</span>
      </div>
      <div v-if="connections.length === 0" class="quick-empty">
        暂无连接，点击左侧 📁 打开连接管理器添加
      </div>
    </div>
    <div v-if="sponsorSlot" class="quick-sponsor-footer">
      <SponsorBanner
        :slot="sponsorSlot"
        variant="banner"
        :pro-upgrade-url="proUpgradeUrl"
        :surface-session="surfaceSession"
        :config-version="configVersion"
        @dismiss="(id, days) => $emit('dismiss-sponsor', id, days)"
      />
    </div>
  </div>
</template>
