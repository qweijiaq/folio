<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth, refreshMe } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { formatBytes, roleLabel, shortDate } from '../utils/format'
import { mergeSite } from '../stores/site'
import UserAvatar from '../components/UserAvatar.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '管理后台', description: '知微管理后台', noindex: true }))

const tab = ref(String(route.query.tab || 'overview'))
const loading = ref(true)
const tabLoading = ref(false)
const stats = ref(null)
const docs = ref([])
const users = ref([])
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
const logs = ref([])
const logTotal = ref(0)
const logPage = ref(1)
const logQuery = ref('')
const logPageSize = 30

const isSuper = computed(() => auth.user?.role === 'super_admin')
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

const tabs = computed(() => {
  const list = [
    { key: 'overview', label: '概览', icon: 'home' },
    { key: 'content', label: '内容管理', icon: 'edit' },
    { key: 'categories', label: '分类管理', icon: 'tag' },
    { key: 'images', label: '图片管理', icon: 'image' },
    { key: 'notices', label: '全站通知', icon: 'bell' }
  ]
  if (isSuper.value) {
    list.push(
      { key: 'users', label: '用户与权限', icon: 'user' },
      { key: 'settings', label: '站点设置', icon: 'settings' },
      { key: 'logs', label: '操作日志', icon: 'clock' }
    )
  }
  return list
})

const siteStats = computed(() => stats.value?.site || {})
const myStats = computed(() => stats.value?.mine || {})

async function loadStats() {
  try {
    stats.value = await api.get('/admin/stats')
  } catch (err) {
    toastError(err)
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
    else if (tab.value === 'content') await loadDocs()
    else if (tab.value === 'categories') await loadCategories()
    else if (tab.value === 'images') await loadImages()
    else if (tab.value === 'notices') await loadNotices()
    else if (tab.value === 'users') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await loadUsers()
      }
    } else if (tab.value === 'settings') {
      if (!isSuper.value) {
        tab.value = 'overview'
        await loadStats()
      } else {
        await loadSettings()
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

        <div v-else-if="tab === 'content'" class="panel">
          <header class="panel-head panel-head--row">
            <div>
              <h2>内容管理</h2>
              <p>{{ isSuper ? '全站内容都在这里，可以编辑或删除。' : '这里只显示你创建的内容。' }}</p>
            </div>
            <div class="admin-actions">
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
