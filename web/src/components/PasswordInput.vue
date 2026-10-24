<script setup>
import { ref } from 'vue'
import Icon from './Icon.vue'

defineOptions({ inheritAttrs: false })

const model = defineModel({ type: String, default: '' })
const visible = ref(false)
</script>

<template>
  <div class="password-input">
    <input v-bind="$attrs" v-model="model" :type="visible ? 'text' : 'password'" />
    <button
      class="password-toggle"
      type="button"
      :aria-label="visible ? '隐藏密码' : '显示密码'"
      :title="visible ? '隐藏密码' : '显示密码'"
      @click="visible = !visible"
    >
      <Icon :name="visible ? 'eyeOff' : 'eye'" :size="16" />
    </button>
  </div>
</template>
