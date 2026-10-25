<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { buildTree, flattenTree } from '../utils/tree'
import { formatDate, formatNumber, readingTime } from '../utils/format'
import DocTree from '../components/DocTree.vue'
import CoverImage from '../components/CoverImage.vue'
import UserAvatar from '../components/UserAvatar.vue'
import MarkdownView from '../components/MarkdownView.vue'
import CommentSection from '../components/CommentSection.vue'
import Backlinks from '../components/Backlinks.vue'
import ReadingTools from '../components/ReadingTools.vue'
import ArticleSkeleton from '../components/ArticleSkeleton.vue'
import TagPill from '../components/TagPill.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import ReadingBar from '../components/ReadingBar.vue'
import TocDrawer from '../components/TocDrawer.vue'
import ShareCard from '../components/ShareCard.vue'
import DocActions from '../components/DocActions.vue'
import { useReadingProgress } from '../composables/useReadingProgress'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

const doc = ref(null)
const treeItems = ref([])
const tree = ref([])
const flat = ref([])
const toc = ref([])
const liked = ref(false)
const favorited = ref(false)
const loading = ref(true)
const notFound = ref(false)
const privateBlocked = ref(false)
const accessInfo = ref(null)
const accessNote = ref('')
const requestingAccess = ref(false)
const cancellingAccess = ref(false)
const sideOpen = ref(false)
const shareOpen = ref(false)
const { progress, activeId, refresh: refreshReading } = useReadingProgress()

const canEdit = computed(() => {
  if (!doc.value || !auth.user) return false
  return doc.value.author.id === auth.user.id || auth.user.role === 'admin'
})

const breadcrumbs = computed(() => {
  if (!doc.value) return []
  const byId = new Map(treeItems.value.map((item) => [item.id, item]))
  const chain = []
  let current = { id: doc.value.id, title: doc.value.title, parentID: doc.value.parentID }
  for (let i = 0; i < 20 && current; i += 1) {
    chain.unshift({ id: current.id, title: current.title })
    const node = byId.get(current.id)
    const parentID = node ? node.parentID : current.parentID
    if (!parentID) break
    const parent = byId.get(parentID)
    if (!parent) break
    current = parent
  }
  return chain
})

const currentIndex = computed(() => flat.value.findIndex((item) => String(item.id) === String(doc.value?.id)))
const prevDoc = computed(() => (currentIndex.value > 0 ? flat.value[currentIndex.value - 1] : null))
const nextDoc = computed(() =>
  currentIndex.value >= 0 && currentIndex.value < flat.value.length - 1 ? flat.value[currentIndex.value + 1] : null
)

const visibleToc = computed(() => toc.value.filter((item) => item.level <= 3))

const docPrivate = computed(() => {
  if (!doc.value) return false
  const byId = new Map(treeItems.value.map((item) => [item.id, item]))
  let current = { id: doc.value.id, parentID: doc.value.parentID, private: Boolean(doc.value.private) }
  for (let i = 0; i < 20 && current; i += 1) {
    if (current.private) return true
    if (!current.parentID) break
    current = byId.get(current.parentID)
  }
  return false
})

usePageMeta(() => {
  const current = doc.value
  if (!current) return { title: '文档', description: '知微知识库', image: '/api/art/cover.png?seed=docs&kind=site' }
  const image = `/api/art/cover.png?seed=${current.id}&kind=doc`
  return {
    title: current.title,
    description: current.abstract || '',
    image,
    type: 'article',
    url: `/docs/${current.id}`,
    published: current.publishedAt || current.createdAt,
    modified: current.updatedAt,
    tags: current.tags || [],
    jsonLd: {
      '@context': 'https://schema.org',
      '@type': 'TechArticle',
      headline: current.title,
      description: current.abstract || '',
      datePublished: current.publishedAt || current.createdAt,
      dateModified: current.updatedAt,
      image: new URL(image, window.location.origin).href,
      mainEntityOfPage: `${window.location.origin}/docs/${current.id}`,
      author: {
        '@type': 'Person',
        name: current.author?.nickname,
        url: `${window.location.origin}/u/${current.author?.username}`
      },
      ...(current.tags?.length ? { keywords: current.tags.join(',') } : {})
    }
  }
})

