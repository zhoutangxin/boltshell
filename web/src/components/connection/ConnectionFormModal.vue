<!-- 新建/编辑 SSH 连接表单弹窗 -->
<script setup lang="ts">
defineProps<{
  open: boolean
  editing: boolean
  form: {
    name: string
    host: string
    port: string
    user: string
    password: string
    groupName: string
    enabled: boolean
  }
  showPassword: boolean
  showGroupMenu: boolean
  filteredGroups: string[]
}>()

defineEmits<{
  (e: 'update:form', v: Record<string, unknown>): void
  (e: 'update:showPassword', v: boolean): void
  (e: 'update:showGroupMenu', v: boolean): void
  (e: 'pick-group', g: string): void
  (e: 'save'): void
  (e: 'close'): void
}>()

function patch(field: string, value: unknown, form: object) {
  return { ...form, [field]: value }
}
</script>

<template>
  <div v-if="open" class="add-backdrop" @click.self="$emit('close')">
    <div class="add-modal">
      <div class="add-titlebar">
        <span>{{ editing ? '编辑连接' : '新建连接' }}</span>
        <button class="add-close" type="button" @click="$emit('close')">×</button>
      </div>
      <div class="add-body">
        <div class="form-grid">
          <label class="form-field">
            <span class="field-label">名称</span>
            <input
              class="input"
              :value="form.name"
              placeholder="可选，默认使用主机"
              @input="$emit('update:form', patch('name', ($event.target as HTMLInputElement).value, form))"
            />
          </label>
          <label class="form-field">
            <span class="field-label">主机 <span class="req">*</span></span>
            <input
              class="input"
              :value="form.host"
              placeholder="192.168.1.10"
              @input="$emit('update:form', patch('host', ($event.target as HTMLInputElement).value, form))"
            />
          </label>
          <label class="form-field">
            <span class="field-label">端口</span>
            <input
              class="input"
              :value="form.port"
              @input="$emit('update:form', patch('port', ($event.target as HTMLInputElement).value, form))"
            />
          </label>
          <label class="form-field">
            <span class="field-label">用户名 <span class="req">*</span></span>
            <input
              class="input"
              :value="form.user"
              @input="$emit('update:form', patch('user', ($event.target as HTMLInputElement).value, form))"
            />
          </label>
          <label class="form-field">
            <span class="field-label">密码 <span class="req">*</span></span>
            <div class="pwd-wrap">
              <input
                class="input"
                :value="form.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="必填"
                autocomplete="off"
                @input="$emit('update:form', patch('password', ($event.target as HTMLInputElement).value, form))"
              />
              <button
                class="pwd-toggle"
                type="button"
                tabindex="-1"
                :title="showPassword ? '隐藏密码' : '显示密码'"
                @click.stop="$emit('update:showPassword', !showPassword)"
              >
                {{ showPassword ? '🙈' : '👁' }}
              </button>
            </div>
          </label>
          <div class="form-field">
            <span class="field-label">分组（文件夹）</span>
            <div class="combo">
              <input
                class="input"
                :value="form.groupName"
                placeholder="输入新分组，或点选已有"
                autocomplete="off"
                @focus="$emit('update:showGroupMenu', true)"
                @input="$emit('update:form', patch('groupName', ($event.target as HTMLInputElement).value, form)); $emit('update:showGroupMenu', true)"
              />
              <button class="combo-arrow" type="button" tabindex="-1" @click.stop="$emit('update:showGroupMenu', !showGroupMenu)">▾</button>
              <div v-if="showGroupMenu" class="combo-menu">
                <div
                  v-for="g in filteredGroups"
                  :key="g"
                  class="combo-item"
                  :class="{ active: form.groupName === g }"
                  @mousedown.prevent="$emit('pick-group', g)"
                >
                  {{ g }}
                </div>
                <div v-if="filteredGroups.length === 0" class="combo-empty">无匹配分组，将作为新分组保存</div>
              </div>
            </div>
          </div>
          <label class="form-row">
            <input
              type="checkbox"
              :checked="form.enabled"
              @change="$emit('update:form', patch('enabled', ($event.target as HTMLInputElement).checked, form))"
            />
            启用
          </label>
        </div>
        <div class="add-actions">
          <button class="btn" type="button" @click="$emit('close')">取消</button>
          <button class="btn btn-primary" type="button" @click="$emit('save')">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
