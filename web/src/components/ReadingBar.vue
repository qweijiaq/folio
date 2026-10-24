<script setup>
import { computed } from 'vue'

const props = defineProps({
  progress: { type: Number, default: 0 }
})

const scale = computed(() => Math.min(1, Math.max(0, props.progress / 100)))
</script>

<template>
  <div class="reading-bar" aria-hidden="true">
    <span :style="{ transform: `scaleX(${scale})` }"></span>
  </div>
</template>
