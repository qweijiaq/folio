<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { register } from '../stores/auth'
import { api } from '../api/client'
import { toast } from '../stores/toast'
import RoughAccent from '../components/RoughAccent.vue'
import EmailCode from '../components/EmailCode.vue'
import CaptchaField from '../components/CaptchaField.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

const form = ref({ username: '', nickname: '', email: '', password: '', code: '' })
const submitting = ref(false)
const error = ref('')
const emailEnabled = ref(false)
const captchaEnabled = ref(false)
const captchaId = ref('')
const captchaCode = ref('')
const captchaRef = ref(null)

const redirect = computed(() => {
  const value = String(route.query.redirect || '')
  return value.startsWith('/') ? value : '/'
})

onMounted(async () => {
  try {
    const data = await api.get('/auth/email/status')
    emailEnabled.value = Boolean(data.requireRegister)
    captchaEnabled.value = Boolean(data.captcha)
  } catch {
    emailEnabled.value = false
  }
})

function validEmail(value) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

async function submit() {
  error.value = ''
  if (!/^[A-Za-z0-9_]{2,20}$/.test(form.value.username)) {
    error.value = '用户名需为 2-20 位字母、数字或下划线'
    return
  }
  if (form.value.password.length < 6) {
    error.value = '密码至少 6 位'
    return
  }
  const email = form.value.email.trim()
  if (emailEnabled.value && !email) {
    error.value = '请填写邮箱并获取验证码'
    return
  }
  if (email && !validEmail(email)) {
    error.value = '邮箱格式不正确'
    return
  }
  if (emailEnabled.value && !form.value.code.trim()) {
    error.value = '请填写邮箱验证码'
    return
  }
  submitting.value = true
  try {
    await register({
      username: form.value.username.trim(),
      nickname: form.value.nickname.trim() || form.value.username.trim(),
      email,
      password: form.value.password,
      code: form.value.code.trim(),
      captchaId: captchaId.value,
      captchaCode: captchaCode.value
    })
    toast('账号已创建，欢迎')
    router.push(redirect.value)
  } catch (err) {
    error.value = err.message || '注册失败'
    captchaRef.value?.refresh()
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page auth-page">
    <div class="auth-layout container">
      <aside class="auth-art">
        <RoughAccent variant="squiggle" class="auth-doodle auth-doodle--squiggle" />
        <div class="auth-art-inner">
          <span class="eyebrow">知微 · 个人知识花园</span>
          <blockquote>先写下第一句，<br />再慢慢长成一片森林。</blockquote>
          <p>注册后可以评论、收藏、关注和私信，头像会自动生成。发布文章与文档需要管理员权限。</p>
        </div>
      </aside>

      <section class="auth-panel">
        <div class="auth-card">
          <span class="eyebrow">注册</span>
          <h1>创建账号</h1>
          <p class="auth-sub">
            {{
              emailEnabled
                ? '填写邮箱并验证，之后可以用验证码登录或找回密码。'
                : '只需要一个用户名和密码。邮箱可选，用于找回账号。'
            }}
          </p>

          <form class="auth-form" @submit.prevent="submit">
            <div class="form-row">
              <div class="field">
                <label for="reg-username">用户名</label>
                <input id="reg-username" v-model="form.username" type="text" autocomplete="username" placeholder="字母、数字或下划线" />
              </div>
              <div class="field">
                <label for="reg-nickname">昵称</label>
                <input id="reg-nickname" v-model="form.nickname" type="text" autocomplete="nickname" placeholder="显示在文章旁" />
              </div>
            </div>
            <div class="field">
              <label for="reg-email">{{ emailEnabled ? '邮箱' : '邮箱（可选）' }}</label>
              <input id="reg-email" v-model="form.email" type="email" autocomplete="email" placeholder="you@example.com" />
            </div>
            <div v-if="emailEnabled" class="field">
              <label>邮箱验证码</label>
              <EmailCode v-model="form.code" :email="form.email" purpose="register" />
            </div>
            <div class="field">
              <label for="reg-password">密码</label>
              <input id="reg-password" v-model="form.password" type="password" autocomplete="new-password" placeholder="至少 6 位" />
            </div>
            <CaptchaField v-if="captchaEnabled && !emailEnabled" ref="captchaRef" v-model:id="captchaId" v-model:code="captchaCode" />

            <p v-if="error" class="form-error">{{ error }}</p>

            <button class="btn btn-primary btn-block" type="submit" :disabled="submitting">
              {{ submitting ? '创建中…' : '创建账号' }}
            </button>
          </form>

          <p class="auth-switch">
            已经有账号？
            <RouterLink :to="{ path: '/login', query: route.query }">直接登录</RouterLink>
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
