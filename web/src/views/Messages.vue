<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, getToken } from '../api/client'
import { auth, refreshInbox } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { timeAgo, shortDate } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '消息', description: '知微私信与通知', noindex: true }))

const tab = computed(() => (route.query.tab === 'notifications' ? 'notifications' : 'messages'))
const withUser = computed(() => String(route.query.with || ''))

const conversations = ref([])
const notifications = ref([])
const notices = ref([])
const notifUnread = ref(0)
const noticeUnread = ref(0)
const notifCounts = ref({})
const notifFilter = ref('all')
const notifUnreadOnly = ref(false)
const notifLoading = ref(false)
const notifFilters = [
  { key: 'all', label: '全部' },
  { key: 'comment', label: '评论' },
  { key: 'like', label: '点赞' },
  { key: 'follow', label: '关注' },
  { key: 'mention', label: '提及' },
  { key: 'access_request', label: '阅读申请' },
  { key: 'access_result', label: '申请结果' }
]
const messages = ref([])
const hasMore = ref(false)
const loadingOlder = ref(false)
const peer = ref(null)
const draft = ref('')
const sending = ref(false)
const loadingConv = ref(true)
const loadingThread = ref(false)
const threadBody = ref(null)
const userQuery = ref('')
const userResults = ref([])
const searchingUsers = ref(false)
const composeTextarea = ref(null)
const emojiOpen = ref(false)
const imageInput = ref(null)
const uploadingImage = ref(false)
const wsConnected = ref(false)
const emojis = ['😀', '😄', '🙂', '😉', '😍', '🤔', '😎', '😢', '😡', '👍', '👏', '🙏', '🎉', '❤️', '🔥', '✨', '🌱', '☕', '📚', '💡', '✅', '👀', '😅', '🤝']

let pollTimer = null
let searchTimer = null
let socket = null
let reconnectTimer = null
let reconnectDelay = 1000
let closedByUser = false

async function loadConversations() {
  try {
    const data = await api.get('/messages/conversations')
    conversations.value = data.items
  } catch (err) {
    /* 未登录由路由拦截 */
  } finally {
    loadingConv.value = false
  }
}

async function loadThread(auto = false) {
  if (!withUser.value) {
    peer.value = null
    messages.value = []
    return
  }
  const el = threadBody.value
  const atBottom = !el || el.scrollHeight - el.scrollTop - el.clientHeight < 90
  if (!auto) loadingThread.value = true
  try {
    const data = await api.get(`/messages/with/${withUser.value}`)
    peer.value = data.peer
    if (auto && messages.value.length) {
      const existing = new Set(messages.value.map((message) => message.id))
      const merged = [...messages.value]
      for (const message of data.items) {
        if (!existing.has(message.id)) merged.push(message)
      }
      merged.sort((a, b) => a.id - b.id)
      messages.value = merged
    } else {
      messages.value = data.items
      hasMore.value = Boolean(data.hasMore)
    }
    if (!auto || atBottom) {
      await nextTick()
      scrollBottom()
    }
    const current = conversations.value.find((item) => item.user.username === withUser.value)
    if (current) current.unread = 0
    await Promise.all([loadConversations(), refreshInbox()])
  } catch (err) {
    toastError(err)
    peer.value = null
    messages.value = []
    hasMore.value = false
  } finally {
    loadingThread.value = false
  }
}

async function loadOlder() {
  if (!peer.value || loadingOlder.value || !messages.value.length || !hasMore.value) return
  const el = threadBody.value
  const previousHeight = el?.scrollHeight || 0
  const previousTop = el?.scrollTop || 0
  loadingOlder.value = true
  try {
    const data = await api.get(`/messages/with/${peer.value.username}`, { before: messages.value[0].id })
    const existing = new Set(messages.value.map((message) => message.id))
    const older = data.items.filter((message) => !existing.has(message.id))
    messages.value = [...older, ...messages.value]
    hasMore.value = Boolean(data.hasMore)
    await nextTick()
    if (el) el.scrollTop = previousTop + (el.scrollHeight - previousHeight)
  } catch (err) {
    toastError(err)
  } finally {
    loadingOlder.value = false
  }
}

