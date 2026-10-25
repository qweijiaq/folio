<script setup>
import { ref, watch } from 'vue'
import { api } from '../api/client'
import Icon from './Icon.vue'

const props = defineProps({
  docId: { type: [Number, String], required: true }
})

const items = ref([])

async function load() {
  if (!props.docId) return
  try {
    const data = await api.get(`/docs/${props.docId}/backlinks`)
    items.value = data.items || []
  } catch {
    items.value = []
  }
}

watch(() => props.docId, load, { immediate: true })
</script>

<template>
  <section v-if="items.length" class="related-section backlinks-section">
    <header class="section-head section-head--tight">
      <div>
        <span class="eyebrow">引用</span>
        <h2>反向链接</h2>
      </div>
    </header>
    <div class="related-list">
      <RouterLink
        v-for="item in items"
        :key="`${item.kind}-${item.id}`"
        :to="`/${item.kind === 'doc' ? 'docs' : 'articles'}/${item.id}`"
        class="related-row"
      >
        <span class="related-kind">{{ item.kind === 'doc' ? '文档' : '文章' }}</span>
        <span class="related-main">
          <strong>{{ item.title }}</strong>
          <span>{{ item.abstract }}</span>
        </span>
        <Icon name="arrowRight" :size="16" />
      </RouterLink>
    </div>
  </section>
</template>
