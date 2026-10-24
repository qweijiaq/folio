<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import CoverImage from '../components/CoverImage.vue'
import UserAvatar from '../components/UserAvatar.vue'
import MarkdownView from '../components/MarkdownView.vue'
import CommentSection from '../components/CommentSection.vue'
import TagPill from '../components/TagPill.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { formatDate, formatNumber, readingTime, shortDate } from '../utils/format'

const route = useRoute()
const router = useRouter()

const doc = ref(null)
const liked = ref(false)
const favorited = ref(false)
const following = ref(false)
const toc = ref([])
const related = ref([])
const loading = ref(true)
const notFound = ref(false)

const canEdit = computed(() => {
  if (!doc.value || !auth.user) return false
  return doc.value.author.id === auth.user.id || auth.user.role === 'admin'
})

const articleToc = computed(() => toc.value.filter((item) => item.level <= 3))

async function load() {
  loading.value = true
  notFound.value = false
  related.value = []
  try {
    const data = await api.get(`/docs/${route.params.id}`)
    if (data.doc.kind === 'doc') {
      router.replace(`/docs/${data.doc.id}`)
      return
    }
    doc.value = data.doc
    liked.value = data.liked
    favorited.value = data.favorited
    following.value = data.followingAuthor
    loadRelated(data.doc.id)
  } catch (err) {
    if (err.status === 404) notFound.value = true
    else toastError(err)
  } finally {
    loading.value = false
  }
}

async function loadRelated(id) {
  try {
    const data = await api.get(`/docs/${id}/related`)
    related.value = data.items
  } catch {
    related.value = []
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
    toast(data.favorited ? '已收藏，可在「我的」中查看' : '已取消收藏')
  } catch (err) {
    toastError(err)
  }
}

async function toggleFollow() {
  if (!requireLogin()) return
  try {
    const username = doc.value.author.username
    const data = following.value
      ? await api.del(`/users/${username}/follow`)
      : await api.post(`/users/${username}/follow`)
    following.value = data.following
    toast(data.following ? `已关注 ${doc.value.author.nickname}` : '已取消关注')
  } catch (err) {
    toastError(err)
  }
}

async function share() {
  const url = window.location.href
  try {
    if (navigator.share) {
      await navigator.share({ title: doc.value.title, url })
    } else {
      await navigator.clipboard.writeText(url)
      toast('链接已复制')
    }
  } catch {
    /* 用户取消分享 */
  }
}

onMounted(load)
watch(
  () => route.params.id,
  load
)
</script>

