<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

const SIZE_KEY = 'folio.reading.fontSize'
const WIDTH_KEY = 'folio.reading.width'
const FOCUS_KEY = 'folio.reading.focus'
const sizes = [15, 17, 19, 21]
const widths = [
  { value: '640px', label: '窄' },
  { value: '760px', label: '标准' },
  { value: '900px', label: '宽' }
]

function readNumber(key, fallback) {
  try {
    const value = Number(localStorage.getItem(key))
    return Number.isFinite(value) && value > 0 ? value : fallback
  } catch {
    return fallback
  }
}

function readString(key, fallback) {
  try {
    return localStorage.getItem(key) || fallback
  } catch {
    return fallback
  }
}

const fontSize = ref(readNumber(SIZE_KEY, 17))
const savedWidth = readString(WIDTH_KEY, '760px')
const width = ref(widths.some((item) => item.value === savedWidth) ? savedWidth : '760px')
const focus = ref(readString(FOCUS_KEY, '0') === '1')

function apply() {
  document.documentElement.style.setProperty('--reading-font-size', `${fontSize.value}px`)
  document.documentElement.style.setProperty('--reading-width', width.value)
  document.body.classList.toggle('reading-focus', focus.value)
  try {
    localStorage.setItem(SIZE_KEY, String(fontSize.value))
    localStorage.setItem(WIDTH_KEY, width.value)
    localStorage.setItem(FOCUS_KEY, focus.value ? '1' : '0')
  } catch {
    /* 隐私模式可能不可用 */
  }
}

function changeSize(delta) {
  const index = sizes.indexOf(fontSize.value)
  const next = Math.min(sizes.length - 1, Math.max(0, index + delta))
  fontSize.value = sizes[next]
  apply()
}

function cycleWidth() {
  const index = widths.findIndex((item) => item.value === width.value)
  width.value = widths[(index + 1) % widths.length].value
  apply()
}

function toggleFocus() {
  focus.value = !focus.value
  apply()
}

function printPage() {
  window.print()
}

function onKeydown(event) {
  if (event.key === 'Escape' && focus.value) {
    focus.value = false
    apply()
  }
}

onMounted(() => {
  apply()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.body.classList.remove('reading-focus')
  document.documentElement.style.removeProperty('--reading-font-size')
  document.documentElement.style.removeProperty('--reading-width')
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="reading-tools" role="toolbar" aria-label="阅读设置">
    <span class="reading-tools-label">阅读</span>
    <button type="button" :disabled="fontSize <= sizes[0]" aria-label="减小字号" title="减小字号" @click="changeSize(-1)">A−</button>
    <span class="reading-tools-size" aria-live="polite">{{ fontSize }}</span>
    <button type="button" :disabled="fontSize >= sizes[sizes.length - 1]" aria-label="增大字号" title="增大字号" @click="changeSize(1)">A+</button>
    <button type="button" :title="`阅读宽度：${widths.find((item) => item.value === width)?.label}`" @click="cycleWidth">
      行宽 · {{ widths.find((item) => item.value === width)?.label }}
    </button>
    <button type="button" :class="{ active: focus }" :aria-pressed="focus" @click="toggleFocus">
      {{ focus ? '退出专注' : '专注' }}
    </button>
    <button type="button" @click="printPage">打印</button>
  </div>
</template>
