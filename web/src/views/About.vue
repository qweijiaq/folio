<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { site } from '../stores/site'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const catalog = ref(null)
const stats = ref({})
const loading = ref(true)
const error = ref('')
const query = ref('')

usePageMeta(() => ({
  title: '关于',
  description: '系统了解知微的功能：文章、知识库、写作、评论、私信、AI 问答、管理与存储。',
  url: '/about',
  jsonLd: {
    '@context': 'https://schema.org',
    '@type': 'AboutPage',
    name: '关于知微',
    description: '系统了解知微的个人知识站功能。',
    url: window.location.origin + '/about'
  }
}))

const categories = computed(() => catalog.value?.categories || [])
const totalFeatures = computed(() => categories.value.reduce((sum, category) => sum + category.features.length, 0))
const filteredCategories = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return categories.value
  return categories.value
    .map((category) => ({
      ...category,
      features: category.features.filter((feature) => {
        const haystack = [
          category.title,
          feature.title,
          feature.summary,
          ...(feature.details || []),
          ...(feature.keywords || [])
        ].join(' ').toLowerCase()
        return haystack.includes(keyword)
      })
    }))
    .filter((category) => category.features.length)
})

function audienceLabel(feature) {
  return feature.audience === 'staff' ? '管理员' : '所有访客'
}

onMounted(async () => {
  try {
    const [features, home] = await Promise.all([
      api.get('/site/features'),
      api.get('/site')
    ])
    catalog.value = features.catalog
    stats.value = home.stats || {}
  } catch (err) {
    error.value = err.message || '加载站点介绍失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page about-page">
    <section class="about-hero container">
      <div class="about-hero-copy">
        <p class="eyebrow">关于 · 知微</p>
        <h1>一个可以慢慢长大的<br /><span>个人知识站</span></h1>
        <p class="about-lede">{{ catalog?.intro || site.description }}</p>
        <div class="about-actions">
          <RouterLink class="btn btn-primary" to="/articles">开始阅读</RouterLink>
          <RouterLink class="btn btn-ghost" to="/ask">问 AI</RouterLink>
        </div>
      </div>
      <aside class="about-note">
        <span class="about-note-mark">✳</span>
        <p>这份关于页同时是 AI 问答的「站点帮助」知识库。新功能加入后，只需要更新同一份目录。</p>
        <span class="about-version" v-if="catalog">版本 {{ catalog.version }} · 更新于 {{ catalog.updatedAt }}</span>
      </aside>
    </section>

    <section class="about-stats container">
      <div><strong>{{ stats.articles || 0 }}</strong><span>篇文章</span></div>
      <div><strong>{{ stats.docs || 0 }}</strong><span>篇文档</span></div>
      <div><strong>{{ stats.users || 0 }}</strong><span>位读者</span></div>
      <div><strong>{{ totalFeatures }}</strong><span>项功能说明</span></div>
    </section>

    <div class="about-layout container">
      <aside class="about-nav">
        <p class="about-nav-title">目录</p>
        <nav>
          <a v-for="category in filteredCategories" :key="category.id" :href="`#${category.id}`">
            <Icon :name="category.icon || 'layers'" :size="15" />
            {{ category.title }}
          </a>
        </nav>
        <label class="about-search">
          <span>在页面内搜索</span>
          <input v-model="query" type="search" placeholder="例如：邀请、私信、七牛" />
        </label>
      </aside>

      <div class="about-content">
        <p v-if="loading" class="muted loading-line">正在整理站点功能…</p>
        <p v-else-if="error" class="form-error">{{ error }}</p>
        <template v-else>
          <p v-if="query && !filteredCategories.length" class="empty-inline">没有找到匹配的功能，换个关键词试试。</p>
          <section v-for="category in filteredCategories" :id="category.id" :key="category.id" class="about-category">
            <header class="about-category-head">
              <span class="about-category-icon"><Icon :name="category.icon || 'layers'" :size="20" /></span>
              <div>
                <h2>{{ category.title }}</h2>
                <p>{{ category.summary }}</p>
              </div>
              <span class="about-category-count">{{ category.features.length }} 项</span>
            </header>

            <div class="about-feature-list">
              <article v-for="feature in category.features" :id="feature.id" :key="feature.id" class="about-feature">
                <header class="about-feature-head">
                  <h3>{{ feature.title }}</h3>
                  <span class="badge" :class="{ 'about-badge-staff': feature.audience === 'staff' }">
                    {{ audienceLabel(feature) }}
                  </span>
                </header>
                <p class="about-feature-summary">{{ feature.summary }}</p>
                <ul v-if="feature.details?.length" class="about-feature-details">
                  <li v-for="detail in feature.details" :key="detail">{{ detail }}</li>
                </ul>
                <div v-if="feature.route" class="about-feature-foot">
                  <a :href="feature.route" class="text-link">
                    去体验 <Icon name="arrowRight" :size="14" />
                  </a>
                </div>
                <figure v-for="shot in feature.screenshots || []" :key="shot" class="about-shot">
                  <a :href="shot" target="_blank" rel="noopener">
                    <img :src="shot" :alt="`${feature.title}截图`" />
                  </a>
                </figure>
                <video v-if="feature.video" class="about-video" controls preload="metadata">
                  <source :src="feature.video" />
                </video>
              </article>
            </div>
          </section>
        </template>
      </div>
    </div>
  </div>
</template>