<template>
  <div class="page article-page">
    <div v-if="loading" class="container page-loading">
      <p class="muted loading-line">正在取回文章…</p>
    </div>

    <div v-else-if="notFound" class="container page-loading">
      <EmptyState icon="info" title="这篇文章不存在" description="它可能被删除，或者还是一个没有公开的草稿。">
        <RouterLink to="/articles" class="btn btn-ghost btn-sm">返回文章列表</RouterLink>
      </EmptyState>
    </div>

    <article v-else-if="doc" class="container article-container">
      <div v-if="doc.status === 'draft'" class="draft-banner">
        <Icon name="info" :size="16" />
        这是草稿，只有你能看到。发布后才会计入文章列表。
      </div>

      <header class="article-head">
        <div class="article-kicker">
          <RouterLink to="/articles" class="kicker-link">文章</RouterLink>
          <template v-if="doc.category">
            <span class="kicker-sep">/</span>
            <RouterLink
              :to="{ name: 'articles', query: { category: String(doc.category.id) } }"
              class="kicker-category"
            >
              {{ doc.category.name }}
            </RouterLink>
          </template>
          <span class="kicker-sep">/</span>
          <span>{{ shortDate(doc.publishedAt || doc.createdAt) }}</span>
        </div>
        <h1>{{ doc.title }}</h1>
        <p v-if="doc.abstract" class="article-abstract">{{ doc.abstract }}</p>

        <div class="article-byline">
          <RouterLink :to="`/u/${doc.author.username}`" class="byline-author">
            <UserAvatar :username="doc.author.username" :nickname="doc.author.nickname" :size="40" />
            <span class="byline-text">
              <strong>{{ doc.author.nickname }}</strong>
              <span>{{ formatDate(doc.publishedAt || doc.createdAt) }}</span>
            </span>
          </RouterLink>
          <div class="byline-meta">
            <span><Icon name="eye" :size="15" /> {{ formatNumber(doc.viewCount) }}</span>
            <span><Icon name="clock" :size="15" /> {{ readingTime(doc.content) }} 分钟</span>
            <RouterLink v-if="canEdit" :to="`/write/${doc.id}`" class="text-link"><Icon name="edit" :size="14" /> 编辑</RouterLink>
          </div>
        </div>
      </header>

      <figure class="article-cover">
        <CoverImage :doc="doc" />
      </figure>

      <div class="article-layout">
        <div class="article-body">
          <MarkdownView :source="doc.content" @toc="toc = $event" />

          <div v-if="doc.tags?.length" class="article-tags">
            <Icon name="tag" :size="15" />
            <TagPill v-for="tag in doc.tags" :key="tag" :name="tag" />
          </div>

          <div class="article-actions">
            <button class="action-btn" :class="{ active: liked }" type="button" @click="toggleLike">
              <Icon name="heart" :size="17" />
              <span>{{ liked ? '已点赞' : '点赞' }}</span>
              <span class="action-count">{{ doc.likeCount || 0 }}</span>
            </button>
            <button class="action-btn" :class="{ active: favorited }" type="button" @click="toggleFavorite">
              <Icon name="bookmark" :size="17" />
              <span>{{ favorited ? '已收藏' : '收藏' }}</span>
              <span class="action-count">{{ doc.favoriteCount || 0 }}</span>
            </button>
            <button class="action-btn" type="button" @click="share">
              <Icon name="share" :size="17" />
              <span>分享</span>
            </button>
            <RouterLink
              v-if="auth.user && auth.user.id !== doc.author.id"
              :to="{ path: '/messages', query: { tab: 'messages', with: doc.author.username } }"
              class="action-btn"
            >
              <Icon name="message" :size="17" />
              <span>私信作者</span>
            </RouterLink>
          </div>

          <div class="author-card">
            <RouterLink :to="`/u/${doc.author.username}`">
              <UserAvatar :username="doc.author.username" :nickname="doc.author.nickname" :size="52" />
            </RouterLink>
            <div class="author-card-main">
              <div class="author-card-name">
                <RouterLink :to="`/u/${doc.author.username}`">{{ doc.author.nickname }}</RouterLink>
                <span>@{{ doc.author.username }}</span>
              </div>
              <p>{{ doc.author.bio || '这个人还没有写简介。' }}</p>
            </div>
            <div v-if="auth.user && auth.user.id !== doc.author.id" class="author-card-actions">
              <button class="btn btn-sm" :class="following ? 'btn-ghost' : 'btn-primary'" type="button" @click="toggleFollow">
                <Icon :name="following ? 'following' : 'follow'" :size="15" />
                {{ following ? '已关注' : '关注' }}
              </button>
            </div>
            <div v-else-if="!auth.user" class="author-card-actions">
              <button class="btn btn-ghost btn-sm" type="button" @click="toggleFollow">关注作者</button>
            </div>
          </div>

          <section v-if="related.length" class="related-section">
            <header class="section-head section-head--tight">
              <div>
                <span class="eyebrow">继续阅读</span>
                <h2>相关文章</h2>
              </div>
            </header>
            <div class="related-list">
              <RouterLink v-for="item in related" :key="item.id" :to="`/articles/${item.id}`" class="related-row">
                <span class="related-kind">文章</span>
                <span class="related-main">
                  <strong>{{ item.title }}</strong>
                  <span>{{ item.abstract }}</span>
                </span>
                <Icon name="arrowRight" :size="16" />
              </RouterLink>
            </div>
          </section>

          <CommentSection :doc-id="doc.id" :open="doc.openComment !== false" @count="doc.commentCount = $event" />
        </div>

        <aside v-if="articleToc.length > 2" class="article-aside">
          <div class="aside-sticky">
            <h4>目录</h4>
            <nav class="toc">
              <a v-for="item in articleToc" :key="item.id" :href="`#${item.id}`" :class="`toc-level-${item.level}`">
                {{ item.text }}
              </a>
            </nav>
          </div>
        </aside>
      </div>
    </article>
  </div>
</template>
