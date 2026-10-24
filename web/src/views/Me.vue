<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth, refreshMe } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { roleLabel, shortDate, timeAgo } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import DocCard from '../components/DocCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

const tab = ref(String(route.query.tab || 'profile'))
const loading = ref(true)
const tabLoading = ref(false)
const myDocs = ref([])
const favorites = ref([])
const history = ref([])
const savingProfile = ref(false)
const savingPassword = ref(false)

const profileForm = reactive({ nickname: '', bio: '', email: '' })
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirm: '' })

const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

const tabs = computed(() => {
  const list = [
    { key: 'profile', label: '资料设置', icon: 'user' },
    { key: 'security', label: '账号安全', icon: 'settings' }
  ]
  if (staff.value) {
    list.push(
      { key: 'articles', label: '我的文章', icon: 'edit', count: auth.user?.counts?.articles ?? 0 },
      { key: 'docs', label: '我的文档', icon: 'book', count: auth.user?.counts?.docs ?? 0 }
    )
  }
  list.push({ key: 'favorites', label: '我的收藏', icon: 'bookmark', count: auth.user?.counts?.favorites ?? 0 })
  list.push({ key: 'history', label: '浏览历史', icon: 'clock' })
  return list
})

function normalizeTab() {
  if (!tabs.value.some((item) => item.key === tab.value)) {
    tab.value = 'profile'
  }
}

async function loadTab() {
  if (!auth.user) return
  normalizeTab()
  tabLoading.value = true
  try {
    if (tab.value === 'articles' || tab.value === 'docs') {
      if (!staff.value) return
      const data = await api.get('/me/docs', {
        kind: tab.value === 'articles' ? 'article' : 'doc',
        pageSize: 50
      })
      myDocs.value = data.items
    } else if (tab.value === 'favorites') {
      const data = await api.get('/me/favorites', { pageSize: 50 })
      favorites.value = data.items
    } else if (tab.value === 'history') {
      const data = await api.get('/me/history', { pageSize: 50 })
      history.value = data.items
    }
  } catch (err) {
    toastError(err)
  } finally {
    tabLoading.value = false
  }
}

function fillProfile() {
  if (!auth.user) return
  profileForm.nickname = auth.user.nickname || ''
  profileForm.bio = auth.user.bio || ''
  profileForm.email = auth.user.email || ''
}

function setTab(key) {
  if (tab.value === key) return
  tab.value = key
  router.replace({ path: '/me', query: { tab: key } })
  loadTab()
}

async function saveProfile() {
  savingProfile.value = true
  try {
    await api.put('/me', {
      nickname: profileForm.nickname,
      bio: profileForm.bio,
      email: profileForm.email
    })
    await refreshMe()
    fillProfile()
    toast('资料已保存')
  } catch (err) {
    toastError(err)
  } finally {
    savingProfile.value = false
  }
}

async function changePassword() {
  if (!passwordForm.oldPassword || !passwordForm.newPassword) {
    toast('请填写原密码和新密码')
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirm) {
    toast('两次输入的新密码不一致')
    return
  }
  if (passwordForm.newPassword.length < 6) {
    toast('新密码至少 6 位')
    return
  }
  savingPassword.value = true
  try {
    await api.put('/me/password', {
      oldPassword: passwordForm.oldPassword,
      newPassword: passwordForm.newPassword
    })
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirm = ''
    toast('密码已更新')
  } catch (err) {
    toastError(err)
  } finally {
    savingPassword.value = false
  }
}

async function removeDoc(doc) {
  if (!window.confirm(`删除「${doc.title}」？文档的子文档和所有互动也会一并删除。`)) return
  try {
    await api.del(`/docs/${doc.id}`)
    await loadTab()
    await refreshMe()
    toast('已删除')
  } catch (err) {
    toastError(err)
  }
}

async function clearHistory() {
  if (!window.confirm('清空全部浏览历史？')) return
  try {
    await api.del('/me/history')
    history.value = []
    toast('浏览历史已清空')
  } catch (err) {
    toastError(err)
  }
}

function docHref(doc) {
  return `/${doc.kind === 'doc' ? 'docs' : 'articles'}/${doc.id}`
}

watch(
  () => route.query.tab,
  (value) => {
    const next = String(value || 'profile')
    if (!tabs.value.some((item) => item.key === next)) {
      tab.value = 'profile'
      return
    }
    if (next !== tab.value) {
      tab.value = next
      loadTab()
    }
  }
)

