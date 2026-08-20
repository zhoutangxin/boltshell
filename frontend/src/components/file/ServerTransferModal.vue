<!-- 跨服务器文件传送弹窗 -->
<script setup lang="ts">
import type { Connection } from '../../types'

defineProps<{
  open: boolean
  srcPath: string
  targetConnId: string
  targetPath: string
  targetDirs: { Name: string }[]
  connections: Connection[]
  browsing: boolean
  sending: boolean
  error: string
  logs: string[]
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'update:targetConnId', v: string): void
  (e: 'target-change'): void
  (e: 'browse', dir?: string): void
  (e: 'browse-parent'): void
  (e: 'transfer'): void
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="editor-overlay" @click.self="$emit('close')">
      <div class="transfer-dialog">
        <div class="editor-header">
          <span class="editor-title">📤 传送文件到其他服务器</span>
          <button class="ft-btn editor-close-btn" @click="$emit('close')">✕</button>
        </div>
        <div class="transfer-body">
          <div class="transfer-row">
            <label>源文件：</label>
            <span class="transfer-value">{{ srcPath }}</span>
          </div>
          <div class="transfer-row">
            <label>目标服务器：</label>
            <select
              class="transfer-select"
              :value="targetConnId"
              @change="$emit('update:targetConnId', ($event.target as HTMLSelectElement).value); $emit('target-change')"
            >
              <option value="" disabled>请选择…</option>
              <option v-for="c in connections" :key="c.ID" :value="c.ID">{{ c.Name || c.Host }}</option>
            </select>
          </div>
          <div class="transfer-row">
            <label>目标路径：</label>
            <div class="transfer-browser">
              <div class="transfer-path-bar">
                <button class="ft-btn" style="padding:0 6px" :disabled="targetPath === '/'" @click="$emit('browse-parent')">⬆</button>
                <span class="transfer-cur-path">{{ targetPath }}</span>
                <button class="ft-btn" style="padding:0 6px;margin-left:auto" :disabled="browsing" @click="$emit('browse')">🔄</button>
              </div>
              <div v-if="targetConnId" class="transfer-dir-list">
                <div v-if="browsing" style="padding:8px;color:#888;">加载中…</div>
                <div v-else-if="targetDirs.length === 0" style="padding:8px;color:#888;">（无子目录）</div>
                <div
                  v-for="d in targetDirs"
                  :key="d.Name"
                  class="transfer-dir-item"
                  @dblclick="$emit('browse', targetPath === '/' ? '/' + d.Name : targetPath + '/' + d.Name)"
                >
                  📁 {{ d.Name }}
                </div>
              </div>
              <div v-else style="padding:8px;color:#888;">请先选择目标服务器</div>
            </div>
          </div>
          <div v-if="logs.length > 0" class="transfer-log-box">
            <div
              v-for="(line, i) in logs"
              :key="i"
              :class="['transfer-log-line', line.startsWith('[错误]') ? 'log-err' : line.startsWith('[完成]') ? 'log-ok' : '']"
            >{{ line }}</div>
          </div>
          <div v-if="error && !logs.length" class="editor-error">{{ error }}</div>
          <div class="transfer-actions">
            <button class="ft-btn primary" :disabled="sending || !targetConnId" @click="$emit('transfer')">
              {{ sending ? '传送中…' : '📤 开始传送' }}
            </button>
            <button class="ft-btn" @click="$emit('close')">取消</button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
