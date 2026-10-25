import { expect } from '@playwright/test'

export const demoPassword = 'folio1234'

export async function login(page, username = 'weijia') {
  await page.goto('/login')
  await page.fill('#login-username', username)
  await page.fill('#login-password', demoPassword)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL((url) => !url.pathname.startsWith('/login'))
  await expect(page.locator('.user-menu')).toBeVisible()
}

export async function apiLogin(request, username = 'weijia') {
  const res = await request.post('/api/auth/login', {
    data: { username, password: demoPassword }
  })
  expect(res.ok()).toBeTruthy()
  return (await res.json()).token
}

export function authHeader(token) {
  return { Authorization: `Bearer ${token}` }
}
