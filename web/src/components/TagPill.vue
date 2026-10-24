<script setup>
defineProps({
  name: { type: String, required: true },
  count: { type: Number, default: null },
  active: { type: Boolean, default: false }
})
</script>

<template>
  <RouterLink :to="{ path: '/search', query: { tag: name } }" class="tag" :class="{ 'tag--active': active }">
    {{ name }}
    <span v-if="count !== null" class="tag-count">{{ count }}</span>
  </RouterLink>
</template>
