<!-- 连接管理器主弹窗：分组树 + 连接列表 -->
<script setup lang="ts">
import { ref } from 'vue'
import type { Connection } from '../../types'

defineProps<{
  open: boolean
  searchText: string
  groupFilter: string
  includeDeleted: boolean
  closeAfterConnect: boolean
  selectedConnId: string
  selectedGroupName: string
  mgrMessage: string
  mgrMessageType: string
  groupOptions: string[]
  groupedConnections: [string, Connection[]][]
  collapsedGroups: Record<string, boolean>
  connTitle: (c: Connection) => string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:searchText', v: string): void
  (e: 'update:groupFilter', v: string): void
  (e: 'update:includeDeleted', v: boolean): void
  (e: 'update:closeAfterConnect', v: boolean): void
  (e: 'add'): void
  (e: 'add-group'): void
  (e: 'edit-group', g: string): void
  (e: 'toggle-selected'): void
  (e: 'refresh'): void
  (e: 'import'): void
  (e: 'export'): void
  (e: 'toggle-group', g: string): void
  (e: 'select-group', g: string): void
  (e: 'select-conn', id: string): void
  (e: 'connect', conn: Connection): void
  (e: 'edit', conn: Connection): void
  (e: 'move-conn', id: string, groupName: string): void
}>()

const draggingID = ref('')
const dropGroup = ref('')

function canDeleteSelected(selectedConnId: string, selectedGroupName: string) {
  if (selectedConnId) return true
  return !!selectedGroupName && selectedGroupName !== '未分组'
}

function groupEditTitle(g: string) {
  return g === '未分组' ? '为未分组指定名称' : '重命名分组'
}

function onDragStart(c: Connection, e: DragEvent) {
  draggingID.value = c.ID
  e.dataTransfer?.setData('text/plain', c.ID)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragEnd() {
  draggingID.value = ''
  dropGroup.value = ''
}

function onFolderDragOver(g: string, e: DragEvent) {
  if (!draggingID.value) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
  dropGroup.value = g
}

function onFolderDrop(g: string, e: DragEvent) {
  e.preventDefault()
  const id = draggingID.value || e.dataTransfer?.getData('text/plain') || ''
  draggingID.value = ''
  dropGroup.value = ''
  if (!id) return
  emit('move-conn', id, g === '未分组' ? '' : g)
}
</script>

<template>
  <div v-if="open" class="mgr-backdrop" @click.self="$emit('close')">
    <div class="conn-mgr">
      <div class="mgr-titlebar">
        <span class="title">连接管理器</span>
        <button class="win-btn close" title="关闭" @click="$emit('close')">✕</button>
      </div>

      <div class="mgr-toolbar">
        <div class="mgr-tool-group">
          <button class="mgr-tool-btn" title="新建连接" @click="$emit('add')">➕</button>
          <button class="mgr-tool-btn" title="新增分组" @click="$emit('add-group')">📁</button>
          <button
            class="mgr-tool-btn"
            title="编辑分组"
            :disabled="!selectedGroupName"
            @click="$emit('edit-group', selectedGroupName)"
          >
            ✏
          </button>
          <button
            class="mgr-tool-btn"
            title="删除/恢复"
            :disabled="!canDeleteSelected(selectedConnId, selectedGroupName)"
            @click="$emit('toggle-selected')"
          >
            🗑
          </button>
          <span class="mgr-tool-sep" />
          <button class="mgr-tool-btn" title="刷新" @click="$emit('refresh')">🔄</button>
          <button class="mgr-tool-text" type="button" title="从 JSON 导入连接" @click="$emit('import')">导入</button>
          <button class="mgr-tool-text" type="button" title="选择连接并导出" @click="$emit('export')">导出</button>
        </div>
        <div class="mgr-search-area">
          <input
            :value="searchText"
            type="text"
            placeholder="搜索连接..."
            @input="$emit('update:searchText', ($event.target as HTMLInputElement).value)"
          />
          <select
            :value="groupFilter"
            @change="$emit('update:groupFilter', ($event.target as HTMLSelectElement).value)"
          >
            <option value="">全部</option>
            <option v-for="g in groupOptions" :key="g" :value="g">{{ g }}</option>
          </select>
          <label>
            <input
              type="checkbox"
              :checked="includeDeleted"
              @change="$emit('update:includeDeleted', ($event.target as HTMLInputElement).checked)"
            />
            显示已删除
          </label>
        </div>
      </div>

      <div class="mgr-list-body">
        <div class="mgr-list-header">
          <span>名称</span>
          <span>主机</span>
          <span>端口</span>
          <span>用户名</span>
          <span class="col-action">操作</span>
        </div>
        <div v-if="groupedConnections.length === 0" class="mgr-empty">暂无连接，点击 ➕ 新建</div>
        <template v-for="[g, items] in groupedConnections" :key="g">
          <div
            class="mgr-folder"
            :class="{ collapsed: collapsedGroups[g], selected: selectedGroupName === g, 'drop-hover': dropGroup === g }"
            @click="$emit('select-group', g)"
            @dblclick="$emit('edit-group', g)"
            @dragover="onFolderDragOver(g, $event)"
            @dragleave="dropGroup === g && (dropGroup = '')"
            @drop="onFolderDrop(g, $event)"
          >
            <div class="folder-name">
              <span class="arrow" @click.stop="$emit('toggle-group', g)">▼</span>
              <span class="folder-icon">📁</span>
              <span class="folder-title">{{ g }}</span>
              <span class="folder-count">({{ items.length }})</span>
            </div>
            <span class="col-host" />
            <span class="col-port" />
            <span class="col-user" />
            <div class="row-actions">
              <button
                class="row-edit-btn"
                type="button"
                :title="groupEditTitle(g)"
                @click.stop="$emit('edit-group', g)"
              >
                ✏
              </button>
            </div>
          </div>
          <div
            v-for="c in collapsedGroups[g] ? [] : items"
            :key="c.ID"
            class="mgr-conn"
            :class="{ selected: selectedConnId === c.ID, deleted: c.Deleted === 1, dragging: draggingID === c.ID }"
            draggable="true"
            @click="$emit('select-conn', c.ID)"
            @dblclick="$emit('connect', c)"
            @dragstart="onDragStart(c, $event)"
            @dragend="onDragEnd"
          >
            <div class="conn-name">
              <span class="conn-icon" title="拖到分组可移动">🖥</span>
              <span class="conn-title">{{ connTitle(c) }}</span>
            </div>
            <span class="col-host">{{ c.Host }}</span>
            <span class="col-port">{{ c.Port }}</span>
            <span class="col-user">{{ c.User }}</span>
            <div class="row-actions" @click.stop @mousedown.stop>
              <button class="row-edit-btn" type="button" title="编辑连接" @click="$emit('edit', c)">✏</button>
            </div>
          </div>
        </template>
      </div>

      <div v-if="mgrMessage" class="mgr-status" :class="mgrMessageType">{{ mgrMessage }}</div>

      <div class="mgr-footer">
        <label class="close-after">
          <input
            type="checkbox"
            :checked="closeAfterConnect"
            @change="$emit('update:closeAfterConnect', ($event.target as HTMLInputElement).checked)"
          />
          连接后关闭窗口
        </label>
      </div>
    </div>
  </div>
</template>
