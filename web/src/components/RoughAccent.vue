<script setup>
import { getCurrentInstance, nextTick, onMounted, watch } from 'vue'
import { ui } from '../stores/ui'
import { drawAccent } from '../utils/sketch'

const props = defineProps({
  variant: { type: String, default: 'underline' },
  tone: { type: String, default: 'accent' }
})

const instance = getCurrentInstance()
const boxes = {
  underline: '0 0 200 18',
  squiggle: '0 0 200 24',
  circle: '0 0 200 80',
  arrow: '0 0 200 24'
}

function rootElement() {
  const tree = instance?.subTree
  return tree && tree.el && tree.el.nodeType === 1 ? tree.el : null
}

function render() {
  const wrapper = rootElement()
  if (!wrapper) return
  let svg = wrapper.querySelector('svg')
  if (!svg) {
    svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    wrapper.appendChild(svg)
  }
  svg.setAttribute('viewBox', boxes[props.variant] || boxes.underline)
  svg.setAttribute('preserveAspectRatio', props.variant === 'circle' ? 'xMidYMid meet' : 'none')
  drawAccent(svg, props.variant, props.tone)
}

onMounted(() => nextTick(render))
watch(
  () => [props.variant, props.tone, ui.theme],
  () => nextTick(render)
)
</script>

<template>
  <span class="rough-accent" :class="`rough-accent--${variant}`" aria-hidden="true"></span>
</template>
