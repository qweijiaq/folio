<script setup>
import { computed } from 'vue'

const props = defineProps({
  text: { type: String, default: '' },
  query: { type: String, default: '' }
})

const parts = computed(() => {
  const text = props.text || ''
  const query = props.query.trim()
  if (!query) return [{ text, match: false }]
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const pieces = text.split(new RegExp(`(${escaped})`, 'gi'))
  return pieces.filter(Boolean).map((piece) => ({
    text: piece,
    match: piece.toLowerCase() === query.toLowerCase()
  }))
})
</script>

<template>
  <span class="highlight-text">
    <template v-for="(part, index) in parts" :key="index">
      <mark v-if="part.match">{{ part.text }}</mark>
      <template v-else>{{ part.text }}</template>
    </template>
  </span>
</template>
