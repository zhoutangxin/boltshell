<!-- 导出连接：勾选要导出的列表 -->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Connection } from '../../types'

const props = defineProps<{
  open: boolean
  groupedConnections: [string, Connection[]][]
  connTitle: (c: Connection) => string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', ids: string[]): void
}>()

const checked = ref<Record<string, boolean>>({})

const allIDs = computed(() =>
  props.groupedConnections.flatMap(([, items]) => items.map((c) => c.ID)),
)

const selectedCount = computed(() => allIDs.value.filter((id) => checked.value[id]).length)

function resetChecked() {
  const next: Record<string, boolean> = {}
  for (const id of allIDs.value) next[id] = true
  checked.value = next
}

watch(
  () => props.open,
  (open) => {
    if (open) resetChecked()
  },
)

function toggleOne(id: string, on: boolean) {
  checked.value = { ...checked.value, [id]: on }
}

function groupIDs(items: Connection[]) {
  return items.map((c) => c.ID)
}

function groupChecked(items: Connection[]) {
  const ids = groupIDs(items)
  return ids.length > 0 && ids.every((id) => checked.value[id])
}

function toggleGroup(items: Connection[], on: boolean) {
  const next = { ...checked.value }
  for (const id of groupIDs(items)) next[id] = on
  checked.value = next
}

function selectAll(on: boolean) {
  const next: Record<string, boolean> = {}
  for (const id of allIDs.value) next[id] = on
  checked.value = next
}

function confirm() {
  emit(
    'confirm',
    allIDs.value.filter((id) => checked.value[id]),
  )
}
</script>

<template>
  <div v-if="open" class="add-backdrop" @click.self="$emit('close')">
    <div class="add-modal export-modal">
      <div class="add-titlebar">
        <span>选择要导出的连接</span>
        <button class="add-close" type="button" @click="$emit('close')">×</button>
      </div>
      <div class="add-body">
        <div class="export-toolbar">
          <button class="btn" type="button" @click="selectAll(true)">全选</button>
          <button class="btn" type="button" @click="selectAll(false)">全不选</button>
          <span class="export-count">已选 {{ selectedCount }} / {{ allIDs.length }}</span>
        </div>
        <div v-if="allIDs.length === 0" class="export-empty">暂无可导出的连接</div>
        <div v-else class="export-list">
          <template v-for="[g, items] in groupedConnections" :key="g">
            <label class="export-group">
              <input
                type="checkbox"
                :checked="groupChecked(items)"
                :disabled="items.length === 0"
                @change="toggleGroup(items, ($event.target as HTMLInputElement).checked)"
              />
              <span>📁 {{ g }}</span>
              <span class="folder-count">({{ items.length }})</span>
            </label>
            <label v-for="c in items" :key="c.ID" class="export-item">
              <input
                type="checkbox"
                :checked="!!checked[c.ID]"
                @change="toggleOne(c.ID, ($event.target as HTMLInputElement).checked)"
              />
              <span class="export-title">{{ connTitle(c) }}</span>
              <span class="export-host">{{ c.Host }}:{{ c.Port }}</span>
            </label>
          </template>
        </div>
        <div class="add-actions">
          <button class="btn" type="button" @click="$emit('close')">取消</button>
          <button class="btn btn-primary" type="button" :disabled="selectedCount === 0" @click="confirm">
            导出选中
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
