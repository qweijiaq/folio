<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { mergeSite, site } from '../stores/site'
import CoverImage from '../components/CoverImage.vue'
import DocCard from '../components/DocCard.vue'
import DocCardSkeleton from '../components/DocCardSkeleton.vue'
import TagPill from '../components/TagPill.vue'
import UserAvatar from '../components/UserAvatar.vue'
import Icon from '../components/Icon.vue'
import RoughAccent from '../components/RoughAccent.vue'
import { shortDate, formatNumber } from '../utils/format'

const data = ref(null)
const loading = ref(true)
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

onMounted(async () => {
  try {
    data.value = await api.get('/site')
    mergeSite(data.value.site)
  } catch {
    /* 首页在接口不可用时仍展示静态文案 */
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page home-page">
    <section class="hero container">
      <div class="hero-copy">
        <p class="eyebrow">一个人的知识花园</p>
        <h1>
          把零散的思考<br />
          整理成<span class="hero-accent">可重访<RoughAccent variant="underline" class="hero-underline" /></span>的路径。
        </h1>
        <p class="hero-lede">{{ site.description }}</p>
        <div class="hero-actions">
          <RouterLink to="/articles" class="btn btn-primary">
            开始阅读
            <Icon name="arrowRight" :size="16" />
          </RouterLink>
          <RouterLink to="/docs" class="btn btn-ghost">探索知识库</RouterLink>
        </div>
        <dl class="hero-stats">
          <div>
            <dt>文章</dt>
            <dd>{{ data?.stats?.articles ?? '—' }}</dd>
          </div>
          <div>
            <dt>文档</dt>
            <dd>{{ data?.stats?.docs ?? '—' }}</dd>
          </div>
          <div>
            <dt>读者</dt>
            <dd>{{ data?.stats?.users ?? '—' }}</dd>
          </div>
          <div>
            <dt>阅读</dt>
            <dd>{{ data?.stats?.views ? formatNumber(data.stats.views) : '—' }}</dd>
          </div>
        </dl>
      </div>

      <div v-if="loading" class="hero-feature hero-feature--skeleton" aria-hidden="true">
        <div class="skeleton-cover"></div>
        <div class="hero-feature-body">
          <span class="skeleton-line skeleton-line--kicker"></span>
          <span class="skeleton-line skeleton-line--title"></span>
          <span class="skeleton-line"></span>
          <span class="skeleton-line skeleton-line--short"></span>
          <span class="skeleton-line skeleton-line--meta"></span>
        </div>
      </div>

      <RouterLink v-else-if="data?.featured" :to="`/articles/${data.featured.id}`" class="hero-feature">
        <div class="hero-feature-cover">
          <CoverImage :doc="data.featured" />
        </div>
        <div class="hero-feature-body">
          <span class="hero-feature-kicker">
            <Icon name="star" :size="13" />
            置顶文章
          </span>
          <h2>{{ data.featured.title }}</h2>
          <p>{{ data.featured.abstract }}</p>
          <div class="hero-feature-meta">
            <UserAvatar :username="data.featured.author.username" :nickname="data.featured.author.nickname" :size="24" />
            <span>{{ data.featured.author.nickname }}</span>
            <span class="meta-dot">·</span>
            <span>{{ shortDate(data.featured.publishedAt || data.featured.createdAt) }}</span>
            <span class="meta-dot">·</span>
            <span>{{ formatNumber(data.featured.viewCount) }} 阅读</span>
          </div>
        </div>
      </RouterLink>

      <div v-else class="hero-feature hero-feature--empty">
        <Icon name="feather" :size="28" />
        <p v-if="staff">还没有文章。<RouterLink to="/write">写下第一篇</RouterLink>，让这里开始生长。</p>
        <p v-else>还没有文章。文章会出现在这里。</p>
      </div>
    </section>

    <section v-if="loading" class="section container">
      <div class="doc-grid">
        <DocCardSkeleton v-for="i in 3" :key="i" />
      </div>
    </section>

    <template v-else>
      <section v-if="data?.recent?.length" class="section container">
        <header class="section-head">
          <div>
            <span class="eyebrow">最近</span>
            <h2>最近文章</h2>
          </div>
          <RouterLink to="/articles" class="text-link">全部文章 <Icon name="arrowRight" :size="15" /></RouterLink>
        </header>
        <div class="doc-grid">
          <DocCard v-for="doc in data.recent" :key="doc.id" :doc="doc" />
        </div>
      </section>

      <section class="section container home-split">
        <div>
          <header class="section-head">
            <div>
              <span class="eyebrow">知识库</span>
              <h2>可以查阅的文档</h2>
            </div>
            <RouterLink to="/docs" class="text-link">进入知识库 <Icon name="arrowRight" :size="15" /></RouterLink>
          </header>
          <div class="compact-list">
            <RouterLink v-for="doc in data?.docs || []" :key="doc.id" :to="`/docs/${doc.id}`" class="compact-row">
              <span class="compact-icon" aria-hidden="true"><Icon name="book" :size="15" /></span>
              <div class="compact-main">
                <h3>{{ doc.title }}</h3>
                <p>{{ doc.abstract }}</p>
              </div>
              <span class="compact-meta">{{ formatNumber(doc.viewCount) }} 阅读</span>
            </RouterLink>
            <p v-if="!data?.docs?.length" class="muted">知识库还在整理中。</p>
          </div>
        </div>

        <aside class="home-aside">
          <div class="aside-card">
            <h3>标签</h3>
            <div class="tag-cloud">
              <TagPill v-for="tag in data?.tags || []" :key="tag.name" :name="tag.name" :count="tag.count" />
            </div>
          </div>
          <div class="aside-card">
            <h3>最近加入</h3>
            <div class="people-list">
              <RouterLink v-for="user in data?.people || []" :key="user.id" :to="`/u/${user.username}`" class="people-row">
                <UserAvatar :username="user.username" :nickname="user.nickname" :size="32" />
                <div class="people-text">
                  <strong>{{ user.nickname }}</strong>
                  <span>@{{ user.username }}</span>
                </div>
              </RouterLink>
            </div>
          </div>
        </aside>
      </section>

      <section class="container home-quote">
        <RoughAccent variant="squiggle" class="quote-squiggle" />
        <blockquote>统一不是让所有东西长得一样，而是让所有东西看起来像来自同一个地方。</blockquote>
        <span>— 摘自《一张好的封面，为什么不需要设计》</span>
      </section>
    </template>
  </div>
</template>
