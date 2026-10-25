import { reactive } from 'vue'

export const confirmState = reactive({
  open: false,
  title: '请确认',
  message: '',
  confirmText: '确定',
  cancelText: '取消',
  danger: false,
  resolve: null
})

export function confirmDialog(options = {}) {
  const config = typeof options === 'string' ? { message: options } : options || {}
  return new Promise((resolve) => {
    if (confirmState.open && confirmState.resolve) confirmState.resolve(false)
    Object.assign(confirmState, {
      open: true,
      title: config.title || '请确认',
      message: config.message || '',
      confirmText: config.confirmText || '确定',
      cancelText: config.cancelText || '取消',
      danger: Boolean(config.danger),
      resolve
    })
  })
}

export function resolveConfirm(value) {
  const resolver = confirmState.resolve
  confirmState.open = false
  confirmState.resolve = null
  resolver?.(Boolean(value))
}
