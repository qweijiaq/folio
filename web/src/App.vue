<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getToken } from './api/client'
import { auth, logout, logoutLocal, refreshInbox, refreshMe } from './stores/auth'
import { loadSite, site } from './stores/site'
import { applyTheme, toggleTheme, ui } from './stores/ui'
import { toast } from './stores/toast'
import Icon from './components/Icon.vue'
import BrandMark from './components/BrandMark.vue'
import UserAvatar from './components/UserAvatar.vue'
import ToastHost from './components/ToastHost.vue'

const router = useRouter()
const route = useRoute()
const menuOpen = ref(false)
const mobileOpen = ref(false)
const scrolled = ref(false)
const query = ref('')
const searchInput = ref(null)
let inboxTimer = null

const unreadTotal = computed(() => (auth.unread.messages || 0) + (auth.unread.notifications || 0) + (auth.unread.notices || 0))
const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))
const writeTarget = computed(() => (auth.user ? '/write' : { path: '/login', query: { redirect: '/write' } }))
const today = new Date().getFullYear()

function submitSearch() {
  const q = query.value.trim()
  if (!q) {
    nextTick(() => searchInput.value?.focus())
    return
  }
  ui.searchOpen = false
  mobileOpen.value = false
  router.push({ name: 'search', query: { q } })
}

function openSearch() {
  ui.searchOpen = !ui.searchOpen
  if (ui.searchOpen) nextTick(() => searchInput.value?.focus())
}

async function doLogout() {
  await logout()
  menuOpen.value = false
  mobileOpen.value = false
  toast('已退出登录')
  router.push('/')
}

function onDocumentClick(event) {
  if (!event.target.closest('.user-menu')) menuOpen.value = false
}

function onScroll() {
  scrolled.value = window.scrollY > 8
}

function onKeydown(event) {
  const tag = (event.target.tagName || '').toLowerCase()
  const typing = tag === 'input' || tag === 'textarea' || tag === 'select' || event.target.isContentEditable
  if (event.key === '/' && !typing) {
    event.preventDefault()
    ui.searchOpen = true
    nextTick(() => searchInput.value?.focus())
  }
  if (event.key === 'Escape') {
    ui.searchOpen = false
    mobileOpen.value = false
    menuOpen.value = false
  }
}

function onUnauthorized() {
  if (!getToken()) return
  logoutLocal()
  toast('登录状态已失效，请重新登录', 'error')
  if (route.path !== '/login') {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
  }
}

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
    mobileOpen.value = false
    ui.searchOpen = false
    refreshInbox()
  }
)

onMounted(() => {
  applyTheme()
  loadSite()
  refreshMe()
  onScroll()
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('folio:unauthorized', onUnauthorized)
  document.addEventListener('click', onDocumentClick)
  inboxTimer = window.setInterval(refreshInbox, 60000)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('folio:unauthorized', onUnauthorized)
  document.removeEventListener('click', onDocumentClick)
  if (inboxTimer) window.clearInterval(inboxTimer)
})
</script>

