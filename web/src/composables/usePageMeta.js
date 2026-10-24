import { watchEffect } from 'vue'
import { site } from '../stores/site'


function absolute(value) {
  if (!value) return window.location.origin + window.location.pathname
  return new URL(value, window.location.origin).href
}

function setMeta(attr, key, content) {
  let element = document.head.querySelector(`meta[${attr}="${key}"]`)
  if (!content) {
    if (element) element.remove()
    return
  }
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(attr, key)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

function setCanonical(href) {
  let link = document.head.querySelector('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.setAttribute('rel', 'canonical')
    document.head.appendChild(link)
  }
  link.setAttribute('href', href)
}

function setJsonLd(data) {
  let script = document.getElementById('folio-page-jsonld')
  if (!data) {
    if (script) script.remove()
    return
  }
  if (!script) {
    script = document.createElement('script')
    script.id = 'folio-page-jsonld'
    script.type = 'application/ld+json'
    document.head.appendChild(script)
  }
  script.textContent = JSON.stringify(data)
}

function clearTags() {
  document.head.querySelectorAll('meta[property="article:tag"]').forEach((element) => element.remove())
}

export function usePageMeta(getMeta) {
  watchEffect(() => {
    const meta = getMeta() || {}
    const siteName = site.name || '知微'
    const title = meta.title ? `${meta.title} · ${siteName}` : siteName
    const description = meta.description || site.description || ''
    const url = absolute(meta.url)
    const image = absolute(meta.image || '/api/art/cover.png?seed=folio-site&kind=site')
    const type = meta.type || 'website'

    document.title = title
    setMeta('name', 'description', description)
    setMeta('name', 'robots', meta.noindex ? 'noindex, nofollow' : 'index, follow')
    setMeta('property', 'og:site_name', siteName)
    setMeta('property', 'og:title', title)
    setMeta('property', 'og:description', description)
    setMeta('property', 'og:type', type)
    setMeta('property', 'og:url', url)
    setMeta('property', 'og:image', image)
    setMeta('property', 'og:image:alt', meta.title || siteName)
    setMeta('property', 'og:image:width', '1200')
    setMeta('property', 'og:image:height', '630')
    setMeta('property', 'og:locale', 'zh_CN')
    setMeta('property', 'article:published_time', meta.published || '')
    setMeta('property', 'article:modified_time', meta.modified || '')
    setMeta('property', 'article:section', meta.section || '')
    setMeta('property', 'profile:username', meta.username || '')
    setMeta('name', 'twitter:card', 'summary_large_image')
    setMeta('name', 'twitter:title', title)
    setMeta('name', 'twitter:description', description)
    setMeta('name', 'twitter:image', image)

    clearTags()
    for (const tag of meta.tags || []) {
      const element = document.createElement('meta')
      element.setAttribute('property', 'article:tag')
      element.setAttribute('content', tag)
      document.head.appendChild(element)
    }

    setCanonical(url)
    setJsonLd(meta.jsonLd || null)
  })
}
