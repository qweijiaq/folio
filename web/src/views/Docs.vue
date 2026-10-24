<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toastError } from '../stores/toast'
import { buildTree } from '../utils/tree'
import DocTree from '../components/DocTree.vue'
import DocCard from '../components/DocCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

const tree = ref([])
const docs = ref([])
const loading = ref(true)
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

onMounted(async () => {
  try {
    const [treeData, docData] = await Promise.all([
      api.get('/doc-tree'),
      api.get('/docs', { kind: 'doc', sort: 'popular', pageSize: 12 })
    ])
    tree.value = buildTree(treeData.items)
    docs.value = docData.items
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page docs-page">
    <header class="page-hero container">
      <span class="eyebrow">知识库</span>
      <h1>文档</h1>
      <p class="page-lede">按主题整理的长期笔记：手册、速查、清单。它们会被反复修订，而不是随时间沉底。</p>
    </header>

    <div class="container docs-layout docs-layout--overview">
      <aside class="docs-side">
        <div class="docs-side-head">
          <span>目录</span>
          <Icon name="layers" :size="15" />
        </div>
        <DocTree :nodes="tree" />
        <p v-if="!tree.length && !loading" class="muted docs-side-empty">还没有建立文档目录。</p>
      </aside>

      <div class="docs-content">
        <p v-if="loading" class="muted loading-line">正在整理书架…</p>
        <EmptyState
          v-else-if="!docs.length"
          icon="book"
          title="知识库还是空的"
          description="先创建一篇文档，再用「上级文档」把它组织成树。"
        >
          <RouterLink v-if="staff" to="/write?kind=doc" class="btn btn-primary btn-sm">新建文档</RouterLink>
        </EmptyState>
        <template v-else>
          <div class="section-head section-head--tight">
            <div>
              <span class="eyebrow">常被翻阅</span>
              <h2>全部文档</h2>
            </div>
            <RouterLink v-if="staff" to="/write?kind=doc" class="text-link">新建文档 <Icon name="plus" :size="14" /></RouterLink>
          </div>
          <div class="doc-grid">
            <DocCard v-for="doc in docs" :key="doc.id" :doc="doc" />
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
