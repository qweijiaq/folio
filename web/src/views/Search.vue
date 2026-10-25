<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import DocCard from '../components/DocCard.vue'
import DocCardSkeleton from '../components/DocCardSkeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import Pagination from '../components/Pagination.vue'
import { usePageMeta } from '../composables/usePageMeta'

const HISTORY_KEY = 'folio.search.history'
const route = useRoute()
const router = useRouter()

const query = ref(String(route.query.q || ''))
const tab = ref('all')
const page = ref(1)
const pageSize = 12
const items = ref([])
const total = ref(0)
const counts = reactive({ all: 0, article: 0, doc: 0 })
const relatedTags = ref([])
const popularTags = ref([])
const history = ref(readHistory())
const loading = ref(false)
const searched = ref(false)

const keyword = computed(() => String(route.query.q || '').trim())
const activeTag = computed(() => String(route.query.tag || '').trim())
const label = computed(() => activeTag.value || keyword.value)

usePageMeta(() => ({
  title: label.value ? `搜索：${label.value}` : '搜索',
  description: '在知微的文章与文档中搜索。',
  noindex: true,
  url: '/search'
}))

function readHistory() {
  try {
    const value = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]')
    return Array.isArray(value) ? value.slice(0, 8) : []
  } catch {
    return []
  }
}

function rememberSearch(value) {
  const text = value.trim()
  if (!text) return
  history.value = [text, ...history.value.filter((item) => item !== text)].slice(0, 8)
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
  } catch {
    /* ignore */
  }
}

async function run() {
  if (!label.value) {
    items.value = []
    total.value = 0
    searched.value = false
    return
  }
  loading.value = true
  searched.value = true
  try {
    const data = await api.get('/search', {
      q: keyword.value || undefined,
      tag: activeTag.value || undefined,
      kind: tab.value === 'all' ? undefined : tab.value,
      page: page.value,
      pageSize
    })
    items.value = data.items || []
    total.value = data.total || 0
    Object.assign(counts, { all: 0, article: 0, doc: 0 }, data.counts || {})
    relatedTags.value = data.tags || []
    if (keyword.value) {
      rememberSearch(keyword.value)
    }
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
}

async function loadPopularTags() {
  try {
    const data = await api.get('/tags')
    popularTags.value = (data.items || []).slice(0, 14)
  } catch {
    popularTags.value = []
  }
}

function submit() {
  const value = query.value.trim()
  if (!value) return
  if (keyword.value === value && !activeTag.value) {
    page.value = 1
    run()
  } else {
    router.push({ name: 'search', query: { q: value } })
  }
}

function setTab(value) {
  if (tab.value === value) return
  tab.value = value
  page.value = 1
  run()
}

function changePage(value) {
  page.value = value
  run()
  document.querySelector('.search-results')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function searchFor(value) {
  router.push({ name: 'search', query: { q: value } })
}

function searchTag(name) {
  router.push({ name: 'search', query: { tag: name } })
}

function clearSearch() {
  query.value = ''
  router.push({ name: 'search' })
}

function clearHistory() {
  history.value = []
  try {
    localStorage.removeItem(HISTORY_KEY)
  } catch {
    /* ignore */
  }
}

watch(
  () => [route.query.q, route.query.tag],
  () => {
    query.value = keyword.value
    tab.value = 'all'
    page.value = 1
    run()
  },
  { immediate: true }
)

onMounted(loadPopularTags)
</script>

<template>
  <div class="page search-page">
    <header class="search-hero container">
      <span class="eyebrow">检索</span>
      <h1>搜索</h1>
      <form class="search-form" @submit.prevent="submit">
        <Icon name="search" :size="20" />
        <input v-model="query" type="search" placeholder="输入标题、摘要、正文或标签…" aria-label="搜索" />
        <button class="btn btn-primary" type="submit">搜索</button>
      </form>
      <p v-if="searched" class="search-summary">
        <template v-if="activeTag">标签「{{ activeTag }}」相关的内容共 <strong>{{ counts.all }}</strong> 条</template>
        <template v-else>与「{{ keyword }}」相关的内容共 <strong>{{ counts.all }}</strong> 条</template>
        <span class="search-sort-hint">{{ keyword ? '按相关度排序' : '按更新时间排序' }}</span>
      </p>
    </header>

    <div class="container">
      <div v-if="searched" class="tabs">
        <button type="button" :class="{ active: tab === 'all' }" @click="setTab('all')">全部 <span>{{ counts.all }}</span></button>
        <button type="button" :class="{ active: tab === 'article' }" @click="setTab('article')">文章 <span>{{ counts.article }}</span></button>
        <button type="button" :class="{ active: tab === 'doc' }" @click="setTab('doc')">文档 <span>{{ counts.doc }}</span></button>
      </div>

      <div v-if="searched && relatedTags.length" class="search-related-tags">
        <span>相关标签</span>
        <button v-for="tag in relatedTags" :key="tag.name" type="button" class="tag" @click="searchTag(tag.name)">
          {{ tag.name }} <span class="tag-count">{{ tag.count }}</span>
        </button>
      </div>

      <section class="search-results">
        <div v-if="loading" class="doc-grid">
          <DocCardSkeleton v-for="i in 4" :key="i" />
        </div>
        <EmptyState
          v-else-if="searched && !total"
          icon="search"
          title="没有找到相关内容"
          :description="activeTag ? '这个标签下暂时没有内容。' : '试试更短的关键词，或者从下面的标签开始。'"
        >
          <button class="btn btn-ghost btn-sm" type="button" @click="clearSearch">重新搜索</button>
        </EmptyState>
        <div v-else-if="searched" class="doc-grid">
          <DocCard v-for="doc in items" :key="`${doc.kind}-${doc.id}`" :doc="doc" :highlight="keyword" />
        </div>
        <Pagination v-if="searched && total > pageSize" :page="page" :total="total" :page-size="pageSize" @change="changePage" />
      </section>

      <section v-if="!searched" class="search-discovery">
        <div v-if="history.length" class="search-discovery-block">
          <header>
            <span class="eyebrow">最近</span>
            <button class="text-link" type="button" @click="clearHistory">清空</button>
          </header>
          <div class="search-chip-row">
            <button v-for="item in history" :key="item" type="button" class="tag" @click="searchFor(item)">{{ item }}</button>
          </div>
        </div>

        <div class="search-discovery-block">
          <header>
            <span class="eyebrow">热门标签</span>
          </header>
          <div class="search-chip-row">
            <button v-for="tag in popularTags" :key="tag.name" type="button" class="tag" @click="searchTag(tag.name)">
              {{ tag.name }} <span class="tag-count">{{ tag.count }}</span>
            </button>
          </div>
          <p v-if="!popularTags.length" class="hint">还没有可以探索的标签。</p>
        </div>

        <div class="search-hint">
          <Icon name="search" :size="24" />
          <p>试试搜索「封面」「Go」「知识管理」，或者按 <kbd>/</kbd> 或 <kbd>⌘K</kbd> 随时唤起快速搜索。</p>
        </div>
      </section>
    </div>
  </div>
</template>
