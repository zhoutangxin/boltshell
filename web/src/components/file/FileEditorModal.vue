<!-- 远端文件在线编辑器弹窗 -->
<script setup lang="ts">
defineProps<{
  open: boolean
  path: string
  content: string
  original: string
  loading: boolean
  saving: boolean
  error: string
}>()

defineEmits<{
  (e: 'update:content', v: string): void
  (e: 'save'): void
  (e: 'close'): void
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="editor-overlay" @click.self="$emit('close')">
      <div class="editor-dialog">
        <div class="editor-header">
          <span class="editor-title">📝 {{ path }}</span>
          <div class="editor-actions">
            <span v-if="content !== original" class="editor-modified">● 已修改</span>
            <button
              class="ft-btn primary"
              :disabled="saving || loading || content === original"
              @click="$emit('save')"
            >
              {{ saving ? '保存中…' : '💾 保存' }}
            </button>
            <button class="ft-btn editor-close-btn" @click="$emit('close')">✕ 关闭</button>
          </div>
        </div>
        <div v-if="error" class="editor-error">{{ error }}</div>
        <div v-if="loading" class="editor-loading">加载中…</div>
        <textarea
          v-else
          :value="content"
          class="editor-textarea"
          spellcheck="false"
          wrap="off"
          @input="$emit('update:content', ($event.target as HTMLTextAreaElement).value)"
        />
      </div>
    </div>
  </Teleport>
</template>
