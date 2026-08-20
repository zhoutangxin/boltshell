<!-- 新增 / 重命名 / 指定分组弹窗 -->
<script setup lang="ts">
defineProps<{ open: boolean; title: string; groupName: string }>()
defineEmits<{
  (e: 'update:groupName', v: string): void
  (e: 'save'): void
  (e: 'close'): void
}>()
</script>

<template>
  <div v-if="open" class="add-backdrop" @click.self="$emit('close')">
    <div class="add-modal group-modal">
      <div class="add-titlebar">
        <span>{{ title }}</span>
        <button class="add-close" type="button" @click="$emit('close')">×</button>
      </div>
      <div class="add-body">
        <label class="form-field">
          <span class="field-label">分组名称</span>
          <input
            class="input"
            :value="groupName"
            placeholder="例如：公司、测试环境"
            @input="$emit('update:groupName', ($event.target as HTMLInputElement).value)"
            @keydown.enter.prevent="$emit('save')"
          />
        </label>
        <div class="add-actions">
          <button class="btn" type="button" @click="$emit('close')">取消</button>
          <button class="btn btn-primary" type="button" @click="$emit('save')">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