async function load() {
  loading.value = true
  notFound.value = false
  privateBlocked.value = false
  accessInfo.value = null
  try {
    const [docData, treeData] = await Promise.all([
      api.get(`/docs/${route.params.id}`),
      api.get('/doc-tree')
    ])
    if (docData.doc.kind !== 'doc') {
      router.replace(`/articles/${docData.doc.id}`)
      return
    }
    doc.value = docData.doc
    liked.value = docData.liked
    favorited.value = docData.favorited
    treeItems.value = treeData.items
    tree.value = buildTree(treeData.items)
    flat.value = flattenTree(tree.value)
  } catch (err) {
    if (err.status === 403) {
      privateBlocked.value = true
      await loadAccessInfo()
    } else if (err.status === 404) {
      notFound.value = true
    } else {
      toastError(err)
    }
  } finally {
    loading.value = false
    nextTick(refreshReading)
  }
}

async function loadAccessInfo() {
  try {
    accessInfo.value = await api.get(`/docs/${route.params.id}/access`)
  } catch (err) {
    accessInfo.value = { private: true, canView: false, status: 'none' }
    toastError(err)
  }
}

async function requestAccess() {
  if (!requireLogin()) return
  requestingAccess.value = true
  try {
    const data = await api.post(`/docs/${route.params.id}/access-requests`, { note: accessNote.value.trim() })
    accessInfo.value = { ...(accessInfo.value || {}), status: data.status || 'pending', requestID: data.id, canView: false }
    accessNote.value = ''
    toast('申请已提交，等待作者处理')
  } catch (err) {
    toastError(err)
  } finally {
    requestingAccess.value = false
  }
}

async function cancelAccess() {
  const requestID = accessInfo.value?.requestID
  if (!requestID || cancellingAccess.value) return
  if (!window.confirm('撤回这条阅读申请？')) return
  cancellingAccess.value = true
  try {
    await api.del(`/access-requests/${requestID}`)
    accessInfo.value = { ...(accessInfo.value || {}), status: 'none', requestID: 0 }
    toast('已撤回阅读申请')
  } catch (err) {
    toastError(err)
  } finally {
    cancellingAccess.value = false
  }
}

function requireLogin() {
  if (auth.user) return true
  router.push({ path: '/login', query: { redirect: route.fullPath } })
  return false
}

async function toggleLike() {
  if (!requireLogin()) return
  try {
    const data = await api.post(`/docs/${doc.value.id}/like`)
    liked.value = data.liked
    doc.value.likeCount = data.likeCount
  } catch (err) {
    toastError(err)
  }
}

async function toggleFavorite() {
  if (!requireLogin()) return
  try {
    const data = await api.post(`/docs/${doc.value.id}/favorite`)
    favorited.value = data.favorited
    doc.value.favoriteCount = data.favoriteCount
    toast(data.favorited ? '已收藏' : '已取消收藏')
  } catch (err) {
    toastError(err)
  }
}

const shareUrl = computed(() => (doc.value ? `${window.location.origin}/docs/${doc.value.id}` : ''))

function share() {
  shareOpen.value = true
}

function goDoc(id) {
  sideOpen.value = false
  router.push(`/docs/${id}`)
}

onMounted(load)
watch(
  () => route.params.id,
  () => {
    load()
    window.scrollTo({ top: 0 })
  }
)
</script>

