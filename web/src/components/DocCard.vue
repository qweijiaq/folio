<script setup>
import { computed } from 'vue'
import CoverImage from './CoverImage.vue'
import UserAvatar from './UserAvatar.vue'
import Icon from './Icon.vue'
import { shortDate, formatNumber } from '../utils/format'

const props = defineProps({
  doc: { type: Object, required: true },
  variant: { type: String, default: 'grid' }
})

const href = computed(() => `/${props.doc.kind === 'doc' ? 'docs' : 'articles'}/${props.doc.id}`)
</script>

<template>
  <RouterLink :to="href" class="doc-card" :class="`doc-card--${variant}`">
    <div class="doc-card-cover">
      <CoverImage :doc="doc" />
      <span class="doc-kind">{{ doc.kind === 'doc' ? '文档' : '文章' }}</span>
      <span v-if="doc.status === 'draft'" class="badge badge-draft">草稿</span>
      <span v-if="doc.private" class="doc-private"><Icon name="lock" :size="12" /> 私密</span>
      <span v-if="doc.pinned" class="doc-pin"><Icon name="star" :size="13" /> 置顶</span>
    </div>
    <div class="doc-card-body">
      <h3 class="doc-card-title">{{ doc.title }}</h3>
      <p v-if="doc.abstract" class="doc-card-abstract">{{ doc.abstract }}</p>
      <div class="doc-card-meta">
        <UserAvatar :username="doc.author?.username" :nickname="doc.author?.nickname" :size="22" />
        <span class="author">{{ doc.author?.nickname }}</span>
        <span v-if="doc.category" class="doc-category">{{ doc.category.name }}</span>
        <span class="meta-dot">·</span>
        <span>{{ shortDate(doc.publishedAt || doc.createdAt) }}</span>
        <span class="meta-dot">·</span>
        <span>{{ formatNumber(doc.viewCount) }} 阅读</span>
        <span v-if="doc.commentCount" class="meta-dot">·</span>
        <span v-if="doc.commentCount">{{ doc.commentCount }} 评论</span>
      </div>
    </div>
  </RouterLink>
</template>
