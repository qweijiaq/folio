<script setup>
import { computed } from 'vue'
import { splitMentions } from '../utils/mentions'

const props = defineProps({
  text: { type: String, default: '' }
})

const segments = computed(() => splitMentions(props.text))

</script>

<template>
  <span>
    <template v-for="(segment, index) in segments" :key="index">
      <RouterLink v-if="segment.type === 'mention'" :to="`/u/${segment.username}`" class="mention-link">
        {{ segment.text }}
      </RouterLink>
      <template v-else>{{ segment.text }}</template>
    </template>
  </span>
</template>
