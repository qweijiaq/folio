<script setup>
import { toasts } from '../stores/toast'
</script>

<template>
  <div class="toast-stack" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="item in toasts" :key="item.id" class="toast" :class="`toast--${item.type}`">
        {{ item.message }}
      </div>
    </TransitionGroup>
  </div>
</template>
