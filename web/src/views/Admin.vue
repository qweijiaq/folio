<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, getToken } from '../api/client'
import { auth, refreshMe } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { formatBytes, roleLabel, shortDate } from '../utils/format'
import { mergeSite } from '../stores/site'
import UserAvatar from '../components/UserAvatar.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import PasswordInput from '../components/PasswordInput.vue'
import TrendChart from '../components/TrendChart.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '管理后台', description: '知微管理后台', noindex: true }))

const tab = ref(String(route.query.tab || 'overview'))
const loading = ref(true)
const tabLoading = ref(false)
const stats = ref(null)
const dashboard = ref(null)
const dashboardRange = ref(7)
const dashboardMetric = ref('views')
const docs = ref([])
const users = ref([])
const invites = ref([])
const inviteNote = ref('')
const inviteDays = ref(7)
const creatingInvite = ref(false)
const creatingUser = ref(false)
const exporting = ref(false)
const newUser = reactive({ username: '', nickname: '', email: '', password: '', role: 'user' })
const images = ref([])
const categories = ref([])
const userQuery = ref('')
const imageQuery = ref('')
const statusFilter = ref('')
const kindFilter = ref('')
const savingRole = ref(0)
const imageUploading = ref(false)
const imageInput = ref(null)
const newCategory = ref({ name: '', sort: 0 })
const editingCategory = ref({ id: 0, name: '', sort: 0 })
const savingSettings = ref(false)
const notices = ref([])
const newNotice = reactive({ title: '', content: '', link: '' })
const creatingNotice = ref(false)
const settingsForm = reactive({
  siteName: '',
  siteDesc: '',
  siteOwner: '',
  allowRegister: true,
  loginCaptcha: false
})
const storageForm = reactive({
  provider: 'local',
  qiniuAccessKey: '',
  qiniuSecretKey: '',
  qiniuBucket: '',
  qiniuDomain: '',
  qiniuRegion: '',
  qiniuPrefix: 'folio'
})
const storageInfo = reactive({
  provider: 'local',
  active: 'local',
  source: 'none',
  configured: false,
  hasAccessKey: false,
  hasSecretKey: false,
  qiniuBucket: '',
  qiniuDomain: '',
  qiniuRegion: '',
  qiniuPrefix: 'folio',
  localUploadDir: ''
})
const savingStorage = ref(false)
const testingStorage = ref(false)
const storageTestResult = ref({ ok: null, message: '' })
const logs = ref([])
const logTotal = ref(0)
const logPage = ref(1)
const logQuery = ref('')
const logPageSize = 30

const isSuper = computed(() => auth.user?.role === 'super_admin')
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))
const storageSourceLabel = computed(() => ({
  env: '环境变量',
  database: '网页设置',
  none: '未配置'
}[storageInfo.source] || '未配置'))
const storageActiveLabel = computed(() => (storageInfo.active === 'qiniu' ? '七牛云' : '本地磁盘'))

const tabs = computed(() => {
  const list = [
    { key: 'overview', label: '概览', icon: 'home' },
    { key: 'dashboard', label: '数据看板', icon: 'chart' },
    { key: 'content', label: '内容管理', icon: 'edit' },
    { key: 'categories', label: '分类管理', icon: 'tag' },
    { key: 'images', label: '图片管理', icon: 'image' },
    { key: 'notices', label: '全站通知', icon: 'bell' }
  ]
  if (isSuper.value) {
    list.push(
      { key: 'users', label: '用户与权限', icon: 'user' },
      { key: 'storage', label: '存储设置', icon: 'cloud' },
      { key: 'settings', label: '站点设置', icon: 'settings' },
      { key: 'logs', label: '操作日志', icon: 'clock' }
    )
  }
  return list
})

const siteStats = computed(() => stats.value?.site || {})
const myStats = computed(() => stats.value?.mine || {})
const dashboardSummary = computed(() => dashboard.value?.summary || {})
const dashboardSystem = computed(() => dashboard.value?.system || {})
const dashboardTrendLabels = computed(() => dashboard.value?.trend?.dates || [])
const dashboardTrendValues = computed(() => dashboard.value?.trend?.[dashboardMetric.value] || [])
const dashboardMetrics = [
  { key: 'views', label: '阅读' },
  { key: 'content', label: '内容' },
  { key: 'users', label: '读者' },
  { key: 'logins', label: '登录' }
]

async function loadDashboard() {
  try {
    dashboard.value = await api.get('/admin/dashboard', { range: dashboardRange.value })
  } catch (err) {
    toastError(err)
  }
}

function setDashboardRange(range) {
  if (dashboardRange.value === range) return
  dashboardRange.value = range
  loadDashboard()
}

function uptimeLabel(seconds) {
  const total = Number(seconds) || 0
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  if (days) return `${days} 天 ${hours} 小时`
  if (hours) return `${hours} 小时 ${minutes} 分`
  return `${minutes} 分钟`
}

async function loadStats() {
  try {
    stats.value = await api.get('/admin/stats')
  } catch (err) {
    toastError(err)
  }
}

async function exportContent() {
  exporting.value = true
  try {
    const response = await fetch('/api/admin/export', {
      headers: { Accept: 'application/zip', Authorization: `Bearer ${getToken()}` }
    })
    if (!response.ok) {
      let message = '导出失败'
      try {
        const data = await response.json()
        message = data.error || message
      } catch {
        /* ignore */
      }
      toast(message, 'error')
      return
    }
    const blob = await response.blob()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `folio-export-${new Date().toISOString().slice(0, 10)}.zip`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    toast('导出已开始')
  } catch {
    toast('导出失败，请检查网络', 'error')
  } finally {
    exporting.value = false
  }
}

