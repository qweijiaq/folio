<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { site } from '../stores/site'
import { toast } from '../stores/toast'
import CoverImage from './CoverImage.vue'
import Icon from './Icon.vue'

const props = defineProps({
  doc: { type: Object, required: true },
  url: { type: String, default: '' }
})
const emit = defineEmits(['close'])
const canNativeShare = ref(typeof navigator !== 'undefined' && typeof navigator.share === 'function')

const shareURL = computed(() => props.url || window.location.href)
const shareTitle = computed(() => `${props.doc.title} · ${site.name}`)
const xShare = computed(() => `https://twitter.com/intent/tweet?text=${encodeURIComponent(shareTitle.value)}&url=${encodeURIComponent(shareURL.value)}`)
const weiboShare = computed(() => `https://service.weibo.com/share/share.php?title=${encodeURIComponent(shareTitle.value)}&url=${encodeURIComponent(shareURL.value)}`)

async function copyLink() {
  try {
    await navigator.clipboard.writeText(shareURL.value)
    toast('链接已复制')
  } catch {
    toast('复制失败，请手动选择链接', 'error')
  }
}

async function nativeShare() {
  if (!navigator.share) return
  try {
    await navigator.share({ title: shareTitle.value, url: shareURL.value })
  } catch {
    /* 用户取消 */
  }
}

function onKeydown(event) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  document.body.style.overflow = 'hidden'
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="share-card" appear>
      <div class="share-overlay" @mousedown.self="emit('close')">
        <section class="share-card" role="dialog" aria-modal="true" aria-label="分享卡片">
          <header class="share-card-head">
            <span class="eyebrow">分享</span>
            <button class="icon-btn" type="button" aria-label="关闭分享卡片" @click="emit('close')">
              <Icon name="close" :size="18" />
            </button>
          </header>

          <div class="share-card-preview">
            <CoverImage :doc="doc" />
          </div>
          <div class="share-card-copy">
            <strong>{{ doc.title }}</strong>
            <p>{{ doc.abstract }}</p>
            <span>{{ site.name }} · {{ site.owner }}</span>
          </div>

          <div class="share-card-actions">
            <button class="btn btn-primary btn-sm" type="button" @click="copyLink">
              <Icon name="link" :size="15" /> 复制链接
            </button>
            <button v-if="canNativeShare" class="btn btn-ghost btn-sm" type="button" @click="nativeShare">
              <Icon name="share" :size="15" /> 系统分享
            </button>
            <a class="btn btn-ghost btn-sm" :href="xShare" target="_blank" rel="noopener noreferrer">X</a>
            <a class="btn btn-ghost btn-sm" :href="weiboShare" target="_blank" rel="noopener noreferrer">微博</a>
          </div>

          <div class="share-card-url">{{ shareURL }}</div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