async function loadNotifications() {
  notifLoading.value = true
  try {
    const data = await api.get('/notifications', {
      type: notifFilter.value === 'all' ? undefined : notifFilter.value,
      unread: notifUnreadOnly.value ? '1' : undefined
    })
    notifications.value = data.items || []
    notices.value = data.notices || []
    notifCounts.value = data.counts || {}
    noticeUnread.value = data.noticeUnread || 0
    notifUnread.value = (data.unread || 0) + noticeUnread.value
  } catch (err) {
    toastError(err)
  } finally {
    notifLoading.value = false
  }
}

function setNotifFilter(key) {
  if (notifFilter.value === key) return
  notifFilter.value = key
  loadNotifications()
}

async function markNotificationRead(item) {
  if (item.read) return
  item.read = true
  notifUnread.value = Math.max(0, notifUnread.value - 1)
  try {
    const data = await api.post(`/notifications/${item.id}/read`)
    if (typeof data.unread === 'number') {
      notifUnread.value = data.unread + noticeUnread.value
    }
    await refreshInbox()
  } catch {
    item.read = false
    notifUnread.value += 1
  }
}

async function markAllNotificationsRead() {
  try {
    const hasUnreadNotice = notices.value.some((notice) => !notice.read)
    await Promise.all([
      api.post('/notifications/read'),
      hasUnreadNotice ? api.post('/notices/read') : Promise.resolve()
    ])
    notifications.value = notifications.value.map((item) => ({ ...item, read: true }))
    notices.value = notices.value.map((item) => ({ ...item, read: true }))
    notifUnread.value = 0
    noticeUnread.value = 0
    await refreshInbox()
    toast('通知已全部标为已读')
  } catch (err) {
    toastError(err)
  }
}

function scrollBottom() {
  const el = threadBody.value
  if (el) el.scrollTop = el.scrollHeight
}

async function send() {
  const text = draft.value.trim()
  if (!text || !peer.value || sending.value) return
  sending.value = true
  try {
    await api.post(`/messages/with/${peer.value.username}`, { content: text })
    draft.value = ''
    await loadThread()
  } catch (err) {
    toastError(err)
  } finally {
    sending.value = false
  }
}

function insertEmoji(emoji) {
  const el = composeTextarea.value
  if (!el) {
    draft.value += emoji
    emojiOpen.value = false
    return
  }
  const start = el.selectionStart ?? draft.value.length
  const end = el.selectionEnd ?? start
  draft.value = draft.value.slice(0, start) + emoji + draft.value.slice(end)
  emojiOpen.value = false
  nextTick(() => {
    el.focus()
    const pos = start + emoji.length
    el.setSelectionRange(pos, pos)
  })
}

function onImagePick(event) {
  const file = (event.target.files || [])[0]
  event.target.value = ''
  if (file) sendImage(file)
}

async function sendImage(file) {
  if (!peer.value || uploadingImage.value) return
  if (!file.type.startsWith('image/')) {
    toast('只能发送图片文件')
    return
  }
  uploadingImage.value = true
  try {
    const image = await api.uploadImage(file)
    await api.post(`/messages/with/${peer.value.username}`, {
      type: 'image',
      imageURL: image.url,
      content: draft.value.trim()
    })
    draft.value = ''
    await loadThread()
  } catch (err) {
    toastError(err)
  } finally {
    uploadingImage.value = false
  }
}

function handleRealtimeMessage(message) {
  const me = auth.user?.id
  if (!me || (message.senderID !== me && message.receiverID !== me)) return
  if (tab.value === 'messages' && withUser.value && peer.value) {
    const inThread = message.senderID === peer.value.id || message.receiverID === peer.value.id
    if (inThread) {
      loadThread(true)
      return
    }
  }
  loadConversations()
  refreshInbox()
}

function connectWS() {
  if (socket || !getToken() || closedByUser) return
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${protocol}://${window.location.host}/api/ws?token=${encodeURIComponent(getToken())}`)
  socket = ws
  ws.onopen = () => {
    wsConnected.value = true
    reconnectDelay = 1000
  }
  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'message' && data.message) handleRealtimeMessage(data.message)
    } catch {
      /* 忽略非 JSON 消息 */
    }
  }
  ws.onclose = () => {
    socket = null
    wsConnected.value = false
    if (closedByUser) return
    window.clearTimeout(reconnectTimer)
    reconnectTimer = window.setTimeout(connectWS, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, 15000)
  }
  ws.onerror = () => ws.close()
}

