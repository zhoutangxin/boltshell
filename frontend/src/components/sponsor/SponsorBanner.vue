<!-- 可配置赞助位：banner（快速连接底）/ compact（侧栏） -->
<script setup lang="ts">
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import type { SponsorSlot } from '../../types/sponsors'

const props = defineProps<{
  slot: SponsorSlot
  variant: 'banner' | 'compact'
  proUpgradeUrl?: string
}>()

const emit = defineEmits<{
  (e: 'dismiss', slotID: string, days: number): void
}>()

function openLink() {
  const url = props.slot.linkUrl?.trim()
  if (url) BrowserOpenURL(url)
}

function onDismiss(ev: MouseEvent) {
  ev.stopPropagation()
  emit('dismiss', props.slot.SlotID, props.slot.dismissDays ?? 7)
}
</script>

<template>
  <div class="sponsor-wrap" :class="`sponsor-wrap--${variant}`">
    <button
      type="button"
      class="ad-slot"
      :class="variant === 'banner' ? 'ad-slot--quick' : 'ad-slot--sidebar'"
      @click="openLink"
    >
      <span v-if="variant === 'compact' && slot.badge" class="ad-badge">{{ slot.badge }}</span>
      <span v-if="variant === 'banner'" class="ad-quick-icon">{{ slot.badge || '⚡' }}</span>
      <span :class="variant === 'banner' ? 'ad-quick-body' : ''">
        <span class="ad-title">{{ slot.title }}</span>
        <span v-if="slot.desc && variant !== 'banner'" class="ad-desc">{{ slot.desc }}</span>
        <span v-if="slot.desc && variant === 'banner'" class="ad-desc ad-desc--inline">{{ slot.desc }}</span>
      </span>
      <img v-if="slot.imageUrl" :src="slot.imageUrl" alt="" class="ad-image" />
    </button>
    <button type="button" class="sponsor-dismiss" title="暂时关闭" @click="onDismiss">×</button>
  </div>
</template>

<style scoped>
.sponsor-wrap {
  position: relative;
}
.sponsor-wrap--banner {
  flex-shrink: 0;
}
.sponsor-dismiss {
  position: absolute;
  top: 4px;
  right: 6px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.06);
  color: #64748b;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
}
.sponsor-wrap:hover .sponsor-dismiss {
  opacity: 1;
}
.ad-image {
  max-height: 48px;
  margin-left: auto;
}
</style>
