<script setup>
import Icon from './Icon.vue'

defineProps({
  title: { type: String, default: '这里还是空的' },
  description: { type: String, default: '' },
  icon: { type: String, default: 'feather' }
})
</script>

<template>
  <div class="empty-state">
    <span class="empty-icon"><Icon :name="icon" :size="26" /></span>
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <slot />
  </div>
</template>
