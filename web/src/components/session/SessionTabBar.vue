<!-- 顶部 Session Tab 栏 + 折叠/传输按钮 -->
<script setup lang="ts">
import type { SessionTab } from '../../types'

defineProps<{
  sessions: SessionTab[]
  activeSessionId: string
  activeTransferCount: number
  sysCollapsed: boolean
  fileCollapsed: boolean
}>()

defineEmits<{
  (e: 'select', sessionID: string): void
  (e: 'close', sessionID: string): void
  (e: 'toggle-transfer'): void
  (e: 'toggle-sys'): void
  (e: 'toggle-file'): void
}>()
</script>

<template>
  <div class="top-bar">
    <div class="tabs">
      <button
        v-for="(s, i) in sessions"
        :key="s.sessionID"
        class="tab"
        :class="{ active: activeSessionId === s.sessionID, disconnected: s.closed }"
        @click="$emit('select', s.sessionID)"
      >
        <span class="dot" :class="{ off: s.closed }" />
        <span>{{ i + 1 }} {{ s.title }}</span>
        <span class="close" @click.stop="$emit('close', s.sessionID)">×</span>
      </button>
    </div>
    <div v-if="sessions.length" class="tab-actions">
      <div class="transfer-toggle-wrap">
        <button type="button" title="文件传输" class="transfer-toggle" @click="$emit('toggle-transfer')">
          ⇅
        </button>
        <span v-if="activeTransferCount" class="transfer-badge">{{ activeTransferCount > 99 ? '99+' : activeTransferCount }}</span>
      </div>
      <button type="button" title="折叠系统信息" @click="$emit('toggle-sys')">◧</button>
      <button type="button" title="折叠文件面板" @click="$emit('toggle-file')">▤</button>
    </div>
  </div>
</template>
