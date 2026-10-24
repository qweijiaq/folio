<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import DocCard from '../components/DocCard.vue'
import DocCardSkeleton from '../components/DocCardSkeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

const query = ref(String(route.query.q || ''))
const tab = ref('all')
const articles = ref([])
const docs = ref([])
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

const articleTotal = computed(() => articles.value.length)
const docTotal = computed(() => docs.value.length)
const total = computed(() => articleTotal.value + docTotal.value)
const results = computed(() => {
  if (tab.value === 'article') return articles.value
  if (tab.value === 'doc') return docs.value
  return [...articles.value, ...docs.value]
})

async function run() {
  if (!label.value) {
    articles.value = []
    docs.value = []
    searched.value = false
    return
  }
  loading.value = true
  searched.value = true
  const filters = activeTag.value ? { tag: activeTag.value } : { q: keyword.value }
  try {
    const [articleData, docData] = await Promise.all([
      api.get('/docs', { ...filters, kind: 'article', pageSize: 20 }),
      api.get('/docs', { ...filters, kind: 'doc', pageSize: 20 })
    ])
    articles.value = articleData.items
    docs.value = docData.items
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
}

function submit() {
  const value = query.value.trim()
  if (!value) return
  if (keyword.value === value && !activeTag.value) run()
  else router.push({ name: 'search', query: { q: value } })
}

watch(
  () => [route.query.q, route.query.tag],
  () => {
    query.value = keyword.value
    tab.value = 'all'
    run()
  },
  { immediate: true }
)
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
        <template v-if="activeTag">标签「{{ activeTag }}」相关的内容共 <strong>{{ total }}</strong> 条</template>
        <template v-else>与「{{ keyword }}」相关的内容共 <strong>{{ total }}</strong> 条</template>
      </p>
    </header>

    <div class="container">
      <div v-if="searched" class="tabs">
        <button type="button" :class="{ active: tab === 'all' }" @click="tab = 'all'">全部 <span>{{ total }}</span></button>
        <button type="button" :class="{ active: tab === 'article' }" @click="tab = 'article'">文章 <span>{{ articleTotal }}</span></button>
        <button type="button" :class="{ active: tab === 'doc' }" @click="tab = 'doc'">文档 <span>{{ docTotal }}</span></button>
      </div>

      <div v-if="loading" class="doc-grid">
        <DocCardSkeleton v-for="i in 4" :key="i" />
      </div>
      <EmptyState
        v-else-if="searched && !total"
        icon="search"
        title="没有找到相关内容"
        :description="activeTag ? '这个标签下暂时没有内容。' : '换一个关键词，或者去文章与文档里随便逛逛。'"
      >
        <RouterLink to="/articles" class="btn btn-ghost btn-sm">浏览文章</RouterLink>
      </EmptyState>
      <div v-else-if="!searched" class="search-hint">
        <Icon name="search" :size="24" />
        <p>试试搜索「封面」「Go」「知识管理」，或者按 <kbd>/</kbd> 或 <kbd>⌘K</kbd> 随时唤起快速搜索。</p>
      </div>
      <div v-else class="doc-grid">
        <DocCard v-for="doc in results" :key="`${doc.kind}-${doc.id}`" :doc="doc" />
      </div>
    </div>
  </div>
</template>
