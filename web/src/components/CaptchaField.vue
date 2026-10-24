<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'

const captchaId = defineModel('id', { type: String, default: '' })
const captchaCode = defineModel('code', { type: String, default: '' })

const enabled = ref(false)
const image = ref('')
const loading = ref(false)

async function refresh() {
  loading.value = true
  captchaCode.value = ''
  try {
    const data = await api.get('/auth/captcha')
    enabled.value = Boolean(data.enabled)
    if (enabled.value) {
      captchaId.value = data.captchaId
      image.value = data.image
    }
  } catch {
    enabled.value = false
  } finally {
    loading.value = false
  }
}

defineExpose({ refresh, enabled })

onMounted(refresh)
</script>

<template>
  <div v-if="enabled" class="captcha-field">
    <div class="captcha-row">
      <input
        v-model="captchaCode"
        type="text"
        maxlength="4"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        placeholder="图形验证码"
      />
      <button
        class="captcha-image"
        type="button"
        :disabled="loading"
        title="看不清？换一张"
        aria-label="图形验证码，点击换一张"
        @click="refresh"
      >
        <img v-if="image" :src="image" alt="图形验证码" />
        <span v-else>换一张</span>
      </button>
    </div>
    <p class="hint">看不清？点击图片换一张</p>
  </div>
</template>
