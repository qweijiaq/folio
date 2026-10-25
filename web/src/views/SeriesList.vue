<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import { shortDate } from '../utils/format'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

usePageMeta(() => ({ title: '合集', description: '按系列阅读知微的文章合集。', url: '/series' }))

const series = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    const data = await api.get('/series')
    series.value = data.items || []
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page series-page">
    <header class="page-hero container">
      <span class="eyebrow">系列 · 合集</span>
      <h1>顺着一条线读下去</h1>
      <p class="page-lede">把同一主题的文章整理成合集，按顺序阅读，也可以随时从其中一篇开始。</p>
    </header>

    <div class="container">
      <p v-if="loading" class="muted loading-line">正在整理合集…</p>
      <EmptyState
        v-else-if="!series.length"
        icon="layers"
        title="还没有合集"
        description="写文章时可以把文章归入合集，合集会出现在这里。"
      >
        <RouterLink to="/articles" class="btn btn-ghost btn-sm">先读几篇文章</RouterLink>
      </EmptyState>
      <div v-else class="series-grid">
        <RouterLink v-for="item in series" :key="item.id" :to="`/series/${item.id}`" class="series-card">
          <header>
            <span class="series-author">@{{ item.author?.nickname || '作者' }}</span>
            <span class="series-count">{{ item.count }} 篇</span>
          </header>
          <h2>{{ item.title }}</h2>
          <p>{{ item.description || '这个合集还没有写介绍。' }}</p>
          <footer>
            <span>更新于 {{ shortDate(item.updatedAt) }}</span>
            <Icon name="arrowRight" :size="16" />
          </footer>
        </RouterLink>
      </div>
    </div>
  </div>
</template>
