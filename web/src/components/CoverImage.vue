<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ui } from '../stores/ui'
import { drawCover } from '../utils/sketch'

const props = defineProps({
  doc: { type: Object, default: null },
  seed: { type: [String, Number], default: '' },
  title: { type: String, default: '' },
  abstract: { type: String, default: '' },
  tags: { type: Array, default: () => [] },
  category: { type: String, default: '' },
  kind: { type: String, default: 'article' },
  alt: { type: String, default: '' }
})

const canvas = ref(null)
let observer = null

const seed = computed(() => {
  if (props.doc?.id) return String(props.doc.id)
  if (props.seed !== '' && props.seed !== null && props.seed !== undefined) return String(props.seed)
  return props.doc?.title || props.title || 'folio'
})

const kind = computed(() => props.doc?.kind || props.kind || 'article')

const label = computed(() => props.doc?.title || props.title || '')

const abstractText = computed(() => props.doc?.abstract || props.abstract || '')

const coverTags = computed(() => {
  const value = props.doc?.tags ?? props.tags
  return Array.isArray(value) ? value : []
})

const categoryName = computed(() => props.doc?.category?.name || props.category || '')

const alt = computed(() => {
  if (props.alt) return props.alt
  const title = props.doc?.title || props.title
  return title ? `《${title}》的手绘封面` : '手绘封面'
})

function render() {
  drawCover(canvas.value, {
    seed: seed.value,
    kind: kind.value,
    label: label.value,
    abstract: abstractText.value,
    tags: coverTags.value,
    category: categoryName.value
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
  () => [seed.value, kind.value, label.value, abstractText.value, coverTags.value.join('|'), categoryName.value, ui.theme],
  () => nextTick(render)
)
</script>

<template>
  <canvas ref="canvas" class="cover-img" width="1200" height="480" role="img" :aria-label="alt" />
</template>
