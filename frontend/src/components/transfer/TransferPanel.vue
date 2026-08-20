<!-- 右侧文件传输进度面板（FinalShell 风格） -->
<script setup lang="ts">
import type { TransferTask } from '../../types'

defineProps<{
  open: boolean
  downloadDir: string
  tasks: TransferTask[]
  transferPercent: (t: TransferTask) => number
  transferStatusText: (t: TransferTask) => string
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'clear-finished'): void
  (e: 'pick-download-dir'): void
  (e: 'open-download-dir'): void
}>()
</script>

<template>
  <aside v-if="open" class="transfer-panel">
    <div class="transfer-head">
      <span>文件传输</span>
      <div class="transfer-head-actions">
        <button type="button" title="清除已完成" @click="$emit('clear-finished')">清除</button>
        <button type="button" title="关闭" @click="$emit('close')">×</button>
      </div>
    </div>
    <div class="transfer-dir-row">
      <span class="transfer-dir-label">下载:</span>
      <span class="transfer-dir-path" :title="downloadDir || '未设置默认目录'" @click="$emit('pick-download-dir')">
        {{ downloadDir || '点击选择目录' }}
      </span>
      <button type="button" class="transfer-icon-btn" title="选择目录" @click="$emit('pick-download-dir')">📁</button>
      <button type="button" class="transfer-icon-btn" title="打开文件夹" :disabled="!downloadDir" @click="$emit('open-download-dir')">📂</button>
    </div>
    <div class="transfer-list">
      <div v-if="tasks.length === 0" class="transfer-empty">暂无传输任务，上传或下载文件后在此查看进度</div>
      <div v-for="t in tasks" :key="t.id" class="transfer-item" :class="t.status">
        <div class="transfer-item-top">
          <span class="transfer-kind" :class="t.kind">{{ t.kind === 'upload' ? '上传' : '下载' }}</span>
          <span class="transfer-name" :title="t.fileName">{{ t.fileName }}</span>
          <span class="transfer-pct">{{ transferPercent(t) }}%</span>
        </div>
        <div class="transfer-progress">
          <div class="transfer-progress-bar" :class="t.kind" :style="{ width: `${transferPercent(t)}%` }"></div>
        </div>
        <div class="transfer-meta">{{ transferStatusText(t) }}</div>
      </div>
    </div>
  </aside>
</template>
