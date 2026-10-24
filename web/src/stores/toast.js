import { reactive } from 'vue'

export const toasts = reactive([])
let seq = 0

export function toast(message, type = 'info') {
  const item = { id: ++seq, message, type }
  toasts.push(item)
  window.setTimeout(() => {
    const index = toasts.findIndex((t) => t.id === item.id)
    if (index >= 0) toasts.splice(index, 1)
  }, 2800)
}

export const toastError = (err) => toast(err?.message || '操作失败', 'error')
