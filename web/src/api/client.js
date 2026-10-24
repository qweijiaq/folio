const TOKEN_KEY = 'folio.token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  constructor(message, status = 0) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request(path, { method = 'GET', body, query } = {}) {
  const url = new URL('/api' + path, window.location.origin)
  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        url.searchParams.set(key, value)
      }
    })
  }
  const headers = { Accept: 'application/json' }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  let res
  try {
    res = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body)
    })
  } catch {
    throw new ApiError('网络连接失败，请确认服务已启动', 0)
  }

  if (!res.ok) {
    let message = '请求失败'
    try {
      const data = await res.json()
      message = data.error || data.message || message
    } catch {
      /* ignore */
    }
    if (res.status === 401 && token) {
      window.dispatchEvent(new Event('folio:unauthorized'))
    }
    throw new ApiError(message, res.status)
  }

  if (res.status === 204) return null
  const type = res.headers.get('content-type') || ''
  return type.includes('application/json') ? res.json() : res.text()
}

export const api = {
  get: (path, query) => request(path, { query }),
  post: (path, body) => request(path, { method: 'POST', body }),
  put: (path, body) => request(path, { method: 'PUT', body }),
  del: (path) => request(path, { method: 'DELETE' }),
  uploadImage: (file) => uploadImage(file)
}

async function uploadImage(file) {
  const form = new FormData()
  form.append('file', file)
  const headers = { Accept: 'application/json' }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`

  let res
  try {
    res = await fetch('/api/images', { method: 'POST', headers, body: form })
  } catch {
    throw new ApiError('图片上传失败，请检查网络', 0)
  }
  if (!res.ok) {
    let message = '图片上传失败'
    try {
      const data = await res.json()
      message = data.error || message
    } catch {
      /* ignore */
    }
    if (res.status === 401 && token) {
      window.dispatchEvent(new Event('folio:unauthorized'))
    }
    throw new ApiError(message, res.status)
  }
  const data = await res.json()
  return data.image
}
