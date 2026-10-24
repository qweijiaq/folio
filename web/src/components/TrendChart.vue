<script setup>
import { computed } from 'vue'

const props = defineProps({
  labels: { type: Array, default: () => [] },
  values: { type: Array, default: () => [] }
})

const width = 600
const height = 220
const pad = { left: 42, right: 14, top: 16, bottom: 28 }

const max = computed(() => Math.max(1, ...props.values))
const plotWidth = width - pad.left - pad.right
const plotHeight = height - pad.top - pad.bottom

const points = computed(() =>
  props.values.map((value, index) => {
    const x = props.values.length <= 1 ? pad.left + plotWidth / 2 : pad.left + (plotWidth * index) / (props.values.length - 1)
    const y = pad.top + plotHeight - (value / max.value) * plotHeight
    return { x, y, value, label: props.labels[index] || '' }
  })
)

const linePath = computed(() => points.value.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(1)} ${point.y.toFixed(1)}`).join(' '))
const areaPath = computed(() => {
  if (!points.value.length) return ''
  const baseline = pad.top + plotHeight
  return `${linePath.value} L ${points.value[points.value.length - 1].x.toFixed(1)} ${baseline} L ${points.value[0].x.toFixed(1)} ${baseline} Z`
})

const yTicks = computed(() =>
  [0, 0.25, 0.5, 0.75, 1].map((fraction) => ({
    y: pad.top + plotHeight - fraction * plotHeight,
    value: Math.round(max.value * fraction)
  }))
)

const xTicks = computed(() => {
  if (!points.value.length) return []
  const count = Math.min(5, points.value.length)
  const indexes = new Set()
  for (let i = 0; i < count; i += 1) {
    indexes.add(Math.round((i * (points.value.length - 1)) / Math.max(1, count - 1)))
  }
  return [...indexes].map((index) => points.value[index])
})

const visibleDots = computed(() => {
  if (points.value.length <= 31) return points.value
  const step = Math.ceil(points.value.length / 12)
  return points.value.filter((_, index) => index % step === 0 || index === points.value.length - 1)
})

const hasData = computed(() => props.values.some((value) => value > 0))
</script>

<template>
  <div class="trend-chart">
    <svg :viewBox="`0 0 ${width} ${height}`" role="img" aria-label="数据趋势">
      <g class="trend-grid">
        <line v-for="tick in yTicks" :key="tick.y" :x1="pad.left" :x2="width - pad.right" :y1="tick.y" :y2="tick.y" />
      </g>
      <g class="trend-axis">
        <text v-for="tick in yTicks" :key="`y-${tick.y}`" :x="pad.left - 8" :y="tick.y + 4" text-anchor="end">
          {{ tick.value }}
        </text>
        <text v-for="point in xTicks" :key="`x-${point.label}`" :x="point.x" :y="height - 8" text-anchor="middle">
          {{ point.label.slice(5) }}
        </text>
      </g>
      <path v-if="areaPath" :d="areaPath" class="trend-area" />
      <path v-if="linePath" :d="linePath" class="trend-line" />
      <g v-if="hasData">
        <circle v-for="point in visibleDots" :key="point.label" :cx="point.x" :cy="point.y" r="3.2" class="trend-dot">
          <title>{{ point.label }}：{{ point.value }}</title>
        </circle>
      </g>
    </svg>
    <p v-if="!hasData" class="trend-empty">还没有数据，内容和访问产生后会在这里出现。</p>
  </div>
</template>