async function loadDocs() {
  try {
    const data = await api.get('/admin/docs', {
      pageSize: 100,
      status: statusFilter.value || undefined,
      kind: kindFilter.value || undefined
    })
    docs.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function loadUsers() {
  try {
    const data = await api.get('/admin/users', {
      q: userQuery.value.trim() || undefined,
      pageSize: 100
    })
    users.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function loadInvites() {
  try {
    const data = await api.get('/admin/invites')
    invites.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function createInvite() {
  creatingInvite.value = true
  try {
    await api.post('/admin/invites', {
      note: inviteNote.value.trim() || '邀请注册',
      expiresInDays: Number(inviteDays.value) || 7
    })
    inviteNote.value = ''
    await loadInvites()
    toast('邀请链接已生成')
  } catch (err) {
    toastError(err)
  } finally {
    creatingInvite.value = false
  }
}

function inviteLink(code) {
  return `${window.location.origin}/register?invite=${code}`
}

async function copyInvite(invite) {
  try {
    await navigator.clipboard.writeText(inviteLink(invite.code))
    toast('邀请链接已复制')
  } catch {
    toast('复制失败，请手动复制邀请码', 'error')
  }
}

async function removeInvite(invite) {
  if (!window.confirm(`撤销这个邀请码？${invite.note || ''}`)) return
  try {
    await api.del(`/admin/invites/${invite.id}`)
    await loadInvites()
    toast('邀请码已撤销')
  } catch (err) {
    toastError(err)
  }
}

function inviteStatusLabel(status) {
  return { active: '可邀请', used: '已使用', expired: '已过期' }[status] || status
}

async function createUser() {
  const username = newUser.username.trim()
  if (!/^[A-Za-z0-9_]{2,20}$/.test(username)) {
    toast('用户名需为 2-20 位字母、数字或下划线')
    return
  }
  if (newUser.password.length < 6) {
    toast('密码至少 6 位')
    return
  }
  creatingUser.value = true
  try {
    await api.post('/admin/users', {
      username,
      nickname: newUser.nickname.trim() || username,
      email: newUser.email.trim(),
      password: newUser.password,
      role: newUser.role
    })
    newUser.username = ''
    newUser.nickname = ''
    newUser.email = ''
    newUser.password = ''
    newUser.role = 'user'
    await loadUsers()
    toast('用户已创建')
  } catch (err) {
    toastError(err)
  } finally {
    creatingUser.value = false
  }
}

async function loadImages() {
  try {
    const data = await api.get('/images', {
      q: imageQuery.value.trim() || undefined,
      pageSize: 50
    })
    images.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function loadNotices() {
  try {
    const data = await api.get('/admin/notices')
    notices.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function createNotice() {
  const title = newNotice.title.trim()
  const content = newNotice.content.trim()
  if (!title || !content) {
    toast('请填写公告标题和内容')
    return
  }
  creatingNotice.value = true
  try {
    await api.post('/admin/notices', { title, content, link: newNotice.link.trim() })
    newNotice.title = ''
    newNotice.content = ''
    newNotice.link = ''
    await loadNotices()
    toast('公告已发布')
  } catch (err) {
    toastError(err)
  } finally {
    creatingNotice.value = false
  }
}

async function removeNotice(notice) {
  if (!window.confirm(`删除公告「${notice.title}」？`)) return
  try {
    await api.del(`/admin/notices/${notice.id}`)
    await loadNotices()
    toast('公告已删除')
  } catch (err) {
    toastError(err)
  }
}

async function loadSettings() {
  try {
    const data = await api.get('/admin/settings')
    const setting = data.settings || {}
    settingsForm.siteName = setting.siteName || ''
    settingsForm.siteDesc = setting.siteDesc || ''
    settingsForm.siteOwner = setting.siteOwner || ''
    settingsForm.allowRegister = Boolean(setting.allowRegister)
    settingsForm.loginCaptcha = Boolean(setting.loginCaptcha)
  } catch (err) {
    toastError(err)
  }
}

async function saveSettings() {
  const name = settingsForm.siteName.trim()
  if (!name) {
    toast('站点名称不能为空')
    return
  }
  savingSettings.value = true
  try {
    const data = await api.put('/admin/settings', {
      siteName: name,
      siteDesc: settingsForm.siteDesc.trim(),
      siteOwner: settingsForm.siteOwner.trim(),
      allowRegister: settingsForm.allowRegister,
      loginCaptcha: settingsForm.loginCaptcha
    })
    if (data.site) mergeSite(data.site)
    toast('站点设置已保存')
  } catch (err) {
    toastError(err)
  } finally {
    savingSettings.value = false
  }
}

function applyStorageInfo(storage = {}) {
  Object.assign(storageInfo, {
    provider: storage.provider || 'local',
    active: storage.active || 'local',
    source: storage.source || 'none',
    configured: Boolean(storage.configured),
    hasAccessKey: Boolean(storage.hasAccessKey),
    hasSecretKey: Boolean(storage.hasSecretKey),
    qiniuBucket: storage.qiniuBucket || '',
    qiniuDomain: storage.qiniuDomain || '',
    qiniuRegion: storage.qiniuRegion || '',
    qiniuPrefix: storage.qiniuPrefix || 'folio',
    localUploadDir: storage.localUploadDir || ''
  })
  storageForm.provider = storageInfo.provider
  storageForm.qiniuBucket = storageInfo.qiniuBucket
  storageForm.qiniuDomain = storageInfo.qiniuDomain
  storageForm.qiniuRegion = storageInfo.qiniuRegion
  storageForm.qiniuPrefix = storageInfo.qiniuPrefix
  storageForm.qiniuAccessKey = ''
  storageForm.qiniuSecretKey = ''
}

async function loadStorage() {
  storageTestResult.value = { ok: null, message: '' }
  try {
    const data = await api.get('/admin/storage')
    applyStorageInfo(data.storage)
  } catch (err) {
    toastError(err)
  }
}

function storagePayload() {
  return {
    provider: storageForm.provider,
    qiniuAccessKey: storageForm.qiniuAccessKey,
    qiniuSecretKey: storageForm.qiniuSecretKey,
    qiniuBucket: storageForm.qiniuBucket,
    qiniuDomain: storageForm.qiniuDomain,
    qiniuRegion: storageForm.qiniuRegion,
    qiniuPrefix: storageForm.qiniuPrefix
  }
}

function validateStorageForm() {
  if (storageForm.provider !== 'qiniu') return true
  if (!storageForm.qiniuAccessKey.trim() && !storageInfo.hasAccessKey) {
    toast('请填写七牛 AccessKey')
    return false
  }
  if (!storageForm.qiniuSecretKey.trim() && !storageInfo.hasSecretKey) {
    toast('请填写七牛 SecretKey')
    return false
  }
  if (!storageForm.qiniuBucket.trim()) {
    toast('请填写七牛空间名')
    return false
  }
  if (!storageForm.qiniuDomain.trim()) {
    toast('请填写七牛绑定域名')
    return false
  }
  return true
}

async function saveStorage() {
  if (!validateStorageForm()) return
  savingStorage.value = true
  try {
    const data = await api.put('/admin/storage', storagePayload())
    applyStorageInfo(data.storage)
    storageTestResult.value = { ok: null, message: '' }
    toast(storageInfo.provider === 'qiniu' ? '已切换到七牛云存储' : '已切换到本地存储')
  } catch (err) {
    toastError(err)
  } finally {
    savingStorage.value = false
  }
}

async function testStorage() {
  if (!validateStorageForm()) return
  testingStorage.value = true
  storageTestResult.value = { ok: null, message: '' }
  try {
    const data = await api.post('/admin/storage/test', storagePayload())
    storageTestResult.value = { ok: true, message: data.message || '连接成功' }
  } catch (err) {
    storageTestResult.value = { ok: false, message: err.message || '连接失败' }
  } finally {
    testingStorage.value = false
  }
}

watch(storageForm, () => {
  if (storageTestResult.value.message) storageTestResult.value = { ok: null, message: '' }
}, { deep: true })

async function loadLogs(page = logPage.value) {
  logPage.value = page
  try {
    const data = await api.get('/admin/logs', {
      page,
      pageSize: logPageSize,
      q: logQuery.value.trim() || undefined
    })
    logs.value = data.items
    logTotal.value = data.total
  } catch (err) {
    toastError(err)
  }
}

function searchLogs() {
  logPage.value = 1
  loadLogs(1)
}

function logPages() {
  return Math.max(1, Math.ceil(logTotal.value / logPageSize))
}

function targetLabel(type) {
  return { article: '文章', doc: '文档', user: '用户', category: '分类', image: '图片', site: '站点' }[type] || type
}

function logTime(value) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${shortDate(value)} ${hh}:${mm}`
}

async function loadCategories() {
  try {
    const data = await api.get('/categories')
    categories.value = data.items
  } catch (err) {
    toastError(err)
  }
}

async function createCategory() {
  const name = newCategory.value.name.trim()
  if (!name) {
    toast('请输入分类名称')
    return
  }
  try {
    await api.post('/categories', { name, sort: Number(newCategory.value.sort) || 0 })
    newCategory.value = { name: '', sort: 0 }
    await loadCategories()
    toast('分类已添加')
  } catch (err) {
    toastError(err)
  }
}

function startEditCategory(category) {
  editingCategory.value = { id: category.id, name: category.name, sort: category.sort }
}

function cancelEditCategory() {
  editingCategory.value = { id: 0, name: '', sort: 0 }
}

async function saveCategory() {
  const current = editingCategory.value
  const name = current.name.trim()
  if (!name) {
    toast('分类名称不能为空')
    return
  }
  try {
    await api.put(`/categories/${current.id}`, { name, sort: Number(current.sort) || 0 })
    cancelEditCategory()
    await loadCategories()
    toast('分类已更新')
  } catch (err) {
    toastError(err)
  }
}

async function removeCategory(category) {
  if (!window.confirm(`删除分类「${category.name}」？该分类下的文章会变为未分类。`)) return
  try {
    await api.del(`/categories/${category.id}`)
    await loadCategories()
    toast('分类已删除')
  } catch (err) {
    toastError(err)
  }
}

async function onImagePick(event) {
  const files = Array.from(event.target.files || [])
  event.target.value = ''
  if (!files.length) return
  imageUploading.value = true
  try {
    for (const file of files) {
      await api.uploadImage(file)
    }
    toast(`已上传 ${files.length} 张图片`)
    await loadImages()
  } catch (err) {
    toastError(err)
  } finally {
    imageUploading.value = false
  }
}

async function copyImage(image) {
  try {
    await navigator.clipboard.writeText(new URL(image.url, window.location.origin).href)
    toast('图片链接已复制')
  } catch {
    toast('复制失败', 'error')
  }
}

async function removeImage(image) {
  if (!window.confirm(`删除图片「${image.filename}」？引用它的文章可能显示失败。`)) return
  try {
    await api.del(`/images/${image.id}`)
    await loadImages()
    toast('图片已删除')
  } catch (err) {
    toastError(err)
  }
}

function normalizeTab() {
  if (!tabs.value.some((item) => item.key === tab.value)) tab.value = 'overview'
}

async function loadTab() {
  normalizeTab()
  tabLoading.value = true
  try {
    if (tab.value === 'overview') await loadStats()
    else if (tab.value === 'dashboard') await loadDashboard()
    else if (tab.value === 'content') await loadDocs()
    else if (tab.value === 'categories') await loadCategories()
    else if (tab.value === 'images') await loadImages()
    else if (tab.value === 'notices') await loadNotices()
    else if (tab.value === 'users') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await Promise.all([loadUsers(), loadInvites()])
      }
    } else if (tab.value === 'settings') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await loadSettings()
      }
    } else if (tab.value === 'storage') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await loadStorage()
      }
    } else if (tab.value === 'logs') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await loadLogs(1)
      }
    }
  } finally {
    tabLoading.value = false
  }
}

function setTab(key) {
  if (tab.value === key) return
  tab.value = key
  router.replace({ path: '/admin', query: { tab: key } })
  loadTab()
}

async function applyDocFilter() {
  await loadDocs()
}

async function updateRole(user, event) {
  const next = event.target.value
  if (!next || next === user.role) return
  savingRole.value = user.id
  try {
    await api.put(`/admin/users/${user.id}/role`, { role: next })
    user.role = next
    toast(next === 'admin' ? `已将 ${user.nickname} 设为管理员` : `已取消 ${user.nickname} 的管理员权限`)
  } catch (err) {
    toastError(err)
    await loadUsers()
  } finally {
    savingRole.value = 0
  }
}

async function removeUser(user) {
  if (!window.confirm(`删除用户 @${user.username} 及其全部内容？这个操作不可撤销。`)) return
  try {
    await api.del(`/admin/users/${user.id}`)
    await loadUsers()
    await loadStats()
    toast('用户已删除')
  } catch (err) {
    toastError(err)
  }
}

async function removeDoc(doc) {
  if (!window.confirm(`删除「${doc.title}」？文档的子文档和所有互动也会一并删除。`)) return
  try {
    await api.del(`/docs/${doc.id}`)
    await loadDocs()
    await loadStats()
    toast('已删除')
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
    const next = String(value || 'overview')
    if (!tabs.value.some((item) => item.key === next)) {
      tab.value = 'overview'
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
  if (!staff.value) {
    toast('需要管理员权限')
    router.replace('/')
    return
  }
  normalizeTab()
  await loadTab()
  loading.value = false
})
</script>

<template>
  <div class="page admin-page">
    <header class="page-hero container">
      <span class="eyebrow">后台</span>
      <div class="page-hero-row">
        <div>
          <h1>管理后台</h1>
          <p class="page-lede">写作、内容、站点设置与权限。只有超级管理员和管理员可以进入这里。</p>
        </div>
        <RouterLink to="/" class="text-link">返回站点 <Icon name="arrowRight" :size="14" /></RouterLink>
      </div>
    </header>

    <div v-if="loading" class="container page-loading">
      <p class="muted loading-line">正在打开后台…</p>
    </div>

    <div v-else-if="staff" class="container me-layout">
      <aside class="me-side">
        <div class="me-card">
          <UserAvatar :username="auth.user.username" :nickname="auth.user.nickname" :size="72" />
          <h2>{{ auth.user.nickname }}</h2>
          <span class="me-handle">@{{ auth.user.username }}</span>
          <span class="badge badge-admin">{{ roleLabel(auth.user.role) }}</span>
          <p>{{ isSuper ? '你可以写作、管理全部内容，并设置管理员。' : '你可以写作和管理自己的内容。' }}</p>
          <RouterLink :to="`/u/${auth.user.username}`" class="text-link">查看公开主页</RouterLink>
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
          </button>
        </nav>
      </aside>

      <section class="me-main">
        <div v-if="tab === 'overview'" class="panel">
          <header class="panel-head">
            <h2>概览</h2>
            <p>{{ isSuper ? '全站数据与你的创作概况。' : '站点公开数据与你的创作概况。' }}</p>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <template v-else>
            <div class="admin-stat-grid">
              <div class="admin-stat">
                <span>已发布文章</span>
                <strong>{{ siteStats.articles || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>已发布文档</span>
                <strong>{{ siteStats.docs || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>读者</span>
                <strong>{{ siteStats.users || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>评论</span>
                <strong>{{ siteStats.comments || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>累计阅读</span>
                <strong>{{ siteStats.views || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>点赞 / 收藏</span>
                <strong>{{ siteStats.likes || 0 }} / {{ siteStats.favorites || 0 }}</strong>
              </div>
            </div>

            <div class="admin-note">
              <span>我的内容</span>
              <p>{{ myStats.articles || 0 }} 篇文章、{{ myStats.docs || 0 }} 篇文档，其中 {{ myStats.drafts || 0 }} 篇草稿。</p>
            </div>

            <div class="admin-quick">
              <RouterLink to="/write" class="btn btn-primary btn-sm">
                <Icon name="edit" :size="15" /> 写文章
              </RouterLink>
              <RouterLink to="/write?kind=doc" class="btn btn-ghost btn-sm">
                <Icon name="plus" :size="15" /> 新建文档
              </RouterLink>
              <RouterLink v-if="isSuper" :to="{ path: '/admin', query: { tab: 'users' } }" class="btn btn-ghost btn-sm">
                <Icon name="user" :size="15" /> 用户与权限
              </RouterLink>
            </div>
          </template>
        </div>

        <div v-else-if="tab === 'dashboard'" class="panel dashboard-panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>数据看板</h2>
              <p>阅读、内容、读者与登录趋势；每日阅读从当前版本开始累计。</p>
            </div>
            <div class="dashboard-range">
              <button v-for="range in [7, 30, 90]" :key="range" type="button" :class="{ active: dashboardRange === range }" @click="setDashboardRange(range)">
                {{ range }} 天
              </button>
            </div>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在计算数据…</p>
          <EmptyState v-else-if="!dashboard" icon="chart" title="暂无数据" description="产生阅读、内容或登录后，这里会显示趋势。">
            <button class="btn btn-ghost btn-sm" type="button" @click="loadDashboard">重新加载</button>
          </EmptyState>
          <template v-else>
            <div class="admin-stat-grid dashboard-stat-grid">
              <div class="admin-stat">
                <span>{{ dashboardRange }} 天阅读</span>
                <strong>{{ dashboardSummary.viewsRange || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>累计阅读</span>
                <strong>{{ dashboardSummary.views || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>新增内容</span>
                <strong>{{ dashboardSummary.newContent || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>新增读者</span>
                <strong>{{ dashboardSummary.newUsers || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>今日登录</span>
                <strong>{{ dashboardSummary.todayLogins || 0 }}</strong>
              </div>
              <div class="admin-stat">
                <span>图片 / 存储</span>
                <strong>{{ dashboardSummary.images || 0 }} <small>{{ formatBytes(dashboardSummary.uploadBytes || 0) }}</small></strong>
              </div>
            </div>

            <section class="dashboard-chart-card">
              <header class="dashboard-chart-head">
                <div>
                  <h3>趋势</h3>
                  <p>{{ dashboardRange }} 天内的每日变化</p>
                </div>
                <div class="tag-row">
                  <button
                    v-for="metric in dashboardMetrics"
                    :key="metric.key"
                    class="tag"
                    :class="{ 'tag--active': dashboardMetric === metric.key }"
                    type="button"
                    @click="dashboardMetric = metric.key"
                  >
                    {{ metric.label }}
                  </button>
                </div>
              </header>
              <TrendChart :labels="dashboardTrendLabels" :values="dashboardTrendValues" />
            </section>

            <div class="dashboard-columns">
              <section class="dashboard-card">
                <header class="dashboard-card-head">
                  <div>
                    <h3>热门内容</h3>
                    <p>按累计阅读排序</p>
                  </div>
                  <RouterLink :to="{ path: '/admin', query: { tab: 'content' } }" class="text-link">全部内容</RouterLink>
                </header>
                <EmptyState v-if="!dashboard.top?.length" icon="edit" title="还没有内容" description="发布文章或文档后会出现在这里。" />
                <div v-else class="dashboard-rank-list">
                  <RouterLink
                    v-for="(item, index) in dashboard.top"
                    :key="item.id"
                    :to="`/${item.kind === 'doc' ? 'docs' : 'articles'}/${item.id}`"
                    class="dashboard-rank"
                  >
                    <span class="dashboard-rank-num">{{ index + 1 }}</span>
                    <span class="dashboard-rank-main">
                      <strong>{{ item.title }}</strong>
                      <small>{{ item.kind === 'doc' ? '文档' : '文章' }} · {{ item.author?.nickname }}<template v-if="item.category"> · {{ item.category.name }}</template></small>
                    </span>
                    <span class="dashboard-rank-count">{{ item.viewCount }} 阅读</span>
                  </RouterLink>
                </div>
              </section>

              <section class="dashboard-card">
                <header class="dashboard-card-head">
                  <div>
                    <h3>最近动态</h3>
                    <p>登录、发布与管理动作</p>
                  </div>
                  <RouterLink v-if="isSuper" :to="{ path: '/admin', query: { tab: 'logs' } }" class="text-link">全部日志</RouterLink>
                </header>
                <p v-if="!dashboard.activity?.length" class="empty-inline">这段时间还没有记录。</p>
                <div v-else class="dashboard-activity-list">
                  <div v-for="entry in dashboard.activity" :key="entry.id" class="dashboard-activity">
                    <span class="badge">{{ entry.action }}</span>
                    <span class="dashboard-activity-main">
                      <strong>@{{ entry.username || '匿名' }}</strong>
                      <span>{{ entry.detail || '—' }}</span>
                    </span>
                    <time>{{ shortDate(entry.createdAt) }}</time>
                  </div>
                </div>
              </section>
            </div>

            <div class="dashboard-system">
              <span>数据库 {{ dashboardSystem.db || '—' }}</span>
              <span>Redis {{ dashboardSystem.redis || 'off' }}</span>
              <span>图片存储 {{ dashboardSystem.storage || 'local' }}</span>
              <span>Go {{ dashboardSystem.goVersion }}</span>
              <span>{{ dashboardSystem.goroutines }} goroutines</span>
              <span>内存 {{ formatBytes(dashboardSystem.memoryBytes || 0) }}</span>
              <span>运行 {{ uptimeLabel(dashboardSystem.uptimeSec) }}</span>
            </div>
          </template>
        </div>

        <div v-else-if="tab === 'content'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>内容管理</h2>
              <p>{{ isSuper ? '全站内容都在这里，可以编辑或删除。' : '这里只显示你创建的内容。' }}</p>
            </div>
            <div class="admin-actions">
              <button class="btn btn-ghost btn-sm" type="button" :disabled="exporting" @click="exportContent">
                {{ exporting ? '导出中…' : '导出 Markdown' }}
              </button>
              <RouterLink to="/write" class="btn btn-ghost btn-sm">写文章</RouterLink>
              <RouterLink to="/write?kind=doc" class="btn btn-primary btn-sm">新建文档</RouterLink>
            </div>
          </header>

          <div class="admin-filters">
            <div class="tag-row">
              <button class="tag" :class="{ 'tag--active': statusFilter === '' }" type="button" @click="statusFilter = ''; applyDocFilter()">全部状态</button>
              <button class="tag" :class="{ 'tag--active': statusFilter === 'published' }" type="button" @click="statusFilter = 'published'; applyDocFilter()">已发布</button>
              <button class="tag" :class="{ 'tag--active': statusFilter === 'draft' }" type="button" @click="statusFilter = 'draft'; applyDocFilter()">草稿</button>
            </div>
            <div class="tag-row">
              <button class="tag" :class="{ 'tag--active': kindFilter === '' }" type="button" @click="kindFilter = ''; applyDocFilter()">全部类型</button>
              <button class="tag" :class="{ 'tag--active': kindFilter === 'article' }" type="button" @click="kindFilter = 'article'; applyDocFilter()">文章</button>
              <button class="tag" :class="{ 'tag--active': kindFilter === 'doc' }" type="button" @click="kindFilter = 'doc'; applyDocFilter()">文档</button>
            </div>
          </div>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState v-else-if="!docs.length" icon="edit" title="还没有内容" description="写一篇文章，或创建一篇文档。">
            <RouterLink to="/write" class="btn btn-primary btn-sm">开始写作</RouterLink>
          </EmptyState>
          <div v-else class="content-list">
            <article v-for="doc in docs" :key="doc.id" class="content-row">
              <div class="content-row-main">
                <RouterLink :to="docHref(doc)" class="content-row-title">{{ doc.title }}</RouterLink>
                <div class="content-row-meta">
                  <span class="badge" :class="doc.status === 'draft' ? 'badge-draft' : 'badge-live'">
                    {{ doc.status === 'draft' ? '草稿' : '已发布' }}
                  </span>
                  <span>{{ doc.kind === 'doc' ? '文档' : '文章' }}</span>
                  <span v-if="doc.category">{{ doc.category.name }}</span>
                  <span v-if="isSuper" class="admin-author">
                    @{{ doc.author?.username }} · {{ doc.author?.nickname }}
                  </span>
                  <span>{{ shortDate(doc.updatedAt) }} 更新</span>
                  <span>{{ doc.viewCount }} 阅读</span>
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

        <div v-else-if="tab === 'categories'" class="panel">
          <header class="panel-head">
            <h2>分类管理</h2>
            <p>文章可以按分类整理；文档仍然使用左侧目录树。删除分类后，文章会变为未分类。</p>
          </header>

          <form class="category-create" @submit.prevent="createCategory">
            <input v-model="newCategory.name" maxlength="16" placeholder="新分类名称" />
            <input v-model.number="newCategory.sort" class="sort-input" type="number" aria-label="排序" />
            <button class="btn btn-primary btn-sm" type="submit">添加分类</button>
          </form>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState v-else-if="!categories.length" icon="tag" title="还没有分类" description="先建立一个分类，再在写文章时选择它。" />
          <div v-else class="category-list">
            <div v-for="category in categories" :key="category.id" class="category-row">
              <template v-if="editingCategory.id === category.id">
                <input v-model="editingCategory.name" class="category-edit-input" maxlength="16" />
                <input v-model.number="editingCategory.sort" class="sort-input" type="number" aria-label="排序" />
                <button class="btn btn-primary btn-sm" type="button" @click="saveCategory">保存</button>
                <button class="btn btn-ghost btn-sm" type="button" @click="cancelEditCategory">取消</button>
              </template>
              <template v-else>
                <span class="category-name">{{ category.name }}</span>
                <span class="category-count">{{ category.count }} 篇文章</span>
                <span class="category-sort">排序 {{ category.sort }}</span>
                <div class="category-actions">
                  <button class="btn btn-ghost btn-sm" type="button" @click="startEditCategory(category)">编辑</button>
                  <button class="icon-btn danger" type="button" aria-label="删除分类" @click="removeCategory(category)">
                    <Icon name="trash" :size="16" />
                  </button>
                </div>
              </template>
            </div>
          </div>
        </div>

        <div v-else-if="tab === 'images'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>图片管理</h2>
              <p>{{ isSuper ? '全站上传的图片，可以复制链接、下载或删除。' : '你上传的图片，可以复制链接、下载或删除。' }}</p>
            </div>
            <div class="admin-actions">
              <form class="admin-search" @submit.prevent="loadImages">
                <input v-model="imageQuery" type="search" placeholder="搜索文件名" />
                <button class="btn btn-ghost btn-sm" type="submit">搜索</button>
              </form>
              <button class="btn btn-primary btn-sm" type="button" :disabled="imageUploading" @click="imageInput?.click()">
                <Icon name="plus" :size="15" />
                {{ imageUploading ? '上传中…' : '上传图片' }}
              </button>
              <input
                ref="imageInput"
                class="hidden-file"
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                multiple
                @change="onImagePick"
              />
            </div>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState
            v-else-if="!images.length"
            icon="image"
            title="还没有图片"
            description="上传的图片会出现在这里，也可以直接粘进文章或文档。"
          >
            <button class="btn btn-primary btn-sm" type="button" @click="imageInput?.click()">上传第一张</button>
          </EmptyState>
          <div v-else class="image-grid">
            <article v-for="image in images" :key="image.id" class="image-card">
              <a :href="image.url" target="_blank" rel="noopener" class="image-thumb">
                <img :src="image.url" :alt="image.filename" loading="lazy" />
              </a>
              <div class="image-info">
                <strong :title="image.filename">{{ image.filename }}</strong>
                <span>{{ formatBytes(image.size) }} · {{ shortDate(image.createdAt) }}</span>
                <span v-if="isSuper">@{{ image.user?.username }} · {{ image.user?.nickname }}</span>
              </div>
              <div class="image-actions">
                <button class="btn btn-ghost btn-sm" type="button" @click="copyImage(image)">复制链接</button>
                <a class="btn btn-ghost btn-sm" :href="image.url" :download="image.filename">下载</a>
                <button class="icon-btn danger" type="button" aria-label="删除图片" @click="removeImage(image)">
                  <Icon name="trash" :size="16" />
                </button>
              </div>
            </article>
          </div>
        </div>

        <div v-else-if="tab === 'users' && isSuper" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>用户与权限</h2>
              <p>超级管理员可以设置管理员。管理员可以发布内容，但不能管理用户。</p>
            </div>
            <form class="admin-search" @submit.prevent="loadUsers">
              <input v-model="userQuery" type="search" placeholder="搜索用户名、昵称或邮箱" />
              <button class="btn btn-ghost btn-sm" type="submit">搜索</button>
            </form>
          </header>

          <div class="admin-user-tools">
            <section class="admin-tool-card">
              <header class="admin-tool-head">
                <div>
                  <h3>邀请注册</h3>
                  <p>生成一次性邀请链接。适合关闭注册后只邀请朋友加入。</p>
                </div>
              </header>
              <form class="invite-form" @submit.prevent="createInvite">
                <input v-model="inviteNote" maxlength="100" placeholder="备注，例如「给阿林」" />
                <select v-model.number="inviteDays" aria-label="有效期">
                  <option :value="1">1 天</option>
                  <option :value="7">7 天</option>
                  <option :value="30">30 天</option>
                  <option :value="90">90 天</option>
                </select>
                <button class="btn btn-primary btn-sm" type="submit" :disabled="creatingInvite">
                  {{ creatingInvite ? '生成中…' : '生成邀请' }}
                </button>
              </form>
              <p v-if="!invites.length" class="hint">还没有邀请码。</p>
              <div v-else class="invite-list">
                <div v-for="invite in invites" :key="invite.id" class="invite-row">
                  <span class="badge" :class="invite.status === 'active' ? 'badge-live' : ''">{{ inviteStatusLabel(invite.status) }}</span>
                  <span class="invite-main">
                    <strong>{{ invite.note || '邀请注册' }}</strong>
                    <small v-if="invite.status === 'active'">{{ inviteLink(invite.code) }}</small>
                    <small v-else-if="invite.status === 'used'">{{ shortDate(invite.usedAt) }} 已使用 · 由 {{ invite.createdByName }}</small>
                    <small v-else>{{ shortDate(invite.expiresAt) }} 过期 · 由 {{ invite.createdByName }}</small>
                  </span>
                  <template v-if="invite.status === 'active'">
                    <button class="text-link" type="button" @click="copyInvite(invite)">复制链接</button>
                    <button class="text-link danger" type="button" @click="removeInvite(invite)">撤销</button>
                  </template>
                </div>
              </div>
            </section>

            <section class="admin-tool-card">
              <header class="admin-tool-head">
                <div>
                  <h3>直接创建用户</h3>
                  <p>不需要邮箱验证，适合帮朋友开账号。</p>
                </div>
              </header>
              <form class="create-user-form" @submit.prevent="createUser">
                <div class="form-row">
                  <div class="field">
                    <label for="new-user-name">用户名</label>
                    <input id="new-user-name" v-model="newUser.username" maxlength="20" placeholder="字母、数字或下划线" />
                  </div>
                  <div class="field">
                    <label for="new-user-nick">昵称</label>
                    <input id="new-user-nick" v-model="newUser.nickname" maxlength="24" placeholder="可选" />
                  </div>
                </div>
                <div class="form-row">
                  <div class="field">
                    <label for="new-user-email">邮箱</label>
                    <input id="new-user-email" v-model="newUser.email" type="email" placeholder="可选" />
                  </div>
                  <div class="field">
                    <label for="new-user-password">初始密码</label>
                    <PasswordInput id="new-user-password" v-model="newUser.password" autocomplete="new-password" placeholder="至少 6 位" />
                  </div>
                </div>
                <div class="form-row">
                  <div class="field">
                    <label for="new-user-role">角色</label>
                    <select id="new-user-role" v-model="newUser.role">
                      <option value="user">读者</option>
                      <option value="admin">管理员</option>
                    </select>
                  </div>
                  <div class="form-actions">
                    <button class="btn btn-primary btn-sm" type="submit" :disabled="creatingUser">
                      {{ creatingUser ? '创建中…' : '创建用户' }}
                    </button>
                  </div>
                </div>
              </form>
            </section>
          </div>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <div v-else class="admin-table admin-users-table">
            <div class="admin-table-head">
              <span>用户</span>
              <span>邮箱</span>
              <span>角色</span>
              <span>内容</span>
              <span>加入时间</span>
              <span></span>
            </div>
            <div v-for="user in users" :key="user.id" class="admin-table-row">
              <RouterLink :to="`/u/${user.username}`" class="admin-user">
                <UserAvatar :username="user.username" :nickname="user.nickname" :size="34" />
                <span>
                  <strong>{{ user.nickname }}</strong>
                  <span>@{{ user.username }}</span>
                </span>
              </RouterLink>
              <span class="admin-email">{{ user.email || '—' }}</span>
              <span class="admin-role-cell">
                <template v-if="user.role === 'super_admin'">
                  <span class="badge badge-admin">超级管理员</span>
                </template>
                <template v-else>
                  <select
                    class="role-select"
                    :value="user.role"
                    :disabled="savingRole === user.id"
                    @change="updateRole(user, $event)"
                  >
                    <option value="user">读者</option>
                    <option value="admin">管理员</option>
                  </select>
                </template>
              </span>
              <span>{{ user.articles }} 文章 / {{ user.docs }} 文档</span>
              <span>{{ shortDate(user.createdAt) }}</span>
              <button
                v-if="user.role !== 'super_admin'"
                class="icon-btn danger"
                type="button"
                aria-label="删除用户"
                @click="removeUser(user)"
              >
                <Icon name="trash" :size="17" />
              </button>
              <span v-else class="hint">当前站点所有者</span>
            </div>
          </div>
        </div>

        <div v-else-if="tab === 'notices'" class="panel">
          <header class="panel-head">
            <h2>全站通知</h2>
            <p>公告会显示在所有用户的「消息 → 通知」顶部，并计入未读提醒。</p>
          </header>

          <form class="settings-form notice-form" @submit.prevent="createNotice">
            <div class="field">
              <label for="notice-title">标题</label>
              <input id="notice-title" v-model="newNotice.title" maxlength="80" placeholder="例如：站点维护通知" />
            </div>
            <div class="field">
              <label for="notice-content">内容</label>
              <textarea id="notice-content" v-model="newNotice.content" rows="3" maxlength="1000" placeholder="写清楚发生了什么、什么时候、需要读者做什么。"></textarea>
            </div>
            <div class="field">
              <label for="notice-link">链接 <span class="hint">可选</span></label>
              <input id="notice-link" v-model="newNotice.link" maxlength="255" placeholder="/articles 或 https://…" />
            </div>
            <div class="form-actions">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="creatingNotice">
                {{ creatingNotice ? '发布中…' : '发布公告' }}
              </button>
            </div>
          </form>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState v-else-if="!notices.length" icon="bell" title="还没有全站通知" description="发布后，所有登录用户都会在通知页看到它。" />
          <div v-else class="notice-admin-list">
            <article v-for="notice in notices" :key="notice.id" class="notice-admin-item">
              <div>
                <strong>{{ notice.title }}</strong>
                <p>{{ notice.content }}</p>
                <span>{{ shortDate(notice.createdAt) }} · {{ notice.createdByName }}<template v-if="notice.link"> · {{ notice.link }}</template></span>
              </div>
              <button class="icon-btn danger" type="button" aria-label="删除公告" @click="removeNotice(notice)">
                <Icon name="trash" :size="16" />
              </button>
            </article>
          </div>
        </div>

        <div v-else-if="tab === 'settings' && isSuper" class="panel">
          <header class="panel-head">
            <h2>站点设置</h2>
            <p>站点名称、描述与维护者会立即显示在页头、页脚和首页；注册开关对访客生效。</p>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <form v-else class="settings-form" @submit.prevent="saveSettings">
            <div class="form-row">
              <div class="field">
                <label for="setting-name">站点名称</label>
                <input id="setting-name" v-model="settingsForm.siteName" maxlength="32" />
              </div>
              <div class="field">
                <label for="setting-owner">维护者</label>
                <input id="setting-owner" v-model="settingsForm.siteOwner" maxlength="32" />
              </div>
            </div>
            <div class="field">
              <label for="setting-desc">站点描述</label>
              <textarea id="setting-desc" v-model="settingsForm.siteDesc" rows="3" maxlength="120" placeholder="一句话说明这个站点…"></textarea>
              <span class="hint">{{ settingsForm.siteDesc.length }} / 120</span>
            </div>

            <div class="settings-switches">
              <label class="switch-row">
                <input v-model="settingsForm.allowRegister" type="checkbox" />
                <span>
                  <strong>开放注册</strong>
                  <small>关闭后登录页不再显示注册入口，注册接口会返回 403。已有账号不受影响。</small>
                </span>
              </label>
              <label class="switch-row">
                <input v-model="settingsForm.loginCaptcha" type="checkbox" />
                <span>
                  <strong>登录图形验证码</strong>
                  <small>开启后密码登录和发送邮箱验证码都需要输入图片字符。环境变量只作为首次启动的默认值。</small>
                </span>
              </label>
            </div>

            <div class="form-actions">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="savingSettings">
                {{ savingSettings ? '保存中…' : '保存设置' }}
              </button>
            </div>
          </form>
        </div>

        <div v-else-if="tab === 'storage' && isSuper" class="panel">
          <header class="panel-head">
            <h2>图片存储</h2>
            <p>新上传的图片可以写入本地磁盘或七牛云。切换只影响之后的图片，历史图片仍按各自记录的方式访问。</p>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载存储设置…</p>
          <form v-else class="settings-form" @submit.prevent="saveStorage">
            <div class="storage-current">
              <span class="badge">{{ storageActiveLabel }}生效中</span>
              <span>配置来源：{{ storageSourceLabel }}</span>
              <span v-if="storageInfo.source === 'env'" class="hint">保存后会以网页设置为准，环境变量仅作为首次启动的默认值。</span>
            </div>

            <div class="field">
              <label for="storage-provider">新图片存放位置</label>
              <select id="storage-provider" v-model="storageForm.provider">
                <option value="local">本地磁盘</option>
                <option value="qiniu">七牛云对象存储</option>
              </select>
            </div>

            <template v-if="storageForm.provider === 'qiniu'">
              <div class="form-row">
                <div class="field">
                  <label for="storage-access">AccessKey</label>
                  <PasswordInput
                    id="storage-access"
                    v-model="storageForm.qiniuAccessKey"
                    autocomplete="off"
                    :placeholder="storageInfo.hasAccessKey ? '已配置，留空保持不变' : '七牛 AccessKey'"
                  />
                </div>
                <div class="field">
                  <label for="storage-secret">SecretKey</label>
                  <PasswordInput
                    id="storage-secret"
                    v-model="storageForm.qiniuSecretKey"
                    autocomplete="off"
                    :placeholder="storageInfo.hasSecretKey ? '已配置，留空保持不变' : '七牛 SecretKey'"
                  />
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label for="storage-bucket">空间名</label>
                  <input id="storage-bucket" v-model="storageForm.qiniuBucket" maxlength="128" placeholder="your-bucket" />
                </div>
                <div class="field">
                  <label for="storage-domain">绑定域名</label>
                  <input id="storage-domain" v-model="storageForm.qiniuDomain" maxlength="255" placeholder="https://image.example.com" />
                </div>
              </div>

              <div class="form-row">
                <div class="field">
                  <label for="storage-region">区域</label>
                  <select id="storage-region" v-model="storageForm.qiniuRegion">
                    <option value="">自动识别</option>
                    <option value="z0">z0 · 华东</option>
                    <option value="cn-east-2">cn-east-2 · 浙江</option>
                    <option value="z1">z1 · 华北</option>
                    <option value="z2">z2 · 华南</option>
                    <option value="na0">na0 · 北美</option>
                    <option value="as0">as0 · 新加坡</option>
                  </select>
                </div>
                <div class="field">
                  <label for="storage-prefix">对象前缀</label>
                  <input id="storage-prefix" v-model="storageForm.qiniuPrefix" maxlength="64" placeholder="folio" />
                </div>
              </div>

              <p class="hint">
                密钥会使用站点 <code>FOLIO_SECRET</code> 加密后保存，接口不会回显。Bucket 需要是公开读空间；域名只填主机名，可以带 <code>https://</code>。
              </p>
              <div
                v-if="storageTestResult.message"
                class="storage-test-result"
                :class="storageTestResult.ok ? 'is-ok' : 'is-error'"
              >
                {{ storageTestResult.message }}
              </div>
            </template>
            <template v-else>
              <div class="storage-local-note">
                <strong>本地磁盘</strong>
                <span>新图片保存到 <code>{{ storageInfo.localUploadDir || 'FOLIO_UPLOAD_DIR' }}</code>，通过 <code>/uploads/*</code> 访问。</span>
              </div>
            </template>

            <div class="form-actions">
              <button
                v-if="storageForm.provider === 'qiniu'"
                class="btn btn-ghost btn-sm"
                type="button"
                :disabled="testingStorage"
                @click="testStorage"
              >
                {{ testingStorage ? '测试中…' : '测试连接' }}
              </button>
              <button class="btn btn-primary btn-sm" type="submit" :disabled="savingStorage">
                {{ savingStorage ? '保存中…' : '保存并切换' }}
              </button>
            </div>
          </form>
        </div>

        <div v-else-if="tab === 'logs' && isSuper" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>操作日志</h2>
              <p>记录登录、内容、分类、图片、角色与站点设置等关键动作，按时间倒序排列。</p>
            </div>
            <form class="admin-search" @submit.prevent="searchLogs">
              <input v-model="logQuery" type="search" placeholder="搜索用户、动作或详情" />
              <button class="btn btn-ghost btn-sm" type="submit">搜索</button>
            </form>
          </header>

          <p v-if="tabLoading" class="muted loading-line">正在加载…</p>
          <EmptyState v-else-if="!logs.length" icon="clock" title="还没有操作记录" description="登录或管理内容后，动作会记录在这里。" />
          <div v-else class="admin-table admin-logs-table">
            <div class="admin-table-head">
              <span>时间</span>
              <span>用户</span>
              <span>动作</span>
              <span>详情</span>
              <span>IP</span>
            </div>
            <div v-for="entry in logs" :key="entry.id" class="admin-table-row">
              <span class="log-time">{{ logTime(entry.createdAt) }}</span>
              <span class="log-user">@{{ entry.username || '匿名' }}</span>
              <span><span class="badge">{{ entry.action }}</span></span>
              <span class="log-detail">
                {{ entry.detail || '—' }}
                <span v-if="entry.targetType" class="log-target">
                  {{ targetLabel(entry.targetType) }}#{{ entry.targetID }}
                </span>
              </span>
              <span class="log-ip">{{ entry.ip || '—' }}</span>
            </div>
          </div>

          <div v-if="logTotal > logPageSize" class="log-pager">
            <button class="btn btn-ghost btn-sm" type="button" :disabled="logPage <= 1" @click="loadLogs(logPage - 1)">上一页</button>
            <span>{{ logPage }} / {{ logPages() }} · 共 {{ logTotal }} 条</span>
            <button class="btn btn-ghost btn-sm" type="button" :disabled="logPage >= logPages()" @click="loadLogs(logPage + 1)">下一页</button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
