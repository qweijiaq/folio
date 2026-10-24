<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { register } from '../stores/auth'
import { toast } from '../stores/toast'
import RoughAccent from '../components/RoughAccent.vue'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()

const form = ref({ username: '', nickname: '', email: '', password: '' })
const submitting = ref(false)
const error = ref('')

const redirect = computed(() => {
  const value = String(route.query.redirect || '')
  return value.startsWith('/') ? value : '/'
})

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
  submitting.value = true
  try {
    await register({
      username: form.value.username.trim(),
      nickname: form.value.nickname.trim() || form.value.username.trim(),
      email: form.value.email.trim(),
      password: form.value.password
    })
    toast('账号已创建，欢迎')
    router.push(redirect.value)
  } catch (err) {
    error.value = err.message || '注册失败'
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
          <p class="auth-sub">只需要一个用户名和密码。邮箱可选，用于找回账号。</p>

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
              <label for="reg-email">邮箱（可选）</label>
              <input id="reg-email" v-model="form.email" type="email" autocomplete="email" placeholder="you@example.com" />
            </div>
            <div class="field">
              <label for="reg-password">密码</label>
              <input id="reg-password" v-model="form.password" type="password" autocomplete="new-password" placeholder="至少 6 位" />
            </div>

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