function selectPeer(username) {
  userResults.value = []
  userQuery.value = ''
  router.push({ name: 'messages', query: { tab: 'messages', with: username } })
}

function switchTab(value) {
  if (value === tab.value) return
  if (value === 'notifications') router.push({ name: 'messages', query: { tab: 'notifications' } })
  else router.push({ name: 'messages', query: withUser.value ? { tab: 'messages', with: withUser.value } : { tab: 'messages' } })
}

function onSearchInput() {
  window.clearTimeout(searchTimer)
  const q = userQuery.value.trim()
  if (!q) {
    userResults.value = []
    return
  }
  searchTimer = window.setTimeout(async () => {
    searchingUsers.value = true
    try {
      const data = await api.get('/users', { q })
      userResults.value = data.items
    } catch {
      /* ignore */
    } finally {
      searchingUsers.value = false
    }
  }, 250)
}

function notifLink(item) {
  if (item.type === 'access_request') return '/admin?tab=requests'
  if (item.type === 'access_result') {
    return item.docID ? `/docs/${item.docID}` : '/messages?tab=notifications'
  }
  if (!item.docID) return `/u/${item.actor.username}`
  return `/${item.docKind === 'doc' ? 'docs' : 'articles'}/${item.docID}`
}

function notifText(item) {
  if (item.type === 'follow') return '关注了你'
  if (item.type === 'mention') return '在评论中提到了你'
  if (item.type === 'access_request') return '申请阅读你的私密文档'
  if (item.type === 'access_result') return '处理了你的私密阅读申请'
  if (item.type === 'like') return `赞了你的${item.docKind === 'doc' ? '文档' : '文章'}`
  return `评论了你的${item.docKind === 'doc' ? '文档' : '文章'}`
}

watch(
  () => [route.query.tab, route.query.with],
  () => {
    if (tab.value === 'notifications') {
      loadNotifications()
    } else {
      loadThread()
      loadConversations()
    }
  },
  { immediate: true }
)

watch(userQuery, onSearchInput)

onMounted(async () => {
  await refreshInbox()
  connectWS()
  pollTimer = window.setInterval(() => {
    if (document.visibilityState === 'hidden') return
    if (tab.value === 'messages') {
      loadConversations()
      if (withUser.value) loadThread(true)
    } else {
      loadNotifications()
    }
  }, 10000)
})

