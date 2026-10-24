<script setup>
import { onBeforeUnmount, ref, watch } from 'vue'
import { api } from '../api/client'
import { toast, toastError } from '../stores/toast'
import CaptchaField from './CaptchaField.vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  email: { type: String, default: '' },
  purpose: { type: String, required: true },
  disabled: { type: Boolean, default: false }
})

const emit = defineEmits(['update:modelValue', 'sent'])
const sending = ref(false)
const cooldown = ref(0)
const debugCode = ref('')
const captchaId = ref('')
const captchaCode = ref('')
const captchaRef = ref(null)
let timer = null

watch(
  () => props.email,
  () => {
    debugCode.value = ''
  }
)

function startCooldown() {
  cooldown.value = 60
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0) {
      clearInterval(timer)
      timer = null
    }
  }, 1000)
}

async function send() {
  const email = props.email.trim()
  if (!email) {
    toast('请先填写邮箱')
    return
  }
  sending.value = true
  try {
    const data = await api.post('/auth/email/code', {
      email,
      purpose: props.purpose,
      captchaId: captchaId.value,
      captchaCode: captchaCode.value
    })
    startCooldown()
    emit('sent', data)
    if (data.debugCode) {
      debugCode.value = data.debugCode
      emit('update:modelValue', data.debugCode)
      toast('开发模式：验证码已自动填入')
    } else {
      toast('验证码已发送，请查看邮箱')
    }
  } catch (err) {
    toastError(err)
  } finally {
    sending.value = false
    captchaRef.value?.refresh()
  }
}

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="email-code">
    <div class="email-code-row">
      <input
        :value="modelValue"
        type="text"
        inputmode="numeric"
        maxlength="6"
        autocomplete="one-time-code"
        placeholder="6 位验证码"
        :disabled="disabled"
        @input="emit('update:modelValue', $event.target.value)"
      />
      <button
        class="btn btn-ghost btn-sm"
        type="button"
        :disabled="disabled || sending || cooldown > 0 || !email.trim()"
        @click="send"
      >
        {{ sending ? '发送中…' : cooldown > 0 ? `${cooldown}s 后重发` : '发送验证码' }}
      </button>
    </div>
    <CaptchaField ref="captchaRef" v-model:id="captchaId" v-model:code="captchaCode" />
    <p v-if="debugCode" class="email-code-debug">
      开发模式验证码 <strong>{{ debugCode }}</strong>，已自动填入
    </p>
  </div>
</template>
