<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toastError } from '../stores/toast'
import DocCard from '../components/DocCard.vue'
import DocCardSkeleton from '../components/DocCardSkeleton.vue'
import Pagination from '../components/Pagination.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

const docs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 9
const loading = ref(true)
const tags = ref([])
const categories = ref([])

const activeTag = computed(() => (route.query.tag ? String(route.query.tag) : ''))
const activeCategory = computed(() => (route.query.category ? String(route.query.category) : ''))
const activeCategoryName = computed(() => categories.value.find((c) => String(c.id) === activeCategory.value)?.name || '')
const sort = computed(() => (route.query.sort ? String(route.query.sort) : 'latest'))
const keyword = computed(() => (route.query.q ? String(route.query.q) : ''))
const author = computed(() => (route.query.author ? String(route.query.author) : ''))
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

usePageMeta(() => ({
  title: '文章',
  description: '较长的思考、读书笔记，以及一些还没想清楚的问题。',
  image: '/api/art/cover.png?seed=articles&kind=site',
  url: '/articles'
}))

async function load() {
  loading.value = true
  try {
    const data = await api.get('/docs', {
      kind: 'article',
      page: page.value,
      pageSize,
      tag: activeTag.value || undefined,
      categoryID: activeCategory.value || undefined,
      sort: sort.value,
      q: keyword.value || undefined,
      author: author.value || undefined
    })
    docs.value = data.items
    total.value = data.total
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
}

async function loadTags() {
  try {
    const data = await api.get('/tags', { kind: 'article' })
    tags.value = data.items.slice(0, 14)
  } catch {
    /* ignore */
  }
}

async function loadCategories() {
  try {
    const data = await api.get('/categories')
    categories.value = data.items
  } catch {
    /* ignore */
  }
}

function setTag(name) {
  const query = { ...route.query }
  if (!name) delete query.tag
  else query.tag = name
  router.push({ name: 'articles', query })
}

function setCategory(event) {
  const value = String(event.target.value || '')
  const query = { ...route.query }
  if (!value) delete query.category
  else query.category = value
  router.push({ name: 'articles', query })
}

function setSort(value) {
  const query = { ...route.query }
  if (value === 'latest') delete query.sort
  else query.sort = value
  router.push({ name: 'articles', query })
}

function changePage(p) {
  page.value = p
  load()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

watch(
  () => route.query,
  () => {
    page.value = 1
    load()
  }
)

onMounted(() => {
  load()
  loadTags()
  loadCategories()
})
</script>

<template>
  <div class="page list-page">
    <header class="page-hero container">
      <span class="eyebrow">写作</span>
      <div class="page-hero-row">
        <div>
          <h1>文章</h1>
          <p class="page-lede">较长的思考、读书笔记，以及一些还没想清楚的问题。</p>
        </div>
        <span class="page-count">{{ total }} 篇</span>
      </div>
    </header>

    <div class="container">
      <div class="filter-bar">
        <div class="filter-tags">
          <button class="tag" :class="{ 'tag--active': !activeTag }" type="button" @click="setTag('')">全部</button>
          <button
            v-for="tag in tags"
            :key="tag.name"
            class="tag"
            :class="{ 'tag--active': activeTag === tag.name }"
            type="button"
            :title="`全部文章中使用「${tag.name}」的数量`"
            @click="setTag(tag.name)"
          >
            {{ tag.name }}
            <span class="tag-count">{{ tag.count }}</span>
          </button>
        </div>
        <div class="filter-right">
          <select class="category-select" :value="activeCategory" aria-label="按分类筛选" @change="setCategory">
            <option value="">全部分类</option>
            <option v-for="category in categories" :key="category.id" :value="String(category.id)">
              {{ category.name }} · {{ category.count }}
            </option>
          </select>
          <div class="sort-group">
            <button type="button" :class="{ active: sort === 'latest' }" @click="setSort('latest')">最新</button>
            <button type="button" :class="{ active: sort === 'popular' }" @click="setSort('popular')">最多阅读</button>
            <button type="button" :class="{ active: sort === 'likes' }" @click="setSort('likes')">最多点赞</button>
          </div>
        </div>
      </div>

      <div v-if="activeCategory" class="filter-note">
        <Icon name="layers" :size="15" />
        {{ activeCategoryName || '当前分类' }}分类，共 {{ total }} 篇
        <RouterLink :to="{ name: 'articles' }" class="filter-clear">查看全部</RouterLink>
      </div>
      <div v-if="keyword" class="filter-note">
        <Icon name="search" :size="15" />
        包含「{{ keyword }}」的文章，共 {{ total }} 篇
      </div>
      <div v-if="author" class="filter-note">
        <Icon name="user" :size="15" />
        @{{ author }} 的文章，共 {{ total }} 篇
        <RouterLink :to="{ name: 'articles' }" class="filter-clear">查看全部</RouterLink>
      </div>

      <div v-if="loading" class="doc-grid">
        <DocCardSkeleton v-for="i in 6" :key="i" />
      </div>
      <EmptyState
        v-else-if="!docs.length"
        icon="edit"
        title="这里还没有文章"
        :description="activeTag ? '这个标签下暂时没有内容，换一个标签看看。' : staff ? '写下第一篇，让书架开始生长。' : '文章会出现在这里。'"
      >
        <RouterLink v-if="staff" to="/write" class="btn btn-primary btn-sm">去写作</RouterLink>
      </EmptyState>
      <div v-else class="doc-grid">
        <DocCard v-for="doc in docs" :key="doc.id" :doc="doc" />
      </div>

      <Pagination v-if="!loading" :page="page" :total="total" :page-size="pageSize" @change="changePage" />
    </div>
  </div>
</template>
