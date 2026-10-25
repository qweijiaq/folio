<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { confirmState, resolveConfirm } from '../stores/confirm'

const confirmButton = ref(null)
const cancelButton = ref(null)

function close(value) {
  resolveConfirm(value)
}

function onKeydown(event) {
  if (!confirmState.open) return
  if (event.key === 'Escape') {
    event.preventDefault()
    close(false)
  } else if (event.key === 'Enter') {
    const tag = (event.target.tagName || '').toLowerCase()
    if (tag === 'button') return
    event.preventDefault()
    close(true)
  } else if (event.key === 'Tab') {
    const first = cancelButton.value
    const last = confirmButton.value
    if (!first || !last) return
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
}

watch(
  () => confirmState.open,
  async (open) => {
    if (!open) return
    await nextTick()
    ;(confirmState.danger ? cancelButton.value : confirmButton.value)?.focus()
  }
)

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <Transition name="confirm">
      <div v-if="confirmState.open" class="confirm-overlay" @mousedown.self="close(false)">
        <section
          class="confirm-panel"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="confirm-title"
          aria-describedby="confirm-message"
        >
          <span class="eyebrow">确认操作</span>
          <h2 id="confirm-title">{{ confirmState.title }}</h2>
          <p id="confirm-message" class="confirm-message">{{ confirmState.message }}</p>
          <div class="confirm-actions">
            <button ref="cancelButton" class="btn btn-ghost btn-sm" type="button" @click="close(false)">
              {{ confirmState.cancelText }}
            </button>
            <button
              ref="confirmButton"
              class="btn btn-sm"
              :class="confirmState.danger ? 'confirm-danger' : 'btn-primary'"
              type="button"
              @click="close(true)"
            >
              {{ confirmState.confirmText }}
            </button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
