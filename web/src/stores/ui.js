import { reactive } from 'vue'

const THEME_KEY = 'folio.theme'

const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches

export const ui = reactive({
  theme: localStorage.getItem(THEME_KEY) || (prefersDark ? 'dark' : 'light'),
  navOpen: false,
  searchOpen: false
})

export function applyTheme() {
  document.documentElement.dataset.theme = ui.theme
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', ui.theme === 'dark' ? '#1b1917' : '#f6f2ea')
}

export function toggleTheme() {
  ui.theme = ui.theme === 'light' ? 'dark' : 'light'
  localStorage.setItem(THEME_KEY, ui.theme)
  applyTheme()
}
