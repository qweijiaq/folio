import { onBeforeUnmount, onMounted, ref } from 'vue'

export function useReadingProgress(selector = '.prose') {
  const progress = ref(0)
  const activeId = ref('')
  let timer = 0

  function measure() {
    timer = 0
    const scrollable = document.documentElement.scrollHeight - window.innerHeight
    progress.value = scrollable > 0 ? Math.min(100, Math.max(0, (window.scrollY / scrollable) * 100)) : 0

    const headings = Array.from(document.querySelectorAll(`${selector} h1[id], ${selector} h2[id], ${selector} h3[id]`))
    let current = headings[0]?.id || ''
    for (const heading of headings) {
      if (heading.getBoundingClientRect().top <= 150) current = heading.id
      else break
    }
    activeId.value = current
  }

  function refresh() {
    if (timer) window.clearTimeout(timer)
    timer = window.setTimeout(measure, 40)
  }

  onMounted(() => {
    window.addEventListener('scroll', refresh, { passive: true })
    window.addEventListener('resize', refresh)
    refresh()
  })

  onBeforeUnmount(() => {
    window.removeEventListener('scroll', refresh)
    window.removeEventListener('resize', refresh)
    if (timer) window.clearTimeout(timer)
  })

  return { progress, activeId, refresh }
}
