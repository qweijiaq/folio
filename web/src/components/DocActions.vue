<script setup>
import { computed } from 'vue'
import { auth } from '../stores/auth'
import Icon from './Icon.vue'

const props = defineProps({
  doc: { type: Object, required: true },
  liked: { type: Boolean, default: false },
  favorited: { type: Boolean, default: false },
  layout: { type: String, default: 'inline' },
  showMessage: { type: Boolean, default: false }
})

defineEmits(['like', 'favorite', 'share'])

const canMessage = computed(() => {
  if (!props.showMessage || !auth.user || !props.doc?.author) return false
  return auth.user.id !== props.doc.author.id
})
</script>

<template>
  <div class="doc-actions" :class="`doc-actions--${layout}`">
    <button
      class="action-btn"
      :class="{ active: liked }"
      type="button"
      :aria-pressed="liked"
      @click="$emit('like')"
    >
      <Icon name="heart" :size="17" />
      <span>{{ liked ? '已点赞' : '点赞' }}</span>
      <span class="action-count">{{ doc.likeCount || 0 }}</span>
    </button>
    <button
      class="action-btn"
      :class="{ active: favorited }"
      type="button"
      :aria-pressed="favorited"
      @click="$emit('favorite')"
    >
      <Icon name="bookmark" :size="17" />
      <span>{{ favorited ? '已收藏' : '收藏' }}</span>
      <span class="action-count">{{ doc.favoriteCount || 0 }}</span>
    </button>
    <button class="action-btn" type="button" @click="$emit('share')">
      <Icon name="share" :size="17" />
      <span>分享</span>
    </button>
    <RouterLink
      v-if="canMessage"
      :to="{ path: '/messages', query: { tab: 'messages', with: doc.author.username } }"
      class="action-btn"
    >
      <Icon name="message" :size="17" />
      <span>私信作者</span>
    </RouterLink>
  </div>
</template>
