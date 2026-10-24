<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '../stores/auth'
import { toast } from '../stores/toast'
import RoughAccent from '../components/RoughAccent.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

const username = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref('')
const isDev = import.meta.env.DEV

const redirect = computed(() => {
  const value = String(route.query.redirect || '')
  return value.startsWith('/') ? value : '/'
})

async function submit() {
  error.value = ''
  if (!username.value.trim() || !password.value) {
    error.value = '请填写用户名和密码'
    return
  }
  submitting.value = true
  try {
    await login(username.value.trim(), password.value)
    toast('欢迎回来')
    router.push(redirect.value)
  } catch (err) {
    error.value = err.message || '登录失败'
  } finally {
    submitting.value = false
  }
}

function fillDemo() {
  username.value = 'weijia'
  password.value = 'folio1234'
}
</script>

<template>
  <div class="page auth-page">
    <div class="auth-layout container">
      <aside class="auth-art">
        <RoughAccent variant="circle" class="auth-doodle" />
        <div class="auth-art-inner">
          <span class="eyebrow">知微 · 个人知识花园</span>
          <blockquote>写作不是为了被看见，<br />而是为了看清自己。</blockquote>
          <p>文章与文档共用一套账号与互动。登录后可以评论、收藏、关注和私信。</p>
        </div>
      </aside>

      <section class="auth-panel">
        <div class="auth-card">
          <span class="eyebrow">登录</span>
          <h1>继续阅读</h1>
          <p class="auth-sub">用用户名和密码登录你的账号。</p>

          <form class="auth-form" @submit.prevent="submit">
            <div class="field">
              <label for="login-username">用户名</label>
              <input id="login-username" v-model="username" type="text" autocomplete="username" placeholder="your_name" />
            </div>
            <div class="field">
              <label for="login-password">密码</label>
              <input id="login-password" v-model="password" type="password" autocomplete="current-password" placeholder="至少 6 位" />
            </div>

            <p v-if="error" class="form-error">{{ error }}</p>

            <button class="btn btn-primary btn-block" type="submit" :disabled="submitting">
              {{ submitting ? '登录中…' : '登录' }}
            </button>
          </form>

          <div v-if="isDev" class="demo-box">
            <span>演示账号：weijia / folio1234</span>
            <button type="button" class="text-link" @click="fillDemo">填入</button>
          </div>

          <p class="auth-switch">
            还没有账号？
            <RouterLink :to="{ path: '/register', query: route.query }">注册一个</RouterLink>
          </p>
        </div>

        <RouterLink to="/" class="auth-back">
          <Icon name="arrowLeft" :size="15" />
          返回首页
        </RouterLink>
      </section>
    </div>
  </div>
</template>
