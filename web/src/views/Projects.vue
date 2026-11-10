<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { toastError } from '../stores/toast'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

usePageMeta(() => ({
  title: '项目',
  description: '知微的项目墙：做过的东西、正在做的事，以及可以继续访问的入口。',
  url: '/projects',
  jsonLd: {
    '@context': 'https://schema.org',
    '@type': 'CollectionPage',
    name: '知微 · 项目墙',
    description: '做过的东西、正在做的事，以及可以继续访问的入口。',
    url: window.location.origin + '/projects'
  }
}))

const projects = ref([])
const loading = ref(true)

function projectIndex(index) {
  return String(index + 1).padStart(2, '0')
}

onMounted(async () => {
  try {
    const data = await api.get('/projects')
    projects.value = data.items || []
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page projects-page">
    <header class="page-hero container">
      <span class="eyebrow">项目 · 作品</span>
      <h1>做过的东西，正在做的事</h1>
      <p class="page-lede">一部分已经稳定运行，一部分还在慢慢长大。每张卡片都尽量留一个可以继续走下去的入口。</p>
    </header>

    <div class="container">
      <p v-if="loading" class="muted loading-line">正在整理项目…</p>
      <EmptyState
        v-else-if="!projects.length"
        icon="layers"
        title="项目墙还是空的"
        description="管理员可以在后台「项目管理」里添加第一个项目。"
      />

      <div v-else class="project-grid">
        <article v-for="(project, index) in projects" :key="project.id" class="project-card">
          <div class="project-cover">
            <img
              v-if="project.coverURL"
              :src="project.coverURL"
              :alt="`${project.title} 封面`"
              loading="lazy"
            />
            <div v-else class="project-fallback">
              <span class="project-index">{{ projectIndex(index) }}</span>
              <span class="project-cover-line" />
              <span class="project-cover-note">{{ project.tags?.slice(0, 3).join(' · ') || 'PROJECT' }}</span>
            </div>
          </div>

          <div class="project-body">
            <header class="project-head">
              <h2>{{ project.title }}</h2>
              <span class="project-number">{{ projectIndex(index) }}</span>
            </header>
            <p class="project-summary">{{ project.summary || '这个项目还没有写简介。' }}</p>

            <div v-if="project.tags?.length" class="project-tags">
              <span v-for="tag in project.tags" :key="tag">#{{ tag }}</span>
            </div>

            <footer class="project-links">
              <a
                v-if="project.url"
                class="btn btn-primary btn-sm"
                :href="project.url"
                target="_blank"
                rel="noopener noreferrer"
              >
                访问项目
                <Icon name="arrowRight" :size="14" />
              </a>
              <a
                v-if="project.repoURL"
                class="btn btn-ghost btn-sm"
                :href="project.repoURL"
                target="_blank"
                rel="noopener noreferrer"
              >
                <Icon name="github" :size="15" />
                GitHub
              </a>
              <span v-if="!project.url && !project.repoURL" class="project-muted">整理中</span>
            </footer>
          </div>
        </article>
      </div>
    </div>
  </div>
</template>
