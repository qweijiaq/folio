<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import { formatNumber, shortDate } from '../utils/format'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import TagPill from '../components/TagPill.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()
const series = ref(null)
const items = ref([])
const loading = ref(true)
const notFound = ref(false)

usePageMeta(() => ({
  title: series.value?.title || '合集',
  description: series.value?.description || '知微文章合集',
  url: series.value ? `/series/${series.value.id}` : '/series',
  type: 'website'
}))

async function load() {
  loading.value = true
  notFound.value = false
  try {
    const data = await api.get(`/series/${route.params.id}`)
    series.value = data.series
    items.value = data.items || []
  } catch (err) {
    if (err.status === 404) notFound.value = true
    else toastError(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="page series-detail-page">
    <div v-if="loading" class="container page-loading"><p class="muted">正在读取合集…</p></div>
    <div v-else-if="notFound" class="container page-loading">
      <EmptyState icon="layers" title="合集不存在" description="它可能被删除或还没有公开。">
        <RouterLink to="/series" class="btn btn-ghost btn-sm">返回合集</RouterLink>
      </EmptyState>
    </div>
    <template v-else-if="series">
      <header class="series-hero container">
        <RouterLink to="/series" class="eyebrow"><Icon name="arrowLeft" :size="13" /> 全部合集</RouterLink>
        <h1>{{ series.title }}</h1>
        <p class="series-lede">{{ series.description || '这个合集还没有写介绍。' }}</p>
        <div class="series-meta">
          <RouterLink :to="`/u/${series.author?.username}`" class="byline-author">
            <span>{{ series.author?.nickname }}</span>
          </RouterLink>
          <span>·</span>
          <span>{{ series.count }} 篇文章</span>
          <span>·</span>
          <span>更新于 {{ shortDate(series.updatedAt) }}</span>
        </div>
      </header>

      <div class="container series-article-list">
        <EmptyState v-if="!items.length" icon="edit" title="合集还是空的" description="把文章归入这个合集后，会按合集内排序显示。" />
        <RouterLink v-for="(item, index) in items" :key="item.id" :to="`/articles/${item.id}`" class="series-article">
          <span class="series-index">{{ String(index + 1).padStart(2, '0') }}</span>
          <div class="series-article-main">
            <h2>{{ item.title }}</h2>
            <p>{{ item.abstract }}</p>
            <div class="series-article-meta">
              <span>{{ shortDate(item.publishedAt || item.createdAt) }}</span>
              <span>{{ formatNumber(item.viewCount) }} 阅读</span>
              <TagPill v-for="tag in item.tags?.slice(0, 2) || []" :key="tag" :name="tag" />
            </div>
          </div>
          <Icon name="arrowRight" :size="18" />
        </RouterLink>
      </div>
    </template>
  </div>
</template>
