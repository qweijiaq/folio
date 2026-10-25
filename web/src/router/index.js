import { createRouter, createWebHistory } from 'vue-router'
import { auth, loggedIn, refreshMe } from '../stores/auth'
import { toast } from '../stores/toast'

const routes = [
  { path: '/', name: 'home', component: () => import('../views/Home.vue'), meta: { title: '首页' } },
  { path: '/articles', name: 'articles', component: () => import('../views/Articles.vue'), meta: { title: '文章' } },
  { path: '/articles/:id', name: 'article', component: () => import('../views/ArticleDetail.vue'), meta: { title: '文章' } },
  { path: '/series', name: 'series', component: () => import('../views/SeriesList.vue'), meta: { title: '合集' } },
  { path: '/series/:id', name: 'series-detail', component: () => import('../views/SeriesDetail.vue'), meta: { title: '合集' } },
  { path: '/docs', name: 'docs', component: () => import('../views/Docs.vue'), meta: { title: '文档' } },
  { path: '/docs/:id', name: 'doc', component: () => import('../views/DocDetail.vue'), meta: { title: '文档' } },
  { path: '/search', name: 'search', component: () => import('../views/Search.vue'), meta: { title: '搜索' } },
  { path: '/about', name: 'about', component: () => import('../views/About.vue'), meta: { title: '关于' } },
  { path: '/u/:username', name: 'profile', component: () => import('../views/Profile.vue'), meta: { title: '主页' } },
  { path: '/login', name: 'login', component: () => import('../views/Login.vue'), meta: { title: '登录', guest: true } },
  { path: '/register', name: 'register', component: () => import('../views/Register.vue'), meta: { title: '注册', guest: true } },
  { path: '/forgot', name: 'forgot', component: () => import('../views/Forgot.vue'), meta: { title: '找回密码', guest: true } },
  {
    path: '/write/:id?',
    name: 'write',
    component: () => import('../views/Write.vue'),
    meta: { title: '写作', auth: true, staff: true }
  },
  { path: '/messages', name: 'messages', component: () => import('../views/Messages.vue'), meta: { title: '消息', auth: true } },
  { path: '/ask', name: 'ask', component: () => import('../views/Ask.vue'), meta: { title: '问 AI', auth: true } },
  { path: '/me', name: 'me', component: () => import('../views/Me.vue'), meta: { title: '我的', auth: true } },
  {
    path: '/admin',
    name: 'admin',
    component: () => import('../views/Admin.vue'),
    meta: { title: '管理后台', auth: true, staff: true }
  },
  { path: '/:pathMatch(.*)*', name: 'notfound', component: () => import('../views/NotFound.vue'), meta: { title: '未找到' } }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, saved) {
    if (saved) return { ...saved, behavior: 'auto' }
    if (to.hash) return { el: to.hash, top: 96, behavior: 'smooth' }
    // 同一路径只改 query（tab、筛选、分页参数）时保留当前位置，
    // 由页面自己决定是否需要滚动；跨页面才回到顶部。
    if (to.path === from.path) return false
    return { top: 0, behavior: 'auto' }
  }
})

const STAFF_ROLES = ['super_admin', 'admin']

router.beforeEach(async (to) => {
  if (to.meta.guest && loggedIn()) {
    const user = auth.user || (await refreshMe())
    if (user) return { path: '/' }
  }
  if (!to.meta.auth && !to.meta.staff) return true
  if (!loggedIn()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  let user = auth.user
  if (to.meta.staff) {
    user = await refreshMe()
  } else if (!user) {
    user = await refreshMe()
  }
  if (!user) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (to.meta.staff && !STAFF_ROLES.includes(user.role)) {
    toast(to.name === 'admin' ? '需要管理员权限' : '只有管理员可以发布内容')
    return { path: '/' }
  }
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title ? `${to.meta.title} · 知微` : '知微'
})

export default router
