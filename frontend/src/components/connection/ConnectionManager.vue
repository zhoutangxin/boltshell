<!-- 连接管理器主弹窗：分组树 + 连接列表 -->
<script setup lang="ts">
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

defineEmits<{
  (e: 'close'): void
  (e: 'update:searchText', v: string): void
  (e: 'update:groupFilter', v: string): void
  (e: 'update:includeDeleted', v: boolean): void
  (e: 'update:closeAfterConnect', v: boolean): void
  (e: 'add'): void
  (e: 'add-group'): void
  (e: 'toggle-selected'): void
  (e: 'refresh'): void
  (e: 'toggle-group', g: string): void
  (e: 'select-group', g: string): void
  (e: 'select-conn', id: string): void
  (e: 'connect', conn: Connection): void
  (e: 'edit', conn: Connection): void
}>()
</script>

<template>
  <div v-if="open" class="mgr-backdrop" @click.self="$emit('close')">
    <div class="conn-mgr">
      <div class="mgr-titlebar">
        <span class="title">连接管理器</span>
        <button class="win-btn close" title="关闭" @click="$emit('close')">✕</button>
      </div>

      <div class="mgr-toolbar">
        <button class="mgr-tool-btn" title="新建连接" @click="$emit('add')">➕</button>
        <button class="mgr-tool-btn" title="新增分组" @click="$emit('add-group')">📁</button>
        <button class="mgr-tool-btn" title="删除/恢复" :disabled="!selectedConnId" @click="$emit('toggle-selected')">🗑</button>
        <span class="mgr-tool-sep" />
        <button class="mgr-tool-btn" title="刷新" @click="$emit('refresh')">🔄</button>
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

      <div class="mgr-list-header">
        <span>名称</span><span>主机</span><span>端口</span><span>用户名</span><span class="col-action">操作</span>
      </div>

      <div class="mgr-list-body">
        <div v-if="groupedConnections.length === 0" class="mgr-empty">暂无连接，点击 ➕ 新建</div>
        <template v-for="[g, items] in groupedConnections" :key="g">
          <div
            class="mgr-folder"
            :class="{ collapsed: collapsedGroups[g], selected: selectedGroupName === g }"
            @click="$emit('select-group', g)"
          >
            <div class="folder-name">
              <span class="arrow" @click.stop="$emit('toggle-group', g)">▼</span>
              <span>📁 {{ g }}</span>
              <span class="folder-count">({{ items.length }})</span>
            </div>
          </div>
          <div
            v-for="c in collapsedGroups[g] ? [] : items"
            :key="c.ID"
            class="mgr-conn"
            :class="{ selected: selectedConnId === c.ID, deleted: c.Deleted === 1 }"
            @click="$emit('select-conn', c.ID)"
            @dblclick="$emit('connect', c)"
          >
            <div class="conn-name">
              <span class="conn-icon">🖥</span>
              <span class="conn-title">{{ connTitle(c) }}</span>
            </div>
            <span class="col-host">{{ c.Host }}</span>
            <span class="col-port">{{ c.Port }}</span>
            <span class="col-user">{{ c.User }}</span>
            <button class="row-edit-btn" type="button" title="编辑" @click.stop="$emit('edit', c)">✏</button>
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
