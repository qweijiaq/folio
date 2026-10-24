<script setup>
import { getCurrentInstance, nextTick, onMounted, watch } from 'vue'
import { ui } from '../stores/ui'
import { drawBrandMark } from '../utils/sketch'

const instance = getCurrentInstance()

function render() {
  const tree = instance?.subTree
  const wrapper = tree && tree.el && tree.el.nodeType === 1 ? tree.el : null
  if (!wrapper) return
  let svg = wrapper.querySelector('svg')
  if (!svg) {
    svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    svg.setAttribute('viewBox', '0 0 40 40')
    wrapper.appendChild(svg)
  }
  drawBrandMark(svg)
}

onMounted(() => nextTick(render))
watch(
  () => ui.theme,
  () => nextTick(render)
)
</script>

<template>
  <span class="brand-mark-svg" aria-hidden="true"></span>
</template>
