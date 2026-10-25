<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderMarkdown } from '../utils/markdown'
import { toast } from '../stores/toast'
import Icon from './Icon.vue'
import { useDocLinks } from '../composables/useDocLinks'

const props = defineProps({
  source: { type: String, default: '' }
})
const emit = defineEmits(['toc'])

const { linkMap, loadDocLinks } = useDocLinks()
const lightbox = ref(null)
let previousOverflow = ''
let lastImage = null

const rendered = computed(() => renderMarkdown(props.source, linkMap.value))
watch(
  () => rendered.value.toc,
  (toc) => emit('toc', toc),
  { immediate: true }
)

function openLightbox(image) {
  if (!image) return
  lastImage = image
  lightbox.value = { src: image.currentSrc || image.src, alt: image.alt || '' }
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  window.addEventListener('keydown', onLightboxKeydown)
}

function closeLightbox() {
  if (!lightbox.value) return
  lightbox.value = null
  document.body.style.overflow = previousOverflow
  window.removeEventListener('keydown', onLightboxKeydown)
  lastImage?.focus?.()
}

function onLightboxKeydown(event) {
  if (event.key === 'Escape') closeLightbox()
}

function onKeydown(event) {
  if ((event.key === 'Enter' || event.key === ' ') && event.target.tagName === 'IMG') {
    event.preventDefault()
    openLightbox(event.target)
  }
}

onMounted(loadDocLinks)

onBeforeUnmount(() => {
  if (lightbox.value) {
    document.body.style.overflow = previousOverflow
    window.removeEventListener('keydown', onLightboxKeydown)
  }
})

function onClick(event) {
  const anchor = event.target.closest('.heading-anchor')
  if (anchor) {
    event.preventDefault()
    const id = (anchor.getAttribute('href') || '').replace(/^#/, '')
    if (!id) return
    const url = `${window.location.origin}${window.location.pathname}${window.location.search}#${id}`
    try {
      history.replaceState(null, '', `#${id}`)
    } catch {
      /* ignore */
    }
    if (!navigator.clipboard) {
      toast('当前浏览器不支持复制', 'error')
      return
    }
    navigator.clipboard
      .writeText(url)
      .then(() => toast('本节链接已复制'))
      .catch(() => toast('复制失败', 'error'))
    return
  }
  const image = event.target.closest('.prose img')
  if (image) {
    openLightbox(image)
    return
  }
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
  <div class="prose" v-html="rendered.html" @click="onClick" @keydown="onKeydown"></div>
</template>
