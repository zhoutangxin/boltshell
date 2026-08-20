<!-- 启动时新版本提示弹窗 -->
<script setup lang="ts">
import type { UpdateCheckResult } from '../../types/updater'

defineProps<{
  open: boolean
  info: UpdateCheckResult | null
  upgrading?: boolean
  upgradeStatus?: string
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'upgrade'): void
}>()
</script>

<template>
  <div v-if="open && info?.HasUpdate" class="add-backdrop update-prompt-backdrop">
    <div class="add-modal update-prompt-modal">
      <div class="add-titlebar">
        <span>发现新版本</span>
        <button class="add-close" type="button" title="关闭" @click="$emit('close')">×</button>
      </div>
      <div class="add-body update-prompt-body">
        <div class="update-prompt-ver">
          <span class="update-prompt-old">v{{ info.CurrentVersion }}</span>
          <span class="update-prompt-arrow">→</span>
          <span class="update-prompt-new">v{{ info.LatestVersion }}</span>
        </div>
        <p v-if="info.ReleaseNotes" class="update-prompt-notes">{{ info.ReleaseNotes }}</p>
        <p v-else class="update-prompt-notes update-prompt-notes--muted">建议升级以获得最新功能与修复。</p>
        <div class="add-actions">
          <button class="btn" type="button" :disabled="upgrading" @click="$emit('close')">稍后</button>
          <button class="btn btn-primary" type="button" :disabled="upgrading" @click="$emit('upgrade')">
            {{ upgrading ? (upgradeStatus || '升级中…') : '立即升级' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.update-prompt-modal {
  width: 400px;
}
.update-prompt-body {
  padding: 16px 20px 20px;
}
.update-prompt-ver {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  font-size: 15px;
  font-weight: 600;
}
.update-prompt-old {
  color: #64748b;
}
.update-prompt-arrow {
  color: #94a3b8;
}
.update-prompt-new {
  color: #ea580c;
}
.update-prompt-notes {
  margin: 0 0 4px;
  font-size: 13px;
  line-height: 1.55;
  color: #475569;
  white-space: pre-wrap;
}
.update-prompt-notes--muted {
  color: #94a3b8;
}
</style>
