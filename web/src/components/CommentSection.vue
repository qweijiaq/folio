<script setup>
import { onMounted, ref, nextTick } from 'vue'
import { api } from '../api/client'
import { confirmDialog } from '../stores/confirm'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { timeAgo } from '../utils/format'
import UserAvatar from './UserAvatar.vue'
import Icon from './Icon.vue'
import Pagination from './Pagination.vue'
import MentionText from './MentionText.vue'

const props = defineProps({
  docId: { type: [Number, String], required: true },
  open: { type: Boolean, default: true }
})
const emit = defineEmits(['count'])

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(true)
const submitting = ref(false)
const content = ref('')
const replyTo = ref(null)
const textarea = ref(null)

async function load() {
  loading.value = true
  try {
    const data = await api.get(`/docs/${props.docId}/comments`, { page: page.value, pageSize })
    items.value = data.items
    total.value = data.total
    emit('count', data.total)
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
}

function focusComposer() {
  nextTick(() => textarea.value?.focus())
}

function startReply(comment) {
  if (!auth.user) {
    toast('登录后才能回复')
    return
  }
  replyTo.value = comment
  focusComposer()
}

function cancelReply() {
  replyTo.value = null
}

async function submit() {
  if (!auth.user) {
    toast('请先登录')
    return
  }
  const text = content.value.trim()
  if (!text) {
    toast('写点什么再发布吧')
    return
  }
  submitting.value = true
  const wasReply = Boolean(replyTo.value)
  try {
    await api.post(`/docs/${props.docId}/comments`, {
      content: text,
      parentID: replyTo.value?.id ?? null
    })
    content.value = ''
    replyTo.value = null
    page.value = 1
    await load()
    toast(wasReply ? '回复已发布' : '评论已发布')
  } catch (err) {
    toastError(err)
  } finally {
    submitting.value = false
  }
}

async function toggleLike(comment) {
  if (!auth.user) {
    toast('登录后才能点赞')
    return
  }
  try {
    const data = await api.post(`/comments/${comment.id}/like`)
    comment.liked = data.liked
    comment.likeCount = data.likeCount
  } catch (err) {
    toastError(err)
  }
}

async function remove(comment) {
  if (!(await confirmDialog({ title: '删除评论', message: '确定删除这条评论吗？', confirmText: '删除', danger: true }))) return
  try {
    await api.del(`/comments/${comment.id}`)
    if (items.value.length === 1 && page.value > 1) page.value -= 1
    await load()
    toast('评论已删除')
  } catch (err) {
    toastError(err)
  }
}

function canDelete(comment) {
  return auth.user && (auth.user.id === comment.user.id || auth.user.role === 'admin')
}

function changePage(p) {
  page.value = p
  load()
}

onMounted(load)
</script>

<template>
  <section id="comments" class="comment-section">
    <header class="section-head">
      <div>
        <span class="eyebrow">讨论</span>
        <h2>评论 <span v-if="total" class="count">{{ total }}</span></h2>
      </div>
    </header>

    <div v-if="!open" class="comment-closed">
      作者关闭了这篇内容的评论。已有的讨论仍然可以阅读和点赞。
    </div>

    <form v-if="open && auth.user" class="comment-form" @submit.prevent="submit">
      <UserAvatar :username="auth.user.username" :nickname="auth.user.nickname" :size="38" />
      <div class="comment-form-main">
        <div v-if="replyTo" class="reply-banner">
          回复 <strong>@{{ replyTo.user.nickname }}</strong>
          <button type="button" @click="cancelReply">取消</button>
        </div>
        <textarea
          ref="textarea"
          v-model="content"
          rows="3"
          maxlength="800"
          :placeholder="replyTo ? `回复 @${replyTo.user.nickname}…` : '写下你的想法…'"
        ></textarea>
        <div class="comment-form-foot">
          <span class="hint">{{ content.length }} / 800 · 用 @用户名 提及他人</span>
          <button class="btn btn-primary btn-sm" type="submit" :disabled="submitting">
            {{ submitting ? '发布中…' : replyTo ? '回复' : '发布' }}
          </button>
        </div>
      </div>
    </form>

    <div v-else-if="open" class="comment-login">
      <p>登录后可以评论、点赞和回复。</p>
      <RouterLink class="btn btn-ghost btn-sm" :to="{ path: '/login', query: { redirect: $route.fullPath } }">
        去登录
      </RouterLink>
    </div>

    <p v-if="loading" class="muted loading-line">正在加载评论…</p>
    <p v-else-if="!items.length" class="empty-inline">还没有评论，来写下第一条吧。</p>

    <div v-else class="comment-list">
      <article v-for="comment in items" :key="comment.id" class="comment">
        <UserAvatar :username="comment.user.username" :nickname="comment.user.nickname" :size="38" />
        <div class="comment-main">
          <header class="comment-head">
            <RouterLink :to="`/u/${comment.user.username}`" class="comment-author">{{ comment.user.nickname }}</RouterLink>
            <span class="comment-time">{{ timeAgo(comment.createdAt) }}</span>
          </header>
          <p class="comment-body"><MentionText :text="comment.content" /></p>
          <div class="comment-actions">
            <button type="button" :class="{ active: comment.liked }" @click="toggleLike(comment)">
              <Icon name="heart" :size="14" />
              <span>{{ comment.likeCount || '' }}</span>
            </button>
            <button v-if="open" type="button" @click="startReply(comment)">回复</button>
            <button v-if="canDelete(comment)" type="button" class="danger" @click="remove(comment)">删除</button>
          </div>

          <div v-if="comment.replies?.length" class="comment-replies">
            <article v-for="reply in comment.replies" :key="reply.id" class="comment comment--reply">
              <UserAvatar :username="reply.user.username" :nickname="reply.user.nickname" :size="30" />
              <div class="comment-main">
                <header class="comment-head">
                  <RouterLink :to="`/u/${reply.user.username}`" class="comment-author">{{ reply.user.nickname }}</RouterLink>
                  <span v-if="reply.replyToName" class="reply-to">回复 @{{ reply.replyToName }}</span>
                  <span class="comment-time">{{ timeAgo(reply.createdAt) }}</span>
                </header>
                <p class="comment-body"><MentionText :text="reply.content" /></p>
                <div class="comment-actions">
                  <button type="button" :class="{ active: reply.liked }" @click="toggleLike(reply)">
                    <Icon name="heart" :size="14" />
                    <span>{{ reply.likeCount || '' }}</span>
                  </button>
                  <button v-if="open" type="button" @click="startReply(reply)">回复</button>
                  <button v-if="canDelete(reply)" type="button" class="danger" @click="remove(reply)">删除</button>
                </div>
              </div>
            </article>
          </div>
        </div>
      </article>
    </div>

    <Pagination :page="page" :total="total" :page-size="pageSize" @change="changePage" />
  </section>
</template>
