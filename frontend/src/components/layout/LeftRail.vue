<!-- 最左侧导航栏：连接管理器 + 底部升级入口 -->
<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  mgrOpen: boolean
  hasUpdate?: boolean
  currentVersion?: string
  latestVersion?: string
  upgrading?: boolean
  upgradeStatus?: string
}>()

defineEmits<{
  (e: 'open-mgr'): void
  (e: 'upgrade'): void
}>()

const upgradeTitle = computed(() => {
  if (props.upgrading) return props.upgradeStatus || '正在升级…'
  if (props.hasUpdate) return `发现新版本 v${props.latestVersion ?? ''}，点击升级`
  return `检查更新 · 当前 v${props.currentVersion ?? '?'}`
})
</script>

<template>
  <aside class="left-rail">
    <button
      class="btn-conn-mgr"
      :class="{ active: mgrOpen }"
      title="连接管理器"
      @click="$emit('open-mgr')"
    >
      📁
    </button>

    <div class="left-rail-spacer" />

    <button
      type="button"
      class="btn-upgrade"
      :class="{ 'has-update': hasUpdate, upgrading }"
      :title="upgradeTitle"
      :disabled="upgrading"
      @click="$emit('upgrade')"
    >
      <span class="upgrade-icon" aria-hidden="true">⬆</span>
      <span class="upgrade-label">升级</span>
      <span v-if="hasUpdate" class="upgrade-badge">新</span>
      <span v-if="hasUpdate && latestVersion" class="upgrade-ver">v{{ latestVersion }}</span>
    </button>
  </aside>
</template>