<template>
  <div class="page docs-page">
    <ReadingBar v-if="doc && !loading" :progress="progress" />
    <TocDrawer v-if="doc && visibleToc.length" :items="visibleToc" :active-id="activeId" title="本页目录" />
    <ArticleSkeleton v-if="loading" />

    <div v-else-if="notFound" class="container page-loading">
      <EmptyState icon="info" title="这篇文档不存在" description="它可能被删除，或者还没有公开。">
        <RouterLink to="/docs" class="btn btn-ghost btn-sm">返回知识库</RouterLink>
      </EmptyState>
    </div>

    <div v-else-if="privateBlocked" class="container page-loading private-access-page">
      <section class="private-access-card">
        <span class="private-access-icon"><Icon name="lock" :size="24" /></span>
        <p class="eyebrow">私密阅读</p>
        <h1>这篇文档仅作者可见</h1>
        <p class="private-access-owner" v-if="accessInfo?.owner">
          作者：<RouterLink :to="`/u/${accessInfo.owner.username}`">{{ accessInfo.owner.nickname }}</RouterLink>
        </p>
        <p class="private-access-lede">如果你需要阅读，可以向作者申请访问权限。作者通过后，这篇文档及其私密范围下的子文档会对你开放。</p>

        <p v-if="!accessInfo" class="muted loading-line">正在检查阅读权限…</p>
        <template v-else-if="!auth.user">
          <RouterLink :to="{ path: '/login', query: { redirect: route.fullPath } }" class="btn btn-primary btn-sm">登录后申请</RouterLink>
        </template>
        <template v-else-if="accessInfo.status === 'pending'">
          <div class="private-access-status">
            <strong>申请已提交</strong>
            <span>请等待作者处理，处理结果会出现在「消息 → 通知」里。</span>
          </div>
          <button v-if="accessInfo.requestID" class="btn btn-ghost btn-sm" type="button" :disabled="cancellingAccess" @click="cancelAccess">
            {{ cancellingAccess ? '撤回中…' : '撤回申请' }}
          </button>
        </template>
        <template v-else-if="accessInfo.status === 'approved'">
          <div class="private-access-status is-approved">
            <strong>已通过申请</strong>
            <span>刷新页面即可开始阅读。</span>
          </div>
          <button class="btn btn-primary btn-sm" type="button" @click="load">刷新阅读</button>
        </template>
        <template v-else>
          <p v-if="accessInfo.status === 'denied'" class="private-access-denied">上次申请未通过，可以补充说明后重新申请。</p>
          <label class="field private-access-note">
            <span>申请说明 <small>可选，最多 200 字</small></span>
            <textarea v-model="accessNote" rows="3" maxlength="200" placeholder="简单说明你为什么需要阅读这篇文档。"></textarea>
          </label>
          <button class="btn btn-primary btn-sm" type="button" :disabled="requestingAccess" @click="requestAccess">
            {{ requestingAccess ? '提交中…' : '申请阅读' }}
          </button>
        </template>

        <RouterLink to="/docs" class="text-link private-access-back">返回知识库</RouterLink>
      </section>
    </div>

    <template v-else-if="doc">
      <div class="container docs-topbar">
        <button class="docs-side-toggle" type="button" @click="sideOpen = !sideOpen">
          <Icon name="layers" :size="16" />
          目录
        </button>
        <nav class="breadcrumb" aria-label="面包屑">
          <RouterLink to="/docs">知识库</RouterLink>
          <template v-for="crumb in breadcrumbs" :key="crumb.id">
            <span class="crumb-sep">/</span>
            <RouterLink v-if="crumb.id !== doc.id" :to="`/docs/${crumb.id}`">{{ crumb.title }}</RouterLink>
            <span v-else class="crumb-current">{{ crumb.title }}</span>
          </template>
        </nav>
      </div>

      <div class="container docs-layout docs-layout--toc">
        <aside class="docs-side" :class="{ open: sideOpen }">
          <div class="docs-side-head">
            <RouterLink to="/docs">文档目录</RouterLink>
            <button class="icon-btn docs-side-close" type="button" aria-label="关闭目录" @click="sideOpen = false">
              <Icon name="close" :size="16" />
            </button>
          </div>
          <DocTree :nodes="tree" :active-id="doc.id" />
          <p v-if="!tree.length" class="muted docs-side-empty">暂无其他文档。</p>
        </aside>

        <article class="docs-article">
          <div v-if="doc.status === 'draft'" class="draft-banner">
            <Icon name="info" :size="16" />
            这是草稿，只有你能看到。
          </div>

          <header class="docs-article-head">
            <div class="article-kicker">
              <span class="kicker-link">文档</span>
              <span class="kicker-sep">/</span>
              <span>{{ formatDate(doc.updatedAt) }} 更新</span>
              <span v-if="docPrivate" class="private-kicker"><Icon name="lock" :size="12" /> 私密</span>
            </div>
            <h1>{{ doc.title }}</h1>
            <p v-if="doc.abstract" class="article-abstract">{{ doc.abstract }}</p>
            <div class="article-byline">
              <RouterLink :to="`/u/${doc.author.username}`" class="byline-author">
                <UserAvatar :username="doc.author.username" :nickname="doc.author.nickname" :size="36" />
                <span class="byline-text">
                  <strong>{{ doc.author.nickname }}</strong>
                  <span>{{ formatDate(doc.createdAt) }} 创建</span>
                </span>
              </RouterLink>
              <div class="byline-meta">
                <span><Icon name="eye" :size="15" /> {{ formatNumber(doc.viewCount) }}</span>
                <span><Icon name="clock" :size="15" /> {{ readingTime(doc.content) }} 分钟</span>
                <RouterLink v-if="auth.user" :to="{ name: 'ask', query: { doc: doc.id } }" class="text-link"><Icon name="spark" :size="14" /> 问 AI</RouterLink>
            <RouterLink v-if="canEdit" :to="`/write/${doc.id}`" class="text-link"><Icon name="edit" :size="14" /> 编辑</RouterLink>
              </div>
            </div>
          </header>

          <figure class="docs-cover">
            <CoverImage :doc="doc" />
          </figure>

          <ReadingTools />
          <MarkdownView :source="doc.content" @toc="toc = $event" />

          <div v-if="doc.tags?.length" class="article-tags">
            <Icon name="tag" :size="15" />
            <TagPill v-for="tag in doc.tags" :key="tag" :name="tag" />
          </div>

          <DocActions
            class="doc-actions--mobile"
            :doc="doc"
            :liked="liked"
            :favorited="favorited"
            layout="inline"
            @like="toggleLike"
            @favorite="toggleFavorite"
            @share="share"
          />

          <nav v-if="prevDoc || nextDoc" class="docs-pager">
            <button v-if="prevDoc" type="button" class="pager-link" @click="goDoc(prevDoc.id)">
              <span class="pager-label"><Icon name="arrowLeft" :size="14" /> 上一篇</span>
              <strong>{{ prevDoc.title }}</strong>
            </button>
            <span v-else></span>
            <button v-if="nextDoc" type="button" class="pager-link pager-link--right" @click="goDoc(nextDoc.id)">
              <span class="pager-label">下一篇 <Icon name="arrowRight" :size="14" /></span>
              <strong>{{ nextDoc.title }}</strong>
            </button>
          </nav>

          <Backlinks :doc-id="doc.id" />

          <CommentSection :doc-id="doc.id" :open="doc.openComment !== false" @count="doc.commentCount = $event" />
        </article>

        <aside class="docs-toc">
          <DocActions
            :doc="doc"
            :liked="liked"
            :favorited="favorited"
            layout="aside"
            @like="toggleLike"
            @favorite="toggleFavorite"
            @share="share"
          />
          <div v-if="visibleToc.length" class="aside-toc">
            <h4>本页目录</h4>
            <nav class="toc">
              <a
                v-for="item in visibleToc"
                :key="item.id"
                :href="`#${item.id}`"
                :class="[`toc-level-${item.level}`, { active: activeId === item.id }]"
                :aria-current="activeId === item.id ? 'location' : undefined"
              >
                {{ item.text }}
              </a>
            </nav>
          </div>
        </aside>
      </div>
      <ShareCard v-if="shareOpen" :doc="doc" :url="shareUrl" @close="shareOpen = false" />
    </template>
  </div>
</template>
