<script setup>
import { computed, watch } from 'vue'
import { renderMarkdown } from '../utils/markdown'
import { toast } from '../stores/toast'

const props = defineProps({
  source: { type: String, default: '' }
})
const emit = defineEmits(['toc'])

const rendered = computed(() => renderMarkdown(props.source))
watch(
  () => rendered.value.toc,
  (toc) => emit('toc', toc),
  { immediate: true }
)

function onClick(event) {
  const toggle = event.target.closest('.code-toggle')
  if (toggle) {
    const block = toggle.closest('.code-block')
    if (!block) return
    const collapsed = block.classList.toggle('is-collapsed')
    toggle.textContent = collapsed ? '展开' : '收起'
    toggle.setAttribute('aria-expanded', String(!collapsed))
    return
  }
  const button = event.target.closest('.code-copy')
  if (!button) return
  const code = button.closest('.code-block')?.querySelector('code')?.innerText || ''
  if (!navigator.clipboard) {
    toast('当前浏览器不支持复制', 'error')
    return
  }
  navigator.clipboard
    .writeText(code)
    .then(() => {
      button.textContent = '已复制'
      window.setTimeout(() => {
        button.textContent = '复制'
      }, 1500)
    })
    .catch(() => toast('复制失败', 'error'))
}
</script>

<template>
  <div class="prose" v-html="rendered.html" @click="onClick"></div>
</template>
