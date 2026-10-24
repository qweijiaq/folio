<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login, loginWithEmail, loginWithQQTicket } from '../stores/auth'
import { api } from '../api/client'
import { site } from '../stores/site'
import { toast } from '../stores/toast'
import RoughAccent from '../components/RoughAccent.vue'
import EmailCode from '../components/EmailCode.vue'
import CaptchaField from '../components/CaptchaField.vue'
import PasswordInput from '../components/PasswordInput.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '登录', description: '登录知微', noindex: true }))

const mode = ref('password')
const username = ref('')
const password = ref('')
const email = ref('')
const emailCode = ref('')
const emailEnabled = ref(false)
const qqEnabled = ref(false)
const qqLoading = ref(false)
const captchaId = ref('')
const captchaCode = ref('')
const captchaRef = ref(null)
const submitting = ref(false)
const error = ref('')
const isDev = import.meta.env.DEV

const redirect = computed(() => {
  const value = String(route.query.redirect || '')
  return value.startsWith('/') ? value : '/'
})

async function finishQQLogin(ticket) {
  submitting.value = true
  try {
    await loginWithQQTicket(ticket)
    toast('QQ 登录成功')
    router.replace('/')
  } catch (err) {
    error.value = err.message || 'QQ 登录失败'
    router.replace({ path: '/login' })
  } finally {
    submitting.value = false
  }
}

async function startQQ() {
  qqLoading.value = true
  error.value = ''
  try {
    const data = await api.get('/auth/qq/url')
    window.location.href = data.url
  } catch (err) {
    error.value = err.message || 'QQ 登录暂不可用'
    qqLoading.value = false
  }
}

onMounted(async () => {
  const qqError = String(route.query.qq_error || '')
  if (qqError) {
    error.value = qqError
    router.replace({ path: '/login' })
  }
  const ticket = String(route.query.qq_ticket || '')
  if (ticket) {
    await finishQQLogin(ticket)
  }
  try {
    const data = await api.get('/auth/email/status')
    emailEnabled.value = Boolean(data.enabled)
  } catch {
    emailEnabled.value = false
  }
  try {
    const data = await api.get('/auth/qq/status')
    qqEnabled.value = Boolean(data.enabled)
  } catch {
    qqEnabled.value = false
  }
})

function switchMode(value) {
  if (mode.value === value) return
  mode.value = value
  error.value = ''
}

async function submit() {
  error.value = ''
  if (mode.value === 'email') {
    if (!email.value.trim() || !emailCode.value.trim()) {
      error.value = '请填写邮箱和验证码'
      return
    }
    submitting.value = true
    try {
      await loginWithEmail(email.value.trim(), emailCode.value.trim())
      toast('欢迎回来')
      router.push(redirect.value)
    } catch (err) {
      error.value = err.message || '登录失败'
    } finally {
      submitting.value = false
    }
    return
  }

  if (!username.value.trim() || !password.value) {
    error.value = '请填写用户名和密码'
    return
  }
  submitting.value = true
  try {
    await login(username.value.trim(), password.value, {
      captchaId: captchaId.value,
      captchaCode: captchaCode.value
    })
    toast('欢迎回来')
    router.push(redirect.value)
  } catch (err) {
    error.value = err.message || '登录失败'
    captchaRef.value?.refresh()
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
          <p class="auth-sub">
            {{ mode === 'email' ? '用邮箱验证码登录，不用记密码。' : '用用户名和密码登录你的账号。' }}
          </p>

          <div v-if="emailEnabled" class="auth-modes">
            <button type="button" :class="{ active: mode === 'password' }" @click="switchMode('password')">密码登录</button>
            <button type="button" :class="{ active: mode === 'email' }" @click="switchMode('email')">邮箱验证码</button>
          </div>

          <form class="auth-form" @submit.prevent="submit">
            <template v-if="mode === 'password'">
              <div class="field">
                <label for="login-username">用户名</label>
                <input id="login-username" v-model="username" type="text" autocomplete="username" placeholder="your_name" />
              </div>
              <div class="field">
                <label for="login-password">密码</label>
                <PasswordInput id="login-password" v-model="password" autocomplete="current-password" placeholder="至少 6 位" />
              </div>
              <CaptchaField ref="captchaRef" v-model:id="captchaId" v-model:code="captchaCode" />
              <p v-if="emailEnabled" class="auth-aux">
                <RouterLink to="/forgot">忘记密码？</RouterLink>
              </p>
            </template>
            <template v-else>
              <div class="field">
                <label for="login-email">邮箱</label>
                <input id="login-email" v-model="email" type="email" autocomplete="email" placeholder="you@example.com" />
              </div>
              <div class="field">
                <label>验证码</label>
                <EmailCode v-model="emailCode" :email="email" purpose="login" />
              </div>
            </template>

            <p v-if="error" class="form-error">{{ error }}</p>

            <button class="btn btn-primary btn-block" type="submit" :disabled="submitting">
              {{ submitting ? '登录中…' : mode === 'email' ? '验证码登录' : '登录' }}
            </button>
          </form>

          <template v-if="qqEnabled">
            <div class="auth-divider"><span>或</span></div>
            <button class="btn btn-ghost btn-block qq-login" type="button" :disabled="qqLoading || submitting" @click="startQQ">
              <span class="qq-mark">Q</span>
              {{ qqLoading ? '正在跳转…' : '使用 QQ 登录' }}
            </button>
          </template>

          <div v-if="isDev && mode === 'password'" class="demo-box">
            <span>演示账号：weijia / folio1234</span>
            <button type="button" class="text-link" @click="fillDemo">填入</button>
          </div>

          <p v-if="site.allowRegister" class="auth-switch">
            还没有账号？
            <RouterLink :to="{ path: '/register', query: route.query }">注册一个</RouterLink>
          </p>
          <p v-else class="auth-switch auth-switch--muted">站点暂时关闭了注册，请使用已有账号登录。</p>
        </div>

        <RouterLink to="/" class="auth-back">
          <Icon name="arrowLeft" :size="15" />
          返回首页
        </RouterLink>
      </section>
    </div>
  </div>
</template>
