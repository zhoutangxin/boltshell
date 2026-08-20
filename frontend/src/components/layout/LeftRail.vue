<!-- 最左侧 40px 导航栏：连接管理器入口 -->
<script setup lang="ts">
defineProps<{ mgrOpen: boolean }>()
defineEmits<{ (e: 'open-mgr'): void }>()
</script>

<template>
  <aside class="left-rail">
    <button
      class="btn-conn-mgr"
      :class="{ active: mgrOpen }"
      title="连接管理器"
      @click="$emit('open-mgr')"
    >
      📁
    </button>
  </aside>
</template>
