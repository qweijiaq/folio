import { reactive } from 'vue'
import { api, getToken, setToken } from '../api/client'

export const auth = reactive({
  user: null,
  ready: false,
  unread: { messages: 0, notifications: 0, notices: 0 }
})

export function loggedIn() {
  return Boolean(getToken())
}

export async function refreshMe() {
  if (!getToken()) {
    auth.user = null
    auth.ready = true
    return null
  }
  try {
    const data = await api.get('/me')
    auth.user = data.user
    await refreshInbox()
  } catch (err) {
    if (err?.status === 401) setToken('')
    auth.user = null
  } finally {
    auth.ready = true
  }
  return auth.user
}

export async function refreshInbox() {
  if (!getToken()) return
  try {
    auth.unread = await api.get('/inbox')
  } catch {
    /* ignore */
  }
}

export async function login(username, password, captcha = {}) {
  const data = await api.post('/auth/login', { username, password, ...captcha })
  setToken(data.token)
  auth.user = data.user
  await refreshInbox()
  return data.user
}

export async function loginWithEmail(email, code) {
  const data = await api.post('/auth/email/login', { email, code })
  setToken(data.token)
  auth.user = data.user
  await refreshInbox()
  return data.user
}

export async function loginWithQQTicket(ticket) {
  const data = await api.post('/auth/qq/exchange', { ticket })
  setToken(data.token)
  auth.user = data.user
  await refreshInbox()
  return data.user
}

export async function register(payload) {
  const data = await api.post('/auth/register', payload)
  setToken(data.token)
  auth.user = data.user
  await refreshInbox()
  return data.user
}

export async function logout() {
  try {
    await api.post('/auth/logout')
  } catch {
    /* 服务端不可用时仍然完成本地登出 */
  }
  logoutLocal()
}

export function logoutLocal() {
  setToken('')
  auth.user = null
  auth.unread = { messages: 0, notifications: 0, notices: 0 }
}