<template>
  <div class="app-shell">
    <header class="site-header" :class="{ 'is-scrolled': scrolled }">
      <div class="container header-inner">
        <RouterLink to="/" class="brand" aria-label="知微首页">
          <span class="brand-mark"><BrandMark /></span>
          <span class="brand-name">{{ site.name }}</span>
        </RouterLink>

        <nav class="primary-nav" aria-label="主导航">
          <RouterLink to="/articles" class="nav-link" active-class="is-active">文章</RouterLink>
          <RouterLink to="/docs" class="nav-link" active-class="is-active">文档</RouterLink>
          <RouterLink to="/articles?sort=popular" class="nav-link">热门</RouterLink>
        </nav>

        <div class="header-actions">
          <button class="icon-btn" type="button" aria-label="搜索" title="搜索（/）" @click="openSearch">
            <Icon name="search" :size="19" />
          </button>
          <button class="icon-btn" type="button" aria-label="切换主题" title="切换明暗" @click="toggleTheme">
            <Icon :name="ui.theme === 'dark' ? 'sun' : 'moon'" :size="18" />
          </button>
          <RouterLink v-if="auth.user" to="/messages" class="icon-btn badge-holder" aria-label="消息" title="消息">
            <Icon name="message" :size="19" />
            <span v-if="unreadTotal" class="nav-badge">{{ unreadTotal > 9 ? '9+' : unreadTotal }}</span>
          </RouterLink>
          <RouterLink v-if="staff" to="/admin" class="icon-btn" aria-label="管理后台" title="管理后台">
            <Icon name="settings" :size="18" />
          </RouterLink>
          <RouterLink v-if="staff" :to="writeTarget" class="btn btn-primary btn-sm write-btn">
            <Icon name="edit" :size="15" />
            写文章
          </RouterLink>

          <div v-if="auth.user" class="user-menu">
            <button class="avatar-button" type="button" aria-label="用户菜单" @click.stop="menuOpen = !menuOpen">
              <UserAvatar :username="auth.user.username" :nickname="auth.user.nickname" :size="32" />
            </button>
            <Transition name="dropdown">
              <div v-if="menuOpen" class="user-dropdown">
                <div class="dropdown-head">
                  <strong>{{ auth.user.nickname }}</strong>
                  <span>@{{ auth.user.username }}</span>
                </div>
                <RouterLink v-if="staff" to="/admin" class="dropdown-item">
                  <Icon name="settings" :size="16" /> 管理后台
                </RouterLink>
                <RouterLink :to="`/u/${auth.user.username}`" class="dropdown-item">
                  <Icon name="user" :size="16" /> 我的主页
                </RouterLink>
                <RouterLink to="/me" class="dropdown-item">
                  <Icon name="settings" :size="16" /> {{ staff ? '创作与设置' : '我的空间' }}
                </RouterLink>
                <RouterLink v-if="staff" to="/write" class="dropdown-item">
                  <Icon name="edit" :size="16" /> 写点什么
                </RouterLink>
                <RouterLink to="/messages" class="dropdown-item">
                  <Icon name="message" :size="16" /> 消息
                  <span v-if="unreadTotal" class="inline-badge">{{ unreadTotal }}</span>
                </RouterLink>
                <button class="dropdown-item dropdown-item--danger" type="button" @click="doLogout">
                  <Icon name="logout" :size="16" /> 退出登录
                </button>
              </div>
            </Transition>
          </div>
          <RouterLink v-else class="nav-login" :to="{ path: '/login', query: { redirect: route.fullPath } }">登录</RouterLink>

          <button class="icon-btn mobile-toggle" type="button" aria-label="菜单" @click.stop="mobileOpen = !mobileOpen">
            <Icon :name="mobileOpen ? 'close' : 'menu'" :size="21" />
          </button>
        </div>
      </div>

      <Transition name="search-slide">
        <form v-if="ui.searchOpen" class="header-search" @submit.prevent="submitSearch">
          <div class="container header-search-inner">
            <Icon name="search" :size="18" />
            <input
              ref="searchInput"
              v-model="query"
              type="search"
              placeholder="搜索文章、文档或标签…"
              aria-label="站内搜索"
            />
            <button type="submit" class="btn btn-primary btn-sm">搜索</button>
            <button type="button" class="icon-btn" aria-label="关闭搜索" @click="ui.searchOpen = false">
              <Icon name="close" :size="18" />
            </button>
          </div>
        </form>
      </Transition>

      <Transition name="search-slide">
        <nav v-if="mobileOpen" class="mobile-nav">
          <div class="container">
            <RouterLink to="/articles" class="mobile-link">文章</RouterLink>
            <RouterLink to="/docs" class="mobile-link">文档</RouterLink>
            <RouterLink to="/articles?sort=popular" class="mobile-link">热门</RouterLink>
            <RouterLink v-if="staff" to="/admin" class="mobile-link">管理后台</RouterLink>
            <RouterLink v-if="auth.user" to="/messages" class="mobile-link">消息</RouterLink>
            <RouterLink v-if="auth.user" to="/me" class="mobile-link">我的</RouterLink>
            <RouterLink v-else :to="{ path: '/login', query: { redirect: route.fullPath } }" class="mobile-link">登录</RouterLink>
          </div>
        </nav>
      </Transition>
    </header>

    <main class="site-main">
      <RouterView />
    </main>

    <footer class="site-footer">
      <div class="container">
        <div class="footer-top">
          <div class="footer-brand">
            <RouterLink to="/" class="brand">
              <span class="brand-mark"><BrandMark /></span>
              <span class="brand-name">{{ site.name }}</span>
            </RouterLink>
            <p>{{ site.description }}</p>
          </div>
          <div class="footer-cols">
            <div class="footer-col">
              <h4>浏览</h4>
              <RouterLink to="/articles">全部文章</RouterLink>
              <RouterLink to="/docs">知识库</RouterLink>
              <RouterLink to="/search">搜索</RouterLink>
            </div>
            <div class="footer-col">
              <h4>参与</h4>
              <RouterLink v-if="staff" to="/write">写文章</RouterLink>
              <RouterLink v-if="staff" to="/admin">管理后台</RouterLink>
              <RouterLink to="/messages">消息</RouterLink>
              <RouterLink to="/me">我的主页</RouterLink>
            </div>
          </div>
        </div>
        <div class="footer-bottom">
          <span>© {{ today }} {{ site.name }} · 由 {{ site.owner }} 维护</span>
          <span class="footer-meta">手写 CSS · 生成式封面与头像</span>
        </div>
      </div>
    </footer>

    <ToastHost />
  </div>
</template>