onMounted(async () => {
  await refreshMe()
  if (!auth.user) {
    router.replace({ path: '/login', query: { redirect: '/me' } })
    return
  }
  normalizeTab()
  fillProfile()
  await loadTab()
  loading.value = false
})
</script>

<template>
  <div class="page me-page">
    <header class="page-hero container">
      <span class="eyebrow">个人</span>
      <h1>我的空间</h1>
      <p class="page-lede">{{ staff ? '管理资料、内容与收藏。' : '管理资料与收藏。' }}这里只属于你。</p>
    </header>

    <div v-if="loading" class="container page-loading">
      <p class="muted loading-line">正在整理你的空间…</p>
    </div>

    <div v-else-if="auth.user" class="container me-layout">
      <aside class="me-side">
        <div class="me-card">
          <UserAvatar :username="auth.user.username" :nickname="auth.user.nickname" :size="72" />
          <h2>{{ auth.user.nickname }}</h2>
          <span class="me-handle">@{{ auth.user.username }}</span>
          <span v-if="staff" class="badge badge-admin">{{ roleLabel(auth.user.role) }}</span>
          <p>{{ auth.user.bio || '还没有写简介。' }}</p>
          <RouterLink :to="`/u/${auth.user.username}`" class="text-link">查看公开主页</RouterLink>
          <RouterLink v-if="staff" to="/admin" class="btn btn-ghost btn-sm admin-entry">
            <Icon name="settings" :size="14" /> 进入管理后台
          </RouterLink>
        </div>

        <nav class="me-nav">
          <button
            v-for="item in tabs"
            :key="item.key"
            type="button"
            :class="{ active: tab === item.key }"
            @click="setTab(item.key)"
          >
            <Icon :name="item.icon" :size="16" />
            <span>{{ item.label }}</span>
            <span v-if="item.count" class="me-nav-count">{{ item.count }}</span>
          </button>
        </nav>
      </aside>

      <section class="me-main">
        <div v-if="tab === 'profile'" class="panel">
          <header class="panel-head">
            <h2>资料设置</h2>
            <p>头像会根据用户名自动生成，所以只需要维护文字资料。</p>
          </header>
          <div class="profile-setting">
            <div class="profile-setting-avatar">
              <UserAvatar :username="auth.user.username" :nickname="profileForm.nickname" :size="88" />
              <span>自动生成的头像</span>
            </div>
            <form class="profile-form" @submit.prevent="saveProfile">
              <div class="form-row">
                <div class="field">
                  <label>用户名</label>
                  <input :value="auth.user.username" type="text" disabled />
                  <span class="hint">用户名不可修改</span>
                </div>
                <div class="field">
                  <label for="me-nickname">昵称</label>
                  <input id="me-nickname" v-model="profileForm.nickname" type="text" maxlength="24" />
                </div>
              </div>
              <div class="field">
                <label for="me-email">邮箱</label>
                <input id="me-email" v-model="profileForm.email" type="email" placeholder="you@example.com" />
              </div>
              <div class="field">
                <label for="me-bio">简介</label>
                <textarea id="me-bio" v-model="profileForm.bio" rows="3" maxlength="120" placeholder="一句话介绍自己…"></textarea>
                <span class="hint">{{ profileForm.bio.length }} / 120</span>
              </div>
              <div class="form-actions">
                <button class="btn btn-primary btn-sm" type="submit" :disabled="savingProfile">
                  {{ savingProfile ? '保存中…' : '保存资料' }}
                </button>
              </div>
            </form>
          </div>
        </div>

        <div v-else-if="tab === 'security'" class="panel">
          <header class="panel-head">
            <h2>账号安全</h2>
            <p>修改密码后，当前登录状态会保留。</p>
          </header>
          <form class="security-form" @submit.prevent="changePassword">
            <div class="field">
              <label for="pwd-old">原密码</label>
              <input id="pwd-old" v-model="passwordForm.oldPassword" type="password" autocomplete="current-password" />
            </div>
            <div class="form-row">
              <div class="field">
                <label for="pwd-new">新密码</label>
                <input id="pwd-new" v-model="passwordForm.newPassword" type="password" autocomplete="new-password" />
              </div>
              <div class="field">
                <label for="pwd-confirm">确认新密码</label>
                <input id="pwd-confirm" v-model="passwordForm.confirm" type="password" autocomplete="new-password" />
              </div>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="savingPassword">
                {{ savingPassword ? '更新中…' : '更新密码' }}
              </button>
            </div>
          </form>
        </div>

        <div v-else-if="tab === 'articles' || tab === 'docs'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>{{ tab === 'articles' ? '我的文章' : '我的文档' }}</h2>
              <p>草稿只有你自己可见，发布后会出现在公开列表。</p>
            </div>
            <RouterLink :to="tab === 'docs' ? '/write?kind=doc' : '/write'" class="btn btn-primary btn-sm">
              <Icon name="plus" :size="15" />
              新建{{ tab === 'articles' ? '文章' : '文档' }}
            </RouterLink>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!myDocs.length"
            icon="edit"
            :title="tab === 'articles' ? '还没有文章' : '还没有文档'"
            description="写下第一篇，它会出现在这里。"
          >
            <RouterLink :to="tab === 'docs' ? '/write?kind=doc' : '/write'" class="btn btn-ghost btn-sm">开始写作</RouterLink>
          </EmptyState>

          <div v-else class="content-list">
            <article v-for="doc in myDocs" :key="doc.id" class="content-row">
              <div class="content-row-main">
                <RouterLink :to="docHref(doc)" class="content-row-title">{{ doc.title }}</RouterLink>
                <div class="content-row-meta">
                  <span class="badge" :class="doc.status === 'draft' ? 'badge-draft' : 'badge-live'">
                    {{ doc.status === 'draft' ? '草稿' : '已发布' }}
                  </span>
                  <span>{{ shortDate(doc.updatedAt) }} 更新</span>
                  <span>{{ doc.viewCount }} 阅读</span>
                  <span>{{ doc.likeCount }} 赞</span>
                  <span v-if="doc.commentCount">{{ doc.commentCount }} 评论</span>
                </div>
              </div>
              <div class="content-row-actions">
                <RouterLink :to="`/write/${doc.id}`" class="btn btn-ghost btn-sm">编辑</RouterLink>
                <button class="icon-btn danger" type="button" aria-label="删除" @click="removeDoc(doc)">
                  <Icon name="trash" :size="17" />
                </button>
              </div>
            </article>
          </div>
        </div>

        <div v-else-if="tab === 'favorites'" class="panel">
          <header class="panel-head">
            <h2>我的收藏</h2>
            <p>收藏是私密的，只有你能看到这份列表。</p>
          </header>
          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!favorites.length"
            icon="bookmark"
            title="还没有收藏"
            description="在文章或文档底部点一下收藏，它就会出现在这里。"
          >
            <RouterLink to="/articles" class="btn btn-ghost btn-sm">去读点什么</RouterLink>
          </EmptyState>
          <div v-else class="doc-grid">
            <DocCard v-for="doc in favorites" :key="doc.id" :doc="doc" />
          </div>
        </div>

        <div v-else-if="tab === 'history'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>浏览历史</h2>
              <p>最近打开过的文章与文档，按时间倒序排列。</p>
            </div>
            <button v-if="history.length" class="btn btn-ghost btn-sm" type="button" @click="clearHistory">
              清空历史
            </button>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!history.length"
            icon="clock"
            title="还没有浏览记录"
            description="打开一篇文章或文档后，它会出现在这里。"
          >
            <RouterLink to="/articles" class="btn btn-ghost btn-sm">去读点什么</RouterLink>
          </EmptyState>
          <div v-else class="content-list">
            <article v-for="item in history" :key="item.doc.id" class="content-row">
              <div class="content-row-main">
                <RouterLink :to="docHref(item.doc)" class="content-row-title">{{ item.doc.title }}</RouterLink>
                <div class="content-row-meta">
                  <span>{{ item.doc.kind === 'doc' ? '文档' : '文章' }}</span>
                  <span v-if="item.doc.category">{{ item.doc.category.name }}</span>
                  <span>{{ item.doc.author?.nickname }}</span>
                  <span>{{ timeAgo(item.viewedAt) }}浏览</span>
                </div>
              </div>
              <div class="content-row-actions">
                <RouterLink :to="docHref(item.doc)" class="btn btn-ghost btn-sm">重新阅读</RouterLink>
              </div>
            </article>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
