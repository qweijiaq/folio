<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ui } from '../stores/ui'
import { drawAvatar } from '../utils/sketch'

const props = defineProps({
  username: { type: String, default: '' },
  nickname: { type: String, default: '' },
  size: { type: Number, default: 40 }
})

const canvas = ref(null)
const label = computed(() => props.nickname || props.username || '读者')
let observer = null

function render() {
  drawAvatar(canvas.value, {
    seed: props.username || label.value,
    label: label.value
  })
}

onMounted(() => {
  render()
  if (typeof ResizeObserver !== 'undefined' && canvas.value) {
    observer = new ResizeObserver(() => render())
    observer.observe(canvas.value)
  }
})

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
})

watch(
  () => [props.username, props.nickname, ui.theme],
  () => nextTick(render)
)
</script>

<template>
  <canvas
    ref="canvas"
    class="avatar"
    width="240"
    height="240"
    role="img"
    :aria-label="`${label}的手绘头像`"
    :style="{ width: size + 'px', height: size + 'px' }"
  />
</template>
