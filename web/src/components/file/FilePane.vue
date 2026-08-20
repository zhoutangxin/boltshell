<!-- 底部 SFTP 文件面板：工具栏 + 目录树 + 文件列表 -->
<script setup lang="ts">
import type { FilePaneState } from '../../types'
import FileEditorModal from './FileEditorModal.vue'
import ServerTransferModal from './ServerTransferModal.vue'

defineProps<{
  visible: boolean
  height: number
  fileTab: 'files' | 'cmd'
  fileState: FilePaneState | null
  fileTreePaths: string[]
  pathInputDraft: string
  canDownload: boolean
  formatFileSize: (n: number) => string
  formatModTime: (ts: number) => string
  editor: {
    open: boolean
    path: string
    content: string
    original: string
    loading: boolean
    saving: boolean
    error: string
  }
  serverTransfer: {
    open: boolean
    srcPath: string
    targetConnID: string
    targetPath: string
    targetDirs: { Name: string }[]
    connections: import('../../types').Connection[]
    browsing: boolean
    sending: boolean
    error: string
    logs: string[]
  }
}>()

defineEmits<{
  (e: 'update:fileTab', v: 'files' | 'cmd'): void
  (e: 'update:pathInputDraft', v: string): void
  (e: 'update:editorContent', v: string): void
  (e: 'update:targetConnId', v: string): void
  (e: 'update:targetPath', v: string): void
  (e: 'split-drag', ev: MouseEvent): void
  (e: 'go-up'): void
  (e: 'refresh'): void
  (e: 'upload', pickDir: boolean): void
  (e: 'download'): void
  (e: 'mkdir'): void
  (e: 'delete'): void
  (e: 'rename'): void
  (e: 'open-transfer'): void
  (e: 'path-focus', ev: FocusEvent): void
  (e: 'path-blur'): void
  (e: 'path-keydown', ev: KeyboardEvent): void
  (e: 'navigate', path: string): void
  (e: 'select-file', name: string): void
  (e: 'open-entry', name: string): void
  (e: 'save-editor'): void
  (e: 'close-editor'): void
  (e: 'close-transfer'): void
  (e: 'transfer-target-change'): void
  (e: 'browse-target', dir?: string): void
  (e: 'browse-parent'): void
  (e: 'do-transfer'): void
}>()
</script>

