<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { auth } from '../stores/auth'
import { ui } from '../stores/ui'
import Icon from './Icon.vue'

const router = useRouter()
const input = ref(null)
const query = ref('')
const active = ref(0)

const staff = computed(() => ['super_admin', 'admin'].includes(auth.user?.role))

const quickLinks = computed(() => {
  const list = [
    { key: 'articles', label: '文章', hint: '浏览全部文章', icon: 'edit', to: { name: 'articles' }, keywords: 'post blog 写作 列表' },
    { key: 'docs', label: '文档', hint: '进入知识库', icon: 'book', to: { name: 'docs' }, keywords: 'docs 知识库 手册' },
    { key: 'about', label: '关于', hint: '了解站点全部功能', icon: 'info', to: { name: 'about' }, keywords: 'about 关于 功能 帮助 介绍' },
    { key: 'popular', label: '热门', hint: '按阅读量查看文章', icon: 'star', to: { name: 'articles', query: { sort: 'popular' } }, keywords: 'popular 最热' }
  ]
  if (auth.user) {
    list.push(
      { key: 'messages', label: '消息', hint: '私信与通知', icon: 'message', to: { name: 'messages' }, keywords: 'message 私信 通知' },
      { key: 'me', label: '我的空间', hint: '资料、收藏与历史', icon: 'user', to: { name: 'me' }, keywords: 'me 我的 收藏' }
    )
  }
  if (staff.value) {
    list.push(
      { key: 'write', label: '写文章', hint: '打开编辑器', icon: 'feather', to: { path: '/write' }, keywords: 'write 写作 新建' },
      { key: 'admin', label: '管理后台', hint: '内容、图片与站点设置', icon: 'settings', to: { name: 'admin' }, keywords: 'admin 后台 设置' }
    )
  }
  return list
})

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  const links = quickLinks.value.filter((item) => {
    if (!q) return true
    return `${item.label} ${item.hint} ${item.keywords}`.toLowerCase().includes(q)
  })
  const items = [...links]
  if (q) {
    items.unshift({
      key: 'search',
      label: `搜索「${query.value.trim()}」`,
      hint: '在文章与文档中搜索',
      icon: 'search',
      to: { name: 'search', query: { q: query.value.trim() } }
    })
  }
  return items.slice(0, 7)
})

function close() {
  ui.searchOpen = false
}

function jump(item) {
  if (!item) return
  close()
  router.push(item.to)
}

function onKeydown(event) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    if (!results.value.length) return
    active.value = (active.value + 1) % results.value.length
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (!results.value.length) return
    active.value = (active.value - 1 + results.value.length) % results.value.length
  } else if (event.key === 'Enter') {
    event.preventDefault()
    jump(results.value[active.value] || results.value[0])
  } else if (event.key === 'Escape') {
    event.preventDefault()
    close()
  }
}

watch(
  () => ui.searchOpen,
  (open) => {
    document.body.style.overflow = open ? 'hidden' : ''
    if (open) {
      active.value = 0
      nextTick(() => input.value?.focus())
    } else {
      query.value = ''
    }
  }
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
})

watch(query, () => {
  active.value = 0
})
</script>

<template>
  <Teleport to="body">
    <Transition name="palette">
      <div v-if="ui.searchOpen" class="command-overlay" @mousedown.self="close">
        <div class="command-panel" role="dialog" aria-modal="true" aria-label="快速搜索与导航">
          <div class="command-input-row">
            <Icon name="search" :size="19" />
            <input
              ref="input"
              v-model="query"
              type="text"
              autocomplete="off"
              spellcheck="false"
              placeholder="搜索文章、文档，或跳转到…"
              aria-label="搜索或跳转"
              @keydown="onKeydown"
            />
            <kbd>esc</kbd>
          </div>

          <div class="command-list" role="listbox" aria-label="结果">
            <button
              v-for="(item, index) in results"
              :key="item.key"
              type="button"
              class="command-item"
              :class="{ active: index === active }"
              role="option"
              :aria-selected="index === active"
              @mouseenter="active = index"
              @click="jump(item)"
            >
              <span class="command-icon"><Icon :name="item.icon" :size="16" /></span>
              <span class="command-text">
                <strong>{{ item.label }}</strong>
                <small>{{ item.hint }}</small>
              </span>
              <kbd v-if="index === active">↵</kbd>
            </button>
            <p v-if="!results.length" class="command-empty">
              没有匹配的入口。按 Enter 直接搜索「{{ query.trim() }}」。
            </p>
          </div>

          <footer class="command-foot">
            <span><kbd>↑</kbd><kbd>↓</kbd> 选择</span>
            <span><kbd>↵</kbd> 打开</span>
            <span><kbd>esc</kbd> 关闭</span>
            <span class="command-brand">知微 · 快速导航</span>
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
