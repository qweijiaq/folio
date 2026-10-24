<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import { toast } from '../stores/toast'
import RoughAccent from '../components/RoughAccent.vue'
import EmailCode from '../components/EmailCode.vue'
import Icon from '../components/Icon.vue'

const router = useRouter()

const form = ref({ email: '', code: '', password: '', confirm: '' })
const emailEnabled = ref(false)
const checking = ref(true)
const submitting = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    const data = await api.get('/auth/email/status')
    emailEnabled.value = Boolean(data.enabled)
  } catch {
    emailEnabled.value = false
  } finally {
    checking.value = false
  }
})

async function submit() {
  error.value = ''
  if (!form.value.email.trim()) {
    error.value = '请填写注册时使用的邮箱'
    return
  }
  if (!form.value.code.trim()) {
    error.value = '请填写邮箱验证码'
    return
  }
  if (form.value.password.length < 6) {
    error.value = '新密码至少 6 位'
    return
  }
  if (form.value.password !== form.value.confirm) {
    error.value = '两次输入的新密码不一致'
    return
  }
  submitting.value = true
  try {
    await api.post('/auth/password/reset', {
      email: form.value.email.trim(),
      code: form.value.code.trim(),
      password: form.value.password
    })
    toast('密码已重置，请用新密码登录')
    router.push('/login')
  } catch (err) {
    error.value = err.message || '重置密码失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page auth-page">
    <div class="auth-layout container">
      <aside class="auth-art">
        <RoughAccent variant="lines" class="auth-doodle" />
        <div class="auth-art-inner">
          <span class="eyebrow">知微 · 个人知识花园</span>
          <blockquote>忘记密码没关系，<br />邮箱还记得你。</blockquote>
          <p>验证码会发送到账号绑定的邮箱，10 分钟内有效，用后即失效。</p>
        </div>
      </aside>

      <section class="auth-panel">
        <div class="auth-card">
          <span class="eyebrow">找回密码</span>
          <h1>重新设置密码</h1>
          <p class="auth-sub">用注册邮箱接收验证码，然后设置一个新密码。</p>

          <p v-if="checking" class="muted loading-line">正在检查邮件服务…</p>
          <div v-else-if="!emailEnabled" class="empty-note">
            站点还没有配置邮件服务，暂时不能通过邮箱找回密码。请联系超级管理员重置。
          </div>

          <form v-else class="auth-form" @submit.prevent="submit">
            <div class="field">
              <label for="forgot-email">邮箱</label>
              <input id="forgot-email" v-model="form.email" type="email" autocomplete="email" placeholder="you@example.com" />
            </div>
            <div class="field">
              <label>验证码</label>
              <EmailCode v-model="form.code" :email="form.email" purpose="reset" />
            </div>
            <div class="field">
              <label for="forgot-password">新密码</label>
              <input id="forgot-password" v-model="form.password" type="password" autocomplete="new-password" placeholder="至少 6 位" />
            </div>
            <div class="field">
              <label for="forgot-confirm">确认新密码</label>
              <input id="forgot-confirm" v-model="form.confirm" type="password" autocomplete="new-password" />
            </div>

            <p v-if="error" class="form-error">{{ error }}</p>

            <button class="btn btn-primary btn-block" type="submit" :disabled="submitting">
              {{ submitting ? '提交中…' : '重置密码' }}
            </button>
          </form>

          <p class="auth-switch">
            想起来了？
            <RouterLink to="/login">返回登录</RouterLink>
          </p>
        </div>

        <RouterLink to="/login" class="auth-back">
          <Icon name="arrowLeft" :size="15" />
          返回登录
        </RouterLink>
      </section>
    </div>
  </div>
</template>
