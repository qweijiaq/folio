import { expect, test } from '@playwright/test'
import { apiLogin, authHeader, login } from './helpers'

test('login, open the user menu and log out', async ({ page }) => {
  await login(page)
  await page.locator('.avatar-button').click()
  await expect(page.getByRole('link', { name: /创作与设置/ })).toBeVisible()
  await page.getByRole('button', { name: '退出登录' }).click()
  await expect(page.locator('.nav-login')).toBeVisible()
})

test('write, publish and clean up an article', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const title = `E2E 文章 ${Date.now()}`
  let docID = 0
  try {
    await login(page)
    await page.goto('/write')
    await expect(page.locator('#editor-title')).toBeVisible()
    await page.fill('#editor-title', title)
    await page.fill('.editor-input', '## 小标题\n\n这是端到端测试写入的正文。')
    await page.getByRole('button', { name: '发布', exact: true }).click()
    await page.waitForURL(/\/articles\/\d+$/)
    docID = Number(new URL(page.url()).pathname.split('/').pop())
    await expect(page.locator('h1').first()).toContainText(title)
    await expect(page.getByText('这是端到端测试写入的正文。', { exact: true })).toBeVisible()
  } finally {
    if (docID) {
      const res = await request.delete(`/api/docs/${docID}`, { headers: authHeader(ownerToken) })
      expect(res.ok()).toBeTruthy()
    }
  }
})

test('request access to a private document', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const title = `E2E 私密文档 ${Date.now()}`
  const created = await request.post('/api/docs', {
    headers: authHeader(ownerToken),
    data: { kind: 'doc', title, content: '私密文档的正文。', status: 'published', private: true }
  })
  expect(created.status()).toBe(201)
  const docID = (await created.json()).doc.id
  try {
    await login(page, 'chen')
    await page.goto(`/docs/${docID}`)
    await expect(page.locator('.private-access-card')).toBeVisible()
    await page.locator('.private-access-note textarea').fill('希望阅读这篇文档，谢谢。')
    await page.getByRole('button', { name: '申请阅读' }).click()
    await expect(page.getByText('申请已提交', { exact: true })).toBeVisible()
  } finally {
    const res = await request.delete(`/api/docs/${docID}`, { headers: authHeader(ownerToken) })
    expect(res.ok()).toBeTruthy()
  }
})

test('AI asks for a user supplied key before answering', async ({ page }) => {
  await login(page, 'chen')
  await page.goto('/ask')
  const input = page.locator('textarea')
  await expect(input).toBeVisible()
  await input.fill('这个站点怎么发布一篇文章？')
  const ask = page.getByRole('button', { name: '提问', exact: true })
  await expect(ask).toBeEnabled()
  await ask.click()
  await expect(page.getByText(/请先填写你自己的.*API Key/)).toBeVisible()
})

test('creates category and series in the editor and collapses settings', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const suffix = Date.now()
  const categoryName = `分类${String(suffix).slice(-6)}`
  const seriesTitle = `E2E 合集 ${suffix}`
  try {
    await login(page)
    await page.goto('/write')
    await expect(page.locator('#editor-title')).toBeVisible()
    expect(await page.locator('select').count()).toBe(0)

    const categoryField = page.locator('.field').filter({ has: page.locator('#editor-category') })
    await categoryField.getByRole('button', { name: '+ 新建' }).click()
    await categoryField.locator('input[placeholder="新分类名称"]').fill(categoryName)
    await categoryField.getByRole('button', { name: '添加' }).click()
    await expect(page.locator('#editor-category')).toContainText(categoryName)

    const seriesField = page.locator('.field').filter({ has: page.locator('#editor-series') })
    await seriesField.getByRole('button', { name: '+ 新建' }).click()
    await seriesField.locator('input[placeholder="新合集名称"]').fill(seriesTitle)
    await seriesField.getByRole('button', { name: '添加' }).click()
    await expect(page.locator('#editor-series')).toContainText(seriesTitle)

    await page.locator('#editor-category').click()
    await expect(page.locator('.select-box-menu')).toBeVisible()
    await page.locator('.select-box-option').filter({ hasText: categoryName }).click()
    await expect(page.locator('.select-box-menu')).toHaveCount(0)

    await page.getByRole('button', { name: /收起设置/ }).click()
    await expect(page.locator('#editor-title')).toHaveCount(0)
    await page.getByRole('button', { name: /展开设置/ }).click()
    await expect(page.locator('#editor-title')).toBeVisible()
  } finally {
    const cats = await request.get('/api/categories')
    for (const item of (await cats.json()).items || []) {
      if (item.name === categoryName) {
        await request.delete(`/api/categories/${item.id}`, { headers: authHeader(ownerToken) })
      }
    }
    const series = await request.get('/api/series')
    for (const item of (await series.json()).items || []) {
      if (item.title === seriesTitle) {
        await request.delete(`/api/series/${item.id}`, { headers: authHeader(ownerToken) })
      }
    }
  }
})

