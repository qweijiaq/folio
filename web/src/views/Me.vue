<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { confirmDialog } from '../stores/confirm'
import { auth, logoutLocal, refreshMe } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { roleLabel, shortDate, timeAgo } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import EmailCode from '../components/EmailCode.vue'
import PasswordInput from '../components/PasswordInput.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import SelectBox from '../components/SelectBox.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '我的空间', description: '管理知微账号资料', noindex: true }))

const tab = ref(String(route.query.tab || 'profile'))
const loading = ref(true)
const tabLoading = ref(false)
const myDocs = ref([])
const favorites = ref([])
const folders = ref([])
const selectedFolderID = ref(null)
const newFolderName = ref('')
const editingFolderID = ref(0)
const folderNameDraft = ref('')
const history = ref([])
const outgoingRequests = ref([])
const cancellingRequest = ref(0)
const loginRecords = ref([])
const sessions = ref([])
const sessionsLoading = ref(false)
const sessionBusy = ref(0)
const featuredDocID = ref(null)
const savingProfile = ref(false)
const savingPassword = ref(false)
const savingEmail = ref(false)
const savingPrivacy = ref(false)
const emailEnabled = ref(false)
const privacySettings = reactive({
  openFollow: true,
  openFans: true,
  openPrivateChat: true,
  openCommentMessage: true,
  openDiggMessage: true,
  openFollowMessage: true,
  openAccessMessage: true,
  emailNotify: false,
  openCollect: false,
  homeStyle: 'cover'
})

const profileForm = reactive({ nickname: '', bio: '' })
const emailForm = reactive({ email: '', code: '' })
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirm: '' })

const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))
const otherSessions = computed(() => sessions.value.filter((session) => !session.current).length)
const homeStyleOptions = [
  { value: 'cover', label: '封面：保留顶部自动封面' },
  { value: 'compact', label: '紧凑：缩小头部高度' },
  { value: 'plain', label: '纯文字：不显示封面' }
]
const folderSelectOptions = computed(() => folders.value.map((folder) => ({ value: folder.id, label: folder.name })))
const currentFolder = computed(
  () => folders.value.find((folder) => String(folder.id) === String(selectedFolderID.value)) || null
)

const tabs = computed(() => {
  const list = [
    { key: 'profile', label: '资料设置', icon: 'user' },
    { key: 'security', label: '账号安全', icon: 'settings' },
    { key: 'privacy', label: '隐私与消息', icon: 'eye' }
  ]
  if (staff.value) {
    list.push(
      { key: 'articles', label: '我的文章', icon: 'edit', count: auth.user?.counts?.articles ?? 0 },
      { key: 'docs', label: '我的文档', icon: 'book', count: auth.user?.counts?.docs ?? 0 }
    )
  }
  list.push({ key: 'favorites', label: '我的收藏', icon: 'bookmark', count: auth.user?.counts?.favorites ?? 0 })
  list.push({ key: 'history', label: '浏览历史', icon: 'clock' })
  list.push({ key: 'requests', label: '阅读申请', icon: 'lock' })
  return list
})

function normalizeTab() {
  if (!tabs.value.some((item) => item.key === tab.value)) {
    tab.value = 'profile'
  }
}

async function loadFavorites() {
  const folderData = await api.get('/favorite-folders')
  folders.value = folderData.items
  if (!folders.value.length) {
    favorites.value = []
    return
  }
  if (!folders.value.some((folder) => String(folder.id) === String(selectedFolderID.value))) {
    const preferred = folders.value.find((folder) => folder.isDefault) || folders.value[0]
    selectedFolderID.value = preferred.id
  }
  const data = await api.get('/me/favorites', { folderID: selectedFolderID.value, pageSize: 50 })
  favorites.value = data.items
}

