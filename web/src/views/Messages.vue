<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth, refreshInbox } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { timeAgo, shortDate } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

const tab = computed(() => (route.query.tab === 'notifications' ? 'notifications' : 'messages'))
const withUser = computed(() => String(route.query.with || ''))

const conversations = ref([])
const notifications = ref([])
const notifUnread = ref(0)
const messages = ref([])
const peer = ref(null)
const draft = ref('')
const sending = ref(false)
const loadingConv = ref(true)
const loadingThread = ref(false)
const threadBody = ref(null)
const userQuery = ref('')
const userResults = ref([])
const searchingUsers = ref(false)

let pollTimer = null
let searchTimer = null

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
    messages.value = data.items
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
  } finally {
    loadingThread.value = false
  }
}

async function loadNotifications(markRead = false) {
  try {
    const data = await api.get('/notifications')
    notifications.value = data.items
    notifUnread.value = data.unread
    if (markRead && data.unread > 0) {
      await api.post('/notifications/read')
      notifications.value = data.items.map((item) => ({ ...item, read: true }))
      notifUnread.value = 0
      await refreshInbox()
    }
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
  if (!item.docID) return `/u/${item.actor.username}`
  return `/${item.docKind === 'doc' ? 'docs' : 'articles'}/${item.docID}`
}

function notifText(item) {
  if (item.type === 'follow') return '关注了你'
  if (item.type === 'like') return `赞了你的${item.docKind === 'doc' ? '文档' : '文章'}`
  return `评论了你的${item.docKind === 'doc' ? '文档' : '文章'}`
}

watch(
  () => [route.query.tab, route.query.with],
  () => {
    if (tab.value === 'notifications') {
      loadNotifications(true)
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
  pollTimer = window.setInterval(() => {
    if (document.visibilityState === 'hidden') return
    if (tab.value === 'messages') {
      loadConversations()
      if (withUser.value) loadThread(true)
    } else {
      loadNotifications(false)
    }
  }, 10000)
})

onBeforeUnmount(() => {
  if (pollTimer) window.clearInterval(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
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
                <p v-if="loadingThread" class="muted loading-line">正在取回对话…</p>
                <p v-else-if="!messages.length" class="empty-inline">还没有消息，先说声你好吧。</p>
                <div
                  v-for="message in messages"
                  :key="message.id"
                  class="message"
                  :class="{ mine: message.senderID === auth.user?.id }"
                >
                  <div class="message-bubble">{{ message.content }}</div>
                  <span class="message-time">{{ timeAgo(message.createdAt) }}</span>
                </div>
              </div>

              <form class="thread-compose" @submit.prevent="send">
                <textarea
                  v-model="draft"
                  rows="2"
                  maxlength="1000"
                  placeholder="写一条私信…（Enter 发送，Shift + Enter 换行）"
                  @keydown.enter.exact.prevent="send"
                ></textarea>
                <div class="thread-compose-foot">
                  <span class="hint">{{ draft.length }} / 1000</span>
                  <button class="btn btn-primary btn-sm" type="submit" :disabled="sending">
                    <Icon name="send" :size="15" />
                    {{ sending ? '发送中…' : '发送' }}
                  </button>
                </div>
              </form>
            </template>
            <EmptyState v-else icon="message" title="选择一段对话" description="从左侧选择会话，或搜索用户开始新的私信。">
              <RouterLink to="/u/weijia" class="btn btn-ghost btn-sm">去主页逛逛</RouterLink>
            </EmptyState>
          </template>

          <template v-else>
            <header class="thread-head thread-head--simple">
              <strong>通知</strong>
              <button v-if="notifications.some((item) => !item.read)" class="text-link" type="button" @click="loadNotifications(true)">
                全部标为已读
              </button>
            </header>
            <div class="notif-list">
              <p v-if="!notifications.length" class="empty-inline">还没有新的通知。</p>
              <RouterLink
                v-for="item in notifications"
                :key="item.id"
                :to="notifLink(item)"
                class="notif-item"
                :class="{ unread: !item.read }"
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
                <Icon :name="item.type === 'follow' ? 'follow' : item.type === 'like' ? 'heart' : 'comment'" :size="16" class="notif-icon" />
              </RouterLink>
            </div>
          </template>
        </section>
      </div>
    </div>
  </div>
</template>