test('asks AI with a document context', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const title = `E2E AI 上下文 ${Date.now()}`
  const created = await request.post('/api/docs', {
    headers: authHeader(ownerToken),
    data: { kind: 'article', title, content: '这是给 AI 提问用的上下文正文。', status: 'published' }
  })
  expect(created.status()).toBe(201)
  const doc = (await created.json()).doc
  try {
    await login(page, 'chen')
    await page.goto(`/ask?doc=${doc.id}`)
    await expect(page.locator('.ask-context')).toContainText(title)
    await page.getByRole('button', { name: '取消' }).click()
    await expect(page.locator('.ask-context')).toHaveCount(0)
  } finally {
    const res = await request.delete(`/api/docs/${doc.id}`, { headers: authHeader(ownerToken) })
    expect(res.ok()).toBeTruthy()
  }
})

test('adjusts reading tools and focus mode', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const title = `E2E 阅读体验 ${Date.now()}`
  const created = await request.post('/api/docs', {
    headers: authHeader(ownerToken),
    data: { kind: 'article', title, content: '# 阅读测试\n\n这是阅读体验测试正文。', status: 'published' }
  })
  expect(created.status()).toBe(201)
  const doc = (await created.json()).doc
  try {
    await page.goto(`/articles/${doc.id}`)
    await expect(page.locator('.reading-tools')).toBeVisible()
    await expect(page.locator('.heading-anchor').first()).toBeAttached()
    const before = await page.locator('.prose').evaluate((el) => getComputedStyle(el).fontSize)
    await page.getByRole('button', { name: '增大字号' }).click()
    const after = await page.locator('.prose').evaluate((el) => getComputedStyle(el).fontSize)
    expect(after).not.toBe(before)
    await page.getByRole('button', { name: '专注' }).click()
    await expect(page.locator('body')).toHaveClass(/reading-focus/)
    await page.keyboard.press('Escape')
    await expect(page.locator('body')).not.toHaveClass(/reading-focus/)
    await page.emulateMedia({ media: 'print' })
    expect(await page.locator('.skip-link').evaluate((el) => getComputedStyle(el).display)).toBe('none')
    expect(await page.locator('.print-watermark').evaluate((el) => getComputedStyle(el).display)).toBe('flex')
    await expect(page.locator('.print-watermark')).toContainText('原创')
    await page.emulateMedia({ media: 'screen' })
  } finally {
    const res = await request.delete(`/api/docs/${doc.id}`, { headers: authHeader(ownerToken) })
    expect(res.ok()).toBeTruthy()
  }
})

test('uses the site confirmation dialog for destructive actions', async ({ page, request }) => {
  const ownerToken = await apiLogin(request)
  const title = `E2E 确认弹窗 ${Date.now()}`
  const created = await request.post('/api/docs', {
    headers: authHeader(ownerToken),
    data: { kind: 'article', title, content: '确认弹窗测试正文。', status: 'published' }
  })
  expect(created.status()).toBe(201)
  const doc = (await created.json()).doc
  try {
    await login(page, 'chen')
    await page.goto(`/articles/${doc.id}`)
    await page.locator('.comment-form textarea').fill('E2E 待删除评论')
    await page.locator('.comment-form').getByRole('button', { name: '发布' }).click()
    const comment = page.locator('.comment').filter({ hasText: 'E2E 待删除评论' }).first()
    await expect(comment).toBeVisible()
    await comment.getByRole('button', { name: '删除' }).click()
    await expect(page.locator('.confirm-panel')).toBeVisible()
    await page.locator('.confirm-panel').getByRole('button', { name: '取消' }).click()
    await expect(comment).toBeVisible()
    await comment.getByRole('button', { name: '删除' }).click()
    await page.locator('.confirm-panel .confirm-danger').click()
    await expect(comment).toHaveCount(0)
  } finally {
    const res = await request.delete(`/api/docs/${doc.id}`, { headers: authHeader(ownerToken) })
    expect(res.ok()).toBeTruthy()
  }
})

test('opens AI summary settings in the admin panel', async ({ page }) => {
  await login(page)
  await page.goto('/admin?tab=ai')
  await expect(page.getByRole('heading', { name: 'AI 摘要' })).toBeVisible()
  await expect(page.locator('#ai-summary-provider')).toBeVisible()
  await page.locator('#ai-summary-provider').click()
  await expect(page.locator('.select-box-menu')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.locator('.select-box-menu')).toHaveCount(0)
})

test('shows the current login session in account security', async ({ page }) => {
  await login(page)
  await page.goto('/me?tab=security')
  await expect(page.getByRole('heading', { name: '登录设备' })).toBeVisible()
  await expect(page.locator('.session-row').first()).toBeVisible()
  await expect(page.getByText('当前设备', { exact: true })).toBeVisible()
})
