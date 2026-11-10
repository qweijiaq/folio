<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

usePageMeta(() => ({
  title: '时间线',
  description: '按时间回看知微已经发布的文章和文档。',
  url: '/timeline',
  jsonLd: {
    '@context': 'https://schema.org',
    '@type': 'CollectionPage',
    name: '知微 · 时间线',
    description: '按时间回看知微已经发布的文章和文档。',
    url: window.location.origin + '/timeline'
  }
}))

const PAGE_SIZE = 30
const kind = ref('all')
const items = ref([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const loadingMore = ref(false)

const kindLabels = { article: '文章', doc: '文档' }
const filterOptions = [
  { value: 'all', label: '全部' },
  { value: 'article', label: '文章' },
  { value: 'doc', label: '文档' }
]

const groups = computed(() => {
  const years = []
  const yearMap = new Map()
  for (const item of items.value) {
    const date = new Date(item.publishedAt || item.createdAt)
    if (Number.isNaN(date.getTime())) continue
    const year = date.getFullYear()
    const month = date.getMonth() + 1
    let yearGroup = yearMap.get(year)
    if (!yearGroup) {
      yearGroup = { year, months: [], monthMap: new Map() }
      yearMap.set(year, yearGroup)
      years.push(yearGroup)
    }
    let monthGroup = yearGroup.monthMap.get(month)
    if (!monthGroup) {
      monthGroup = { month, items: [] }
      yearGroup.monthMap.set(month, monthGroup)
      yearGroup.months.push(monthGroup)
    }
    monthGroup.items.push({
      ...item,
      date,
      dateTime: date.toISOString(),
      day: String(date.getDate()).padStart(2, '0'),
      month: String(month).padStart(2, '0')
    })
  }
  return years
})

const hasMore = computed(() => items.value.length < total.value)

function docLink(item) {
  return `/${item.kind === 'doc' ? 'docs' : 'articles'}/${item.id}`
}

async function load(reset = false) {
  if (reset) {
    page.value = 1
    items.value = []
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const data = await api.get('/timeline', {
      kind: kind.value === 'all' ? '' : kind.value,
      page: page.value,
      pageSize: PAGE_SIZE
    })
    items.value = reset ? data.items || [] : [...items.value, ...(data.items || [])]
    total.value = data.total || 0
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function changeKind(value) {
  if (kind.value === value) return
  kind.value = value
  load(true)
}

function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  page.value += 1
  load(false)
}

onMounted(() => load(true))
</script>

<template>
  <div class="page timeline-page">
    <header class="page-hero container">
      <span class="eyebrow">时间线 · 归档</span>
      <h1>文章和文档，按时间排在一起</h1>
      <p class="page-lede">这里按发布时间倒序收拢所有公开内容。可以切换文章或文档，也可以顺着年份慢慢往回翻。</p>
    </header>

    <div class="container timeline-shell">
      <div class="timeline-toolbar">
        <div class="segmented timeline-filter" role="group" aria-label="筛选时间线内容">
          <button
            v-for="option in filterOptions"
            :key="option.value"
            type="button"
            :class="{ active: kind === option.value }"
            @click="changeKind(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
        <span class="timeline-total">{{ total }} 条公开内容</span>
      </div>

      <p v-if="loading" class="muted loading-line">正在整理时间线…</p>
      <EmptyState
        v-else-if="!items.length"
        icon="clock"
        title="时间线还是空的"
        description="发布第一篇文章或文档后，它会按时间出现在这里。"
      >
        <RouterLink to="/articles" class="btn btn-ghost btn-sm">先看看文章</RouterLink>
      </EmptyState>

      <div v-else class="timeline-list">
        <section v-for="yearGroup in groups" :key="yearGroup.year" class="timeline-year-block">
          <header class="timeline-year">
            <h2>{{ yearGroup.year }}</h2>
            <span>{{ yearGroup.months.reduce((sum, month) => sum + month.items.length, 0) }} 条</span>
          </header>

          <div v-for="monthGroup in yearGroup.months" :key="`${yearGroup.year}-${monthGroup.month}`" class="timeline-month-block">
            <h3 class="timeline-month">{{ String(monthGroup.month).padStart(2, '0') }} 月</h3>

            <article v-for="item in monthGroup.items" :key="item.id" class="timeline-entry">
              <time class="timeline-date" :datetime="item.dateTime">
                <strong>{{ item.day }}</strong>
              </time>
              <span class="timeline-node" aria-hidden="true" />
              <div class="timeline-body">
                <div class="timeline-meta">
                  <span class="timeline-kind" :class="`timeline-kind--${item.kind}`">{{ kindLabels[item.kind] }}</span>
                  <span v-if="item.author?.nickname">{{ item.author.nickname }}</span>
                  <span v-if="item.viewCount">{{ item.viewCount }} 阅读</span>
                </div>
                <h4><RouterLink :to="docLink(item)">{{ item.title }}</RouterLink></h4>
                <p v-if="item.abstract">{{ item.abstract }}</p>
                <div v-if="item.tags?.length" class="timeline-tags">
                  <span v-for="tag in item.tags.slice(0, 4)" :key="tag">#{{ tag }}</span>
                </div>
              </div>
            </article>
          </div>
        </section>
      </div>

      <div v-if="hasMore" class="timeline-more">
        <button class="btn btn-ghost" type="button" :disabled="loadingMore" @click="loadMore">
          {{ loadingMore ? '加载中…' : '加载更多' }}
          <Icon v-if="!loadingMore" name="chevronDown" :size="15" />
        </button>
      </div>
    </div>
  </div>
</template>