<template>
  <template v-if="visible">
    <div class="splitter" @mousedown.prevent="$emit('split-drag', $event)" />

    <div class="file-pane" :style="{ height: height + 'px' }">
      <div class="file-tabs">
        <div class="file-tab" :class="{ active: fileTab === 'files' }" @click="$emit('update:fileTab', 'files')">文件</div>
        <div class="file-tab" :class="{ active: fileTab === 'cmd' }" @click="$emit('update:fileTab', 'cmd')">命令</div>
      </div>

      <template v-if="fileTab === 'files' && fileState">
        <div class="file-toolbar">
          <button class="ft-btn" type="button" @click="$emit('go-up')">⬆ 上级</button>
          <button class="ft-btn" type="button" @click="$emit('refresh')">🔄 刷新</button>
          <span class="ft-sep" />
          <button class="ft-btn primary" type="button" @click="$emit('upload', false)">⬆ 上传</button>
          <button class="ft-btn" type="button" @click="$emit('upload', true)">📁 上传文件夹</button>
          <button class="ft-btn" type="button" :disabled="!canDownload" @click="$emit('download')">⬇ 下载</button>
          <button class="ft-btn" type="button" @click="$emit('mkdir')">📁 新建文件夹</button>
          <button class="ft-btn" type="button" :disabled="!fileState.selected" @click="$emit('delete')">🗑 删除</button>
          <button class="ft-btn" type="button" :disabled="!fileState.selected" @click="$emit('rename')">✏ 重命名</button>
          <button class="ft-btn" type="button" :disabled="!fileState.selected" @click="$emit('open-transfer')">📤 传送</button>
          <span class="ft-sep" />
          <span class="ft-tag">SFTP</span>
          <div class="path-bar">
            <input
              class="path-input"
              :value="pathInputDraft"
              spellcheck="false"
              @input="$emit('update:pathInputDraft', ($event.target as HTMLInputElement).value)"
              @keydown="$emit('path-keydown', $event)"
              @focus="$emit('path-focus', $event)"
              @blur="$emit('path-blur')"
            />
          </div>
        </div>
        <div class="file-body">
          <div class="dir-tree">
            <div
              v-for="p in fileTreePaths"
              :key="p"
              class="tree-node"
              :class="{ active: fileState.path === p }"
              :style="{ paddingLeft: 8 + (p.split('/').filter(Boolean).length) * 12 + 'px' }"
              @click="$emit('navigate', p)"
            >
              <span class="tree-arrow">▾</span>📁 {{ p === '/' ? '/' : p.split('/').pop() }}
            </div>
          </div>
          <div class="file-list-wrap">
            <div class="file-list-header">
              <span>名称</span><span>大小</span><span>类型</span><span>修改时间</span><span>权限</span><span>用户</span>
            </div>
            <div class="file-list-body">
              <div v-if="fileState.loading" class="file-loading">加载中…</div>
              <div v-if="fileState.path !== '/'" class="file-row dir" @dblclick="$emit('navigate', '..')">
                <div class="fname">📁 <span>..</span></div>
                <span class="muted">-</span><span class="muted">上级</span><span class="muted">-</span><span class="muted">-</span><span class="muted">-</span>
              </div>
              <div
                v-for="f in fileState.files"
                :key="f.Path"
                class="file-row"
                :class="{ dir: f.IsDir, selected: fileState.selected === f.Name }"
                @click="$emit('select-file', f.Name)"
                @dblclick="$emit('open-entry', f.Name)"
              >
                <div class="fname">{{ f.IsDir ? '📁' : '📄' }} <span>{{ f.Name }}</span></div>
                <span class="muted">{{ f.IsDir ? '-' : formatFileSize(f.Size) }}</span>
                <span class="muted">{{ f.IsDir ? '文件夹' : '文件' }}</span>
                <span class="muted">{{ formatModTime(f.ModTime) }}</span>
                <span class="muted">{{ f.Mode }}</span>
                <span class="muted">{{ f.Owner }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="file-status">
          <span>协议: SFTP（与 SSH 同连接）</span>
          <span>{{ fileState.status || '就绪' }}</span>
        </div>

        <FileEditorModal
          :open="editor.open"
          :path="editor.path"
          :content="editor.content"
          :original="editor.original"
          :loading="editor.loading"
          :saving="editor.saving"
          :error="editor.error"
          @update:content="$emit('update:editorContent', $event)"
          @save="$emit('save-editor')"
          @close="$emit('close-editor')"
        />

        <ServerTransferModal
          :open="serverTransfer.open"
          :src-path="serverTransfer.srcPath"
          :target-conn-id="serverTransfer.targetConnID"
          :target-path="serverTransfer.targetPath"
          :target-dirs="serverTransfer.targetDirs"
          :connections="serverTransfer.connections"
          :browsing="serverTransfer.browsing"
          :sending="serverTransfer.sending"
          :error="serverTransfer.error"
          :logs="serverTransfer.logs"
          @close="$emit('close-transfer')"
          @update:target-conn-id="$emit('update:targetConnId', $event)"
          @update:target-path="$emit('update:targetPath', $event)"
          @target-change="$emit('transfer-target-change')"
          @browse="$emit('browse-target', $event)"
          @browse-parent="$emit('browse-parent')"
          @transfer="$emit('do-transfer')"
        />
      </template>

      <div v-else class="cmd-panel active">
        <div class="cmd-hint">批量命令（后续版本支持多机执行）</div>
        <textarea readonly>systemctl status sshd
df -h</textarea>
      </div>
    </div>
  </template>
</template>
