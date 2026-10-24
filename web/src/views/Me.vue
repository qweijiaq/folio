<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth, refreshMe } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { roleLabel, shortDate, timeAgo } from '../utils/format'
import UserAvatar from '../components/UserAvatar.vue'
import EmailCode from '../components/EmailCode.vue'
import PasswordInput from '../components/PasswordInput.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

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
const loginRecords = ref([])
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
  homeStyle: 'cover'
})

const profileForm = reactive({ nickname: '', bio: '' })
const emailForm = reactive({ email: '', code: '' })
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirm: '' })

const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))
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
  if (!window.confirm(`删除收藏夹「${folder.name}」？里面的收藏会移到默认收藏夹。`)) return
  try {
    await api.del(`/favorite-folders/${folder.id}`)
    selectedFolderID.value = null
    await loadFavorites()
    toast('收藏夹已删除')
  } catch (err) {
    toastError(err)
  }
}

async function moveFavorite(doc, event) {
  const folderID = Number(event.target.value)
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
  if (!window.confirm(`取消收藏「${doc.title}」？`)) return
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
    } else if (tab.value === 'favorites') {
      await loadFavorites()
    } else if (tab.value === 'history') {
      const data = await api.get('/me/history', { pageSize: 50 })
      history.value = data.items
    } else if (tab.value === 'privacy') {
      await loadPrivacySettings()
    } else if (tab.value === 'security') {
      await loadLoginRecords()
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
            <p>修改密码后，当前登录状态会保留。</p>
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
            </div>

            <div class="field">
              <label for="me-home-style">公开主页样式</label>
              <select id="me-home-style" v-model="privacySettings.homeStyle">
                <option value="cover">封面：保留顶部自动封面</option>
                <option value="compact">紧凑：较矮的封面条</option>
                <option value="plain">纯文字：不显示封面</option>
              </select>
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
                <select class="folder-select" :value="selectedFolderID" aria-label="移动到收藏夹" @change="moveFavorite(doc, $event)">
                  <option v-for="folder in folders" :key="folder.id" :value="folder.id">{{ folder.name }}</option>
                </select>
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
      </section>
    </div>
  </div>
</template>