onBeforeUnmount(() => {
  closedByUser = true
  if (socket) socket.close()
  if (pollTimer) window.clearInterval(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
  if (reconnectTimer) window.clearTimeout(reconnectTimer)
})
</script>

<template>
  <div class="page messages-page">
    <header class="page-hero container">
      <span class="eyebrow">连接</span>
      <div class="page-hero-row">
        <div>
          <h1>消息</h1>
          <p class="page-lede">私信与通知。没有已读回执，也没有“正在输入”。</p>
        </div>
      </div>
    </header>

    <div class="container">
      <div class="tabs">
        <button type="button" :class="{ active: tab === 'messages' }" @click="switchTab('messages')">私信</button>
        <button type="button" :class="{ active: tab === 'notifications' }" @click="switchTab('notifications')">
          通知
          <span v-if="notifUnread" class="tab-badge">{{ notifUnread }}</span>
        </button>
      </div>

      <div
        class="messages-layout"
        :class="{ 'has-peer': peer && tab === 'messages', 'messages-layout--single': tab === 'notifications' }"
      >
        <aside v-if="tab === 'messages'" class="conv-panel">
          <div class="conv-search">
            <Icon name="search" :size="16" />
            <input v-model="userQuery" type="search" placeholder="搜索用户，发起私信" />
            <div v-if="userQuery" class="user-results">
              <p v-if="searchingUsers" class="hint">搜索中…</p>
              <template v-else>
                <button
                  v-for="user in userResults"
                  :key="user.id"
                  type="button"
                  class="user-result"
                  @click="selectPeer(user.username)"
                >
                  <UserAvatar :username="user.username" :nickname="user.nickname" :size="28" />
                  <span>
                    <strong>{{ user.nickname }}</strong>
                    <span>@{{ user.username }}</span>
                  </span>
                </button>
                <p v-if="!userResults.length" class="hint">没有找到这位读者。</p>
              </template>
            </div>
          </div>

          <div class="conv-list">
            <p v-if="loadingConv" class="muted hint conv-loading">正在加载会话…</p>
            <p v-else-if="!conversations.length" class="muted hint conv-loading">还没有会话。搜索一位读者，发第一条消息吧。</p>
            <button
              v-for="item in conversations"
              :key="item.user.id"
              type="button"
              class="conv-item"
              :class="{ active: item.user.username === withUser }"
              @click="selectPeer(item.user.username)"
            >
              <UserAvatar :username="item.user.username" :nickname="item.user.nickname" :size="42" />
              <span class="conv-main">
                <span class="conv-top">
                  <strong>{{ item.user.nickname }}</strong>
                  <time>{{ timeAgo(item.lastAt) }}</time>
                </span>
                <span class="conv-bottom">
                  <span class="conv-last">{{ item.lastMessage }}</span>
                  <span v-if="item.unread" class="conv-unread">{{ item.unread }}</span>
                </span>
              </span>
            </button>
          </div>
        </aside>

        <section class="thread-panel">
          <template v-if="tab === 'messages'">
            <template v-if="peer">
              <header class="thread-head">
                <button
                  class="icon-btn thread-back"
                  type="button"
                  aria-label="返回会话列表"
                  @click="router.push({ name: 'messages', query: { tab: 'messages' } })"
                >
                  <Icon name="arrowLeft" :size="18" />
                </button>
                <RouterLink :to="`/u/${peer.username}`" class="thread-peer">
                  <UserAvatar :username="peer.username" :nickname="peer.nickname" :size="38" />
                  <span>
                    <strong>{{ peer.nickname }}</strong>
                    <span>@{{ peer.username }}</span>
                  </span>
                </RouterLink>
              </header>

              <div ref="threadBody" class="thread-body">
                <div v-if="hasMore && !loadingThread" class="load-older">
                  <button class="btn btn-ghost btn-sm" type="button" :disabled="loadingOlder" @click="loadOlder">
                    {{ loadingOlder ? '加载中…' : '加载更早的消息' }}
                  </button>
                </div>
                <p v-if="loadingThread" class="muted loading-line">正在取回对话…</p>
                <p v-else-if="!messages.length" class="empty-inline">还没有消息，先说声你好吧。</p>
                <div
                  v-for="message in messages"
                  :key="message.id"
                  class="message"
                  :class="{ mine: message.senderID === auth.user?.id }"
                >
                  <div v-if="message.type === 'image'" class="message-image">
                    <a :href="message.imageURL" target="_blank" rel="noopener">
                      <img :src="message.imageURL" :alt="message.content || '图片消息'" loading="lazy" />
                    </a>
                  </div>
                  <div v-if="message.content" class="message-bubble">{{ message.content }}</div>
                  <span class="message-time">{{ timeAgo(message.createdAt) }}</span>
                </div>
              </div>

              <form class="thread-compose" @submit.prevent="send">
                <div class="thread-compose-tools">
                  <button class="compose-tool" type="button" title="表情" @click="emojiOpen = !emojiOpen">😊</button>
                  <button class="compose-tool" type="button" title="发送图片" :disabled="uploadingImage" @click="imageInput?.click()">
                    <Icon name="image" :size="16" />
                    {{ uploadingImage ? '上传中…' : '图片' }}
                  </button>
                  <span class="compose-status" :class="{ online: wsConnected }">
                    {{ wsConnected ? '实时连接' : '每 10 秒同步' }}
                  </span>
                  <div v-if="emojiOpen" class="emoji-panel">
                    <button v-for="emoji in emojis" :key="emoji" type="button" @click="insertEmoji(emoji)">{{ emoji }}</button>
                  </div>
                </div>
                <textarea
                  ref="composeTextarea"
                  v-model="draft"
                  rows="2"
                  maxlength="1000"
                  placeholder="写一条私信…（Enter 发送，Shift + Enter 换行）"
                  @keydown.enter.exact.prevent="send"
                ></textarea>
                <div class="thread-compose-foot">
                  <span class="hint">{{ draft.length }} / 1000 · 支持图片和表情</span>
                  <button class="btn btn-primary btn-sm" type="submit" :disabled="sending || uploadingImage">
                    <Icon name="send" :size="15" />
                    {{ sending ? '发送中…' : '发送' }}
                  </button>
                </div>
                <input ref="imageInput" class="hidden-file" type="file" accept="image/jpeg,image/png,image/gif,image/webp" @change="onImagePick" />
              </form>
            </template>
            <EmptyState v-else icon="message" title="选择一段对话" description="从左侧选择会话，或搜索用户开始新的私信。">
              <RouterLink to="/u/weijia" class="btn btn-ghost btn-sm">去主页逛逛</RouterLink>
            </EmptyState>
          </template>

          <template v-else>
            <header class="thread-head thread-head--simple">
              <strong>通知</strong>
              <button
                v-if="notifUnread > 0"
                class="text-link"
                type="button"
                @click="markAllNotificationsRead"
              >
                全部标为已读
              </button>
            </header>
            <div class="notif-filters">
              <div class="notif-filter-group">
                <button
                  v-for="filter in notifFilters"
                  :key="filter.key"
                  type="button"
                  class="notif-filter"
                  :class="{ active: notifFilter === filter.key }"
                  @click="setNotifFilter(filter.key)"
                >
                  {{ filter.label }}
                  <span v-if="notifCounts[filter.key]" class="notif-filter-count">{{ notifCounts[filter.key] }}</span>
                </button>
              </div>
              <label class="notif-unread-toggle">
                <input v-model="notifUnreadOnly" type="checkbox" @change="loadNotifications" />
                只看未读
              </label>
            </div>
            <div class="notice-list">
              <article v-for="notice in notices" :key="notice.id" class="notice-item" :class="{ unread: !notice.read }">
                <header class="notice-head">
                  <span class="notice-tag">全站通知</span>
                  <strong>{{ notice.title }}</strong>
                  <time>{{ shortDate(notice.createdAt) }}</time>
                </header>
                <p>{{ notice.content }}</p>
                <div class="notice-foot">
                  <span>由 {{ notice.createdByName }} 发布</span>
                  <RouterLink v-if="notice.link?.startsWith('/')" :to="notice.link" class="text-link">查看详情</RouterLink>
                  <a v-else-if="notice.link" :href="notice.link" target="_blank" rel="noopener" class="text-link">查看详情</a>
                </div>
              </article>
            </div>

            <div class="notif-list">
              <p v-if="notifLoading" class="muted loading-line">正在加载通知…</p>
              <p v-else-if="!notifications.length && !notices.length" class="empty-inline">
                {{ notifUnreadOnly || notifFilter !== 'all' ? '没有符合条件的通知。' : '还没有新的通知。' }}
              </p>
              <RouterLink
                v-for="item in notifications"
                :key="item.id"
                :to="notifLink(item)"
                class="notif-item"
                :class="{ unread: !item.read }"
                @click="markNotificationRead(item)"
              >
                <UserAvatar :username="item.actor.username" :nickname="item.actor.nickname" :size="38" />
                <span class="notif-main">
                  <span class="notif-line">
                    <strong>{{ item.actor.nickname }}</strong>
                    {{ notifText(item) }}
                    <em v-if="item.docTitle">《{{ item.docTitle }}》</em>
                  </span>
                  <span v-if="item.content" class="notif-excerpt">{{ item.content }}</span>
                  <span class="notif-time">{{ shortDate(item.createdAt) }}</span>
                </span>
                <Icon :name="item.type.startsWith('access') ? 'lock' : item.type === 'follow' ? 'follow' : item.type === 'mention' ? 'message' : item.type === 'like' ? 'heart' : 'comment'" :size="16" class="notif-icon" />
              </RouterLink>
            </div>
          </template>
        </section>
      </div>
    </div>
  </div>
</template>