async function selectFolder(folder) {
  if (String(folder.id) === String(selectedFolderID.value)) return
  selectedFolderID.value = folder.id
  editingFolderID.value = 0
  try {
    const data = await api.get('/me/favorites', { folderID: folder.id, pageSize: 50 })
    favorites.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function createFolder() {
  const name = newFolderName.value.trim()
  if (!name) {
    toast('请输入收藏夹名称')
    return
  }
  try {
    const data = await api.post('/favorite-folders', { name })
    newFolderName.value = ''
    selectedFolderID.value = data.folder.id
    await loadFavorites()
    toast('收藏夹已创建')
  } catch (err) {
    toastError(err)
  }
}

function startRenameFolder(folder) {
  editingFolderID.value = folder.id
  folderNameDraft.value = folder.name
}

function cancelRenameFolder() {
  editingFolderID.value = 0
  folderNameDraft.value = ''
}

async function saveFolderName(folder) {
  const name = folderNameDraft.value.trim()
  if (!name) {
    toast('收藏夹名称不能为空')
    return
  }
  try {
    await api.put(`/favorite-folders/${folder.id}`, { name })
    cancelRenameFolder()
    await loadFavorites()
    toast('收藏夹已重命名')
  } catch (err) {
    toastError(err)
  }
}

async function removeFolder(folder) {
  if (!(await confirmDialog({ title: '删除收藏夹', message: `删除收藏夹「${folder.name}」？里面的收藏会移到默认收藏夹。`, confirmText: '删除', danger: true }))) return
  try {
    await api.del(`/favorite-folders/${folder.id}`)
    selectedFolderID.value = null
    await loadFavorites()
    toast('收藏夹已删除')
  } catch (err) {
    toastError(err)
  }
}

async function moveFavorite(doc, folderValue) {
  const folderID = Number(folderValue)
  if (!folderID) return
  try {
    await api.post(`/docs/${doc.id}/favorite`, { folderID })
    await loadFavorites()
    toast('已移动收藏')
  } catch (err) {
    toastError(err)
  }
}

async function removeFavorite(doc) {
  if (!(await confirmDialog({ title: '取消收藏', message: `取消收藏「${doc.title}」？`, confirmText: '取消收藏' }))) return
  try {
    await api.post(`/docs/${doc.id}/favorite`)
    await loadFavorites()
    toast('已取消收藏')
  } catch (err) {
    toastError(err)
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
      if (tab.value === 'articles') {
        const settings = await api.get('/me/settings')
        featuredDocID.value = settings.settings?.featuredDocID || null
      }
    } else if (tab.value === 'favorites') {
      await loadFavorites()
    } else if (tab.value === 'history') {
      const data = await api.get('/me/history', { pageSize: 50 })
      history.value = data.items
    } else if (tab.value === 'requests') {
      const data = await api.get('/me/access-requests/outgoing')
      outgoingRequests.value = data.items || []
    } else if (tab.value === 'privacy') {
      await loadPrivacySettings()
    } else if (tab.value === 'security') {
      await Promise.all([loadLoginRecords(), loadSessions()])
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
}

function accessStatusLabel(status) {
  return { pending: '待处理', approved: '已通过', denied: '已拒绝', cancelled: '已撤回' }[status] || status
}

async function cancelOutgoingRequest(item) {
  if (!(await confirmDialog({ title: '撤回阅读申请', message: '撤回这条阅读申请？', confirmText: '撤回' }))) return
  cancellingRequest.value = item.id
  try {
    await api.del(`/access-requests/${item.id}`)
    await loadTab()
    toast('已撤回阅读申请')
  } catch (err) {
    toastError(err)
  } finally {
    cancellingRequest.value = 0
  }
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
      bio: profileForm.bio
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

async function loadLoginRecords() {
  try {
    const data = await api.get('/me/login-records')
    loginRecords.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function loadSessions() {
  sessionsLoading.value = true
  try {
    const data = await api.get('/me/sessions')
    sessions.value = data.items || []
  } catch (err) {
    toastError(err)
  } finally {
    sessionsLoading.value = false
  }
}

async function revokeSession(session) {
  const label = session.device || '该设备'
  const question = session.current ? '退出当前设备后需要重新登录，继续？' : `退出「${label}」这个登录会话？`
  if (!(await confirmDialog({ title: '退出登录设备', message: question, confirmText: '退出', danger: true }))) return
  sessionBusy.value = session.id
  try {
    const data = await api.del(`/me/sessions/${session.id}`)
    if (data?.current) {
      logoutLocal()
      toast('已退出当前设备')
      router.push('/login')
      return
    }
    await loadSessions()
    toast('已退出该设备')
  } catch (err) {
    toastError(err)
  } finally {
    sessionBusy.value = 0
  }
}

async function revokeOtherSessions() {
  if (!otherSessions.value) return
  if (!(await confirmDialog({ title: '退出其他设备', message: `退出其他 ${otherSessions.value} 个登录会话？`, confirmText: '退出', danger: true }))) return
  try {
    const data = await api.post('/me/sessions/revoke-others')
    await loadSessions()
    toast(`已退出 ${data?.revoked || 0} 个设备`)
  } catch (err) {
    toastError(err)
  }
}

async function revokeAllSessions() {
  if (!(await confirmDialog({ title: '退出所有设备', message: '退出所有设备后需要重新登录，确定继续？', confirmText: '退出所有设备', danger: true }))) return
  try {
    await api.post('/me/sessions/revoke-all')
    logoutLocal()
    toast('已退出所有设备')
    router.push('/login')
  } catch (err) {
    toastError(err)
  }
}

async function loadPrivacySettings() {
  try {
    const data = await api.get('/me/settings')
    Object.assign(privacySettings, data.settings)
  } catch (err) {
    toastError(err)
  }
}

async function savePrivacySettings() {
  savingPrivacy.value = true
  try {
    const data = await api.put('/me/settings', { ...privacySettings })
    Object.assign(privacySettings, data.settings)
    toast('隐私与消息设置已保存')
  } catch (err) {
    toastError(err)
  } finally {
    savingPrivacy.value = false
  }
}

async function bindEmail() {
  const email = emailForm.email.trim()
  if (!email) {
    toast('请填写要绑定的邮箱')
    return
  }
  if (!emailForm.code.trim()) {
    toast('请填写邮箱验证码')
    return
  }
  savingEmail.value = true
  try {
    await api.put('/me/email', { email, code: emailForm.code.trim() })
    await refreshMe()
    emailForm.email = ''
    emailForm.code = ''
    toast('邮箱已绑定')
  } catch (err) {
    toastError(err)
  } finally {
    savingEmail.value = false
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

async function toggleFeatured(doc) {
  const next = featuredDocID.value === doc.id ? 0 : doc.id
  try {
    await api.put('/me/featured-doc', { docID: next })
    featuredDocID.value = next || null
    toast(next ? '已置顶到公开主页' : '已取消置顶')
  } catch (err) {
    toastError(err)
  }
}

async function removeDoc(doc) {
  if (!(await confirmDialog({ title: '删除内容', message: `删除「${doc.title}」？文档的子文档和所有互动也会一并删除。`, confirmText: '删除', danger: true }))) return
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
  if (!(await confirmDialog({ title: '清空浏览历史', message: '清空全部浏览历史？', confirmText: '清空', danger: true }))) return
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
  api.get('/auth/email/status').then((data) => {
    emailEnabled.value = Boolean(data.enabled)
  }).catch(() => {
    emailEnabled.value = false
  })
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
            <p>修改密码后当前设备保持登录，其他设备会退出。发现异常登录时可以立即撤销会话。</p>
          </header>
          <form class="security-form" @submit.prevent="changePassword">
            <div class="field">
              <label for="pwd-old">原密码</label>
              <PasswordInput id="pwd-old" v-model="passwordForm.oldPassword" autocomplete="current-password" />
            </div>
            <div class="form-row">
              <div class="field">
                <label for="pwd-new">新密码</label>
                <PasswordInput id="pwd-new" v-model="passwordForm.newPassword" autocomplete="new-password" />
              </div>
              <div class="field">
                <label for="pwd-confirm">确认新密码</label>
                <PasswordInput id="pwd-confirm" v-model="passwordForm.confirm" autocomplete="new-password" />
              </div>
            </div>
            <div class="form-actions">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="savingPassword">
                {{ savingPassword ? '更新中…' : '更新密码' }}
              </button>
            </div>
          </form>

          <section class="email-bind">
            <header>
              <h3>邮箱</h3>
              <p>绑定邮箱后可以用验证码登录，也可以自助找回密码。更换邮箱同样需要验证。</p>
            </header>
            <p v-if="auth.user.email" class="email-current">
              当前邮箱：<strong>{{ auth.user.email }}</strong>
            </p>
            <p v-else class="email-current muted">还没有绑定邮箱。</p>
            <template v-if="emailEnabled">
              <div class="form-row email-bind-row">
                <div class="field">
                  <label for="me-email">新邮箱</label>
                  <input id="me-email" v-model="emailForm.email" type="email" autocomplete="email" placeholder="you@example.com" />
                </div>
                <div class="field">
                  <label>验证码</label>
                  <EmailCode v-model="emailForm.code" :email="emailForm.email" purpose="bind" />
                </div>
              </div>
              <div class="form-actions">
                <button class="btn btn-primary btn-sm" type="button" :disabled="savingEmail" @click="bindEmail">
                  {{ savingEmail ? '绑定中…' : auth.user.email ? '更换邮箱' : '绑定邮箱' }}
                </button>
              </div>
            </template>
            <p v-else class="hint">站点未配置邮件服务，暂时不能绑定或验证邮箱。</p>
          </section>

          <section class="session-manager">
            <header>
              <h3>登录设备</h3>
              <p>每次登录都会生成一个会话。撤销后该设备的登录 token 立即失效。</p>
            </header>
            <p v-if="sessionsLoading" class="muted loading-line">正在加载登录设备…</p>
            <p v-else-if="!sessions.length" class="muted">还没有可管理的登录会话。</p>
            <ul v-else class="session-list">
              <li v-for="session in sessions" :key="session.id" class="session-row">
                <div class="session-main">
                  <div class="session-device">
                    <strong>{{ session.device || '未知设备' }}</strong>
                    <span v-if="session.current" class="badge badge-live">当前设备</span>
                  </div>
                  <span class="session-meta">{{ session.ip || '未知 IP' }} · 最近活跃 {{ timeAgo(session.lastSeenAt) }}</span>
                  <span class="session-ua">{{ session.userAgent || '没有浏览器信息' }}</span>
                </div>
                <button
                  class="btn btn-ghost btn-sm"
                  type="button"
                  :disabled="sessionBusy === session.id"
                  @click="revokeSession(session)"
                >
                  {{ sessionBusy === session.id ? '退出中…' : '退出' }}
                </button>
              </li>
            </ul>
            <div v-if="sessions.length" class="session-actions">
              <button v-if="otherSessions" class="btn btn-ghost btn-sm" type="button" @click="revokeOtherSessions">
                退出其他设备
              </button>
              <button class="btn btn-ghost btn-sm session-danger" type="button" @click="revokeAllSessions">
                退出所有设备
              </button>
            </div>
          </section>

          <section class="login-records">
            <header>
              <h3>最近登录与安全动作</h3>
              <p>只记录你自己的登录、注册、改密与绑定邮箱，保留最近的 12 条。</p>
            </header>
            <p v-if="!loginRecords.length" class="muted">还没有记录。</p>
            <ul v-else>
              <li v-for="record in loginRecords" :key="record.id">
                <span class="login-record-action">{{ record.action }}</span>
                <span class="login-record-detail">{{ record.detail || '—' }}</span>
                <span class="login-record-ip">{{ record.ip || '—' }}</span>
                <time>{{ timeAgo(record.createdAt) }}</time>
              </li>
            </ul>
          </section>
        </div>

        <div v-else-if="tab === 'privacy'" class="panel">
          <header class="panel-head">
            <h2>隐私与消息</h2>
            <p>控制别人能看到什么，以及哪些互动会通知你。修改后立即生效。</p>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <form v-else class="settings-form" @submit.prevent="savePrivacySettings">
            <div class="settings-switches">
              <label class="switch-row">
                <input v-model="privacySettings.openFollow" type="checkbox" />
                <span>
                  <strong>公开关注列表</strong>
                  <small>关闭后，别人看不到你关注了谁，关注数量也会隐藏。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openFans" type="checkbox" />
                <span>
                  <strong>公开粉丝列表</strong>
                  <small>关闭后，别人看不到你的粉丝，粉丝数量也会隐藏。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openPrivateChat" type="checkbox" />
                <span>
                  <strong>允许接收私信</strong>
                  <small>关闭后，别人无法再给你发送新的私信，已有对话仍可查看。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openCommentMessage" type="checkbox" />
                <span>
                  <strong>评论与回复通知</strong>
                  <small>关闭后，别人评论你的内容时不再生成站内通知。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openDiggMessage" type="checkbox" />
                <span>
                  <strong>点赞通知</strong>
                  <small>关闭后，别人点赞你的内容时不再生成站内通知。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openFollowMessage" type="checkbox" />
                <span>
                  <strong>关注通知</strong>
                  <small>关闭后，别人关注你时不再生成站内通知。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openAccessMessage" type="checkbox" />
                <span>
                  <strong>阅读申请通知</strong>
                  <small>关闭后，私密文档的阅读申请和处理结果不再生成站内通知。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.emailNotify" type="checkbox" :disabled="!auth.user.email" />
                <span>
                  <strong>邮件通知</strong>
                  <small v-if="!auth.user.email">先在「账号安全」绑定邮箱，才能开启邮件通知。</small>
                  <small v-else>评论、关注、阅读申请和处理结果会发送到 {{ auth.user.email }}；点赞不会发邮件。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="privacySettings.openCollect" type="checkbox" />
                <span>
                  <strong>公开我的收藏夹</strong>
                  <small>开启后，访客可以在你的公开主页看到收藏夹和已发布内容；默认保持私密。</small>
                </span>
              </label>
            </div>

            <div class="field">
              <label for="me-home-style">公开主页样式</label>
              <SelectBox id="me-home-style" v-model="privacySettings.homeStyle" :options="homeStyleOptions" />
              <span class="hint">只影响 `/u/{{ auth.user.username }}` 公开主页的头部。</span>
            </div>

            <div class="form-actions">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="savingPrivacy">
                {{ savingPrivacy ? '保存中…' : '保存设置' }}
              </button>
            </div>
          </form>
        </div>

        <div v-else-if="tab === 'articles' || tab === 'docs'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>{{ tab === 'articles' ? '我的文章' : '我的文档' }}</h2>
              <p>草稿和定时内容不会出现在公开列表，发布后才会公开。</p>
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
                  <span class="badge" :class="doc.status === 'draft' ? 'badge-draft' : doc.status === 'scheduled' ? 'badge-scheduled' : 'badge-live'">
                    {{ doc.status === 'draft' ? '草稿' : doc.status === 'scheduled' ? '定时发布' : '已发布' }}
                  </span>
                  <span>{{ shortDate(doc.updatedAt) }} 更新</span>
                  <span>{{ doc.viewCount }} 阅读</span>
                  <span>{{ doc.likeCount }} 赞</span>
                  <span v-if="doc.commentCount">{{ doc.commentCount }} 评论</span>
                </div>
              </div>
              <div class="content-row-actions">
                <button
                  v-if="tab === 'articles' && doc.status === 'published'"
                  class="btn btn-ghost btn-sm"
                  type="button"
                  @click="toggleFeatured(doc)"
                >
                  {{ featuredDocID === doc.id ? '取消置顶' : '置顶' }}
                </button>
                <RouterLink :to="`/write/${doc.id}`" class="btn btn-ghost btn-sm">编辑</RouterLink>
                <button class="icon-btn danger" type="button" aria-label="删除" @click="removeDoc(doc)">
                  <Icon name="trash" :size="17" />
                </button>
              </div>
            </article>
          </div>
        </div>

        <div v-else-if="tab === 'favorites'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>我的收藏</h2>
              <p>收藏是私密的。文章页点收藏会进入默认收藏夹，可以在这里整理到不同收藏夹。</p>
            </div>
            <form class="folder-create" @submit.prevent="createFolder">
              <input v-model="newFolderName" maxlength="20" placeholder="新收藏夹名称" />
              <button class="btn btn-primary btn-sm" type="submit">新建收藏夹</button>
            </form>
          </header>

          <div v-if="folders.length" class="folder-tools">
            <div class="folder-chips">
              <button
                v-for="folder in folders"
                :key="folder.id"
                class="tag"
                :class="{ 'tag--active': String(folder.id) === String(selectedFolderID) }"
                type="button"
                @click="selectFolder(folder)"
              >
                {{ folder.name }}
                <span class="tag-count">{{ folder.count }}</span>
              </button>
            </div>
            <div v-if="currentFolder && !currentFolder.isDefault" class="folder-manage">
              <template v-if="editingFolderID === currentFolder.id">
                <input v-model="folderNameDraft" class="folder-name-input" maxlength="20" />
                <button class="btn btn-ghost btn-sm" type="button" @click="saveFolderName(currentFolder)">保存</button>
                <button class="btn btn-ghost btn-sm" type="button" @click="cancelRenameFolder">取消</button>
              </template>
              <template v-else>
                <button class="text-link" type="button" @click="startRenameFolder(currentFolder)">重命名</button>
                <button class="text-link danger" type="button" @click="removeFolder(currentFolder)">删除收藏夹</button>
              </template>
            </div>
          </div>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!favorites.length"
            icon="bookmark"
            title="这个收藏夹还是空的"
            description="在文章或文档底部点一下收藏，它就会出现在这里。"
          >
            <RouterLink to="/articles" class="btn btn-ghost btn-sm">去读点什么</RouterLink>
          </EmptyState>
          <div v-else class="content-list">
            <article v-for="doc in favorites" :key="doc.id" class="content-row">
              <div class="content-row-main">
                <RouterLink :to="docHref(doc)" class="content-row-title">{{ doc.title }}</RouterLink>
                <div class="content-row-meta">
                  <span>{{ doc.kind === 'doc' ? '文档' : '文章' }}</span>
                  <span v-if="doc.category">{{ doc.category.name }}</span>
                  <span>{{ doc.author?.nickname }}</span>
                  <span>{{ shortDate(doc.updatedAt) }}</span>
                </div>
              </div>
              <div class="content-row-actions">
                <SelectBox
                  class="folder-select"
                  :model-value="selectedFolderID"
                  :options="folderSelectOptions"
                  aria-label="移动到收藏夹"
                  @update:model-value="moveFavorite(doc, $event)"
                />
                <button class="icon-btn danger" type="button" aria-label="取消收藏" @click="removeFavorite(doc)">
                  <Icon name="trash" :size="16" />
                </button>
              </div>
            </article>
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

        <div v-else-if="tab === 'requests'" class="panel">
          <header class="panel-head">
            <h2>我的阅读申请</h2>
            <p>你向私密文档作者提交的阅读申请会显示在这里。待处理申请可以撤回。</p>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!outgoingRequests.length"
            icon="lock"
            title="还没有阅读申请"
            description="打开一篇私密文档链接并提交申请后，进度会显示在这里。"
          />
          <div v-else class="content-list">
            <article v-for="item in outgoingRequests" :key="item.id" class="content-row access-outgoing-row">
              <div class="content-row-main">
                <div class="content-row-title">私密文档 #{{ item.docID }}</div>
                <div class="content-row-meta">
                  <span v-if="item.owner?.nickname">作者 {{ item.owner.nickname }}</span>
                  <span>{{ shortDate(item.createdAt) }}</span>
                  <span v-if="item.handledAt">处理于 {{ shortDate(item.handledAt) }}</span>
                </div>
                <p v-if="item.note" class="access-request-note">{{ item.note }}</p>
              </div>
              <div class="content-row-actions">
                <span class="badge" :class="item.status === 'approved' ? 'badge-live' : item.status === 'denied' ? 'access-badge-denied' : ''">
                  {{ accessStatusLabel(item.status) }}
                </span>
                <RouterLink v-if="item.status === 'approved'" :to="`/docs/${item.docID}`" class="btn btn-ghost btn-sm">去阅读</RouterLink>
                <RouterLink v-else-if="item.status === 'denied' || item.status === 'cancelled'" :to="`/docs/${item.docID}`" class="text-link">重新申请</RouterLink>
                <button v-else class="text-link danger" type="button" :disabled="cancellingRequest === item.id" @click="cancelOutgoingRequest(item)">
                  {{ cancellingRequest === item.id ? '撤回中…' : '撤回' }}
                </button>
              </div>
            </article>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
