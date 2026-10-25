import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import go from 'highlight.js/lib/languages/go'
import python from 'highlight.js/lib/languages/python'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import css from 'highlight.js/lib/languages/css'
import xml from 'highlight.js/lib/languages/xml'
import sql from 'highlight.js/lib/languages/sql'
import yaml from 'highlight.js/lib/languages/yaml'
import markdown from 'highlight.js/lib/languages/markdown'
import ini from 'highlight.js/lib/languages/ini'
import diff from 'highlight.js/lib/languages/diff'

const languages = { javascript, typescript, go, python, bash, json, css, xml, sql, yaml, markdown, ini, diff }
Object.entries(languages).forEach(([name, def]) => hljs.registerLanguage(name, def))
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('golang', go)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('md', markdown)
hljs.registerLanguage('yml', yaml)

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: false,
  typographer: false
})
md.enable(['table', 'strikethrough'])

md.inline.ruler.before('link', 'wiki_link', (state, silent) => {
  const start = state.pos
  if (state.src.charCodeAt(start) !== 0x5b || state.src.charCodeAt(start + 1) !== 0x5b) return false
  const end = state.src.indexOf(']]', start + 2)
  if (end < 0) return false
  const title = state.src.slice(start + 2, end).trim()
  if (!title || /[\r\n]/.test(title)) return false
  if (!silent) {
    const token = state.push('wiki_link', 'a', 0)
    token.content = title
    token.markup = '[['
  }
  state.pos = end + 2
  return true
})

md.renderer.rules.wiki_link = (tokens, idx, options, env) => {
  const title = tokens[idx].content.trim()
  const item = (env?.linkMap || {})[title.toLowerCase()]
  if (!item) {
    return `<span class="wiki-link wiki-link--missing" title="没有找到同名内容">[[${md.utils.escapeHtml(title)}]]</span>`
  }
  const kind = item.kind === 'doc' ? 'docs' : 'articles'
  return `<a class="wiki-link" href="/${kind}/${item.id}">${md.utils.escapeHtml(title)}</a>`
}

md.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  const info = (token.info || '').trim()
  const lang = info.split(/\s+/)[0] || ''
  let body
  if (lang && hljs.getLanguage(lang)) {
    body = hljs.highlight(token.content, { language: lang, ignoreIllegals: true }).value
  } else {
    body = md.utils.escapeHtml(token.content)
  }
  const label = lang ? md.utils.escapeHtml(lang) : 'text'
  return `<div class="code-block"><div class="code-head"><span class="code-lang">${label}</span><span class="code-actions"><button class="code-toggle" type="button" aria-expanded="true">收起</button><button class="code-copy" type="button">复制</button></span></div><pre class="hljs"><code class="language-${label}">${body}</code></pre></div>`
}

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const href = tokens[idx].attrGet('href') || ''
  if (/^https?:\/\//i.test(href)) {
    tokens[idx].attrSet('target', '_blank')
    tokens[idx].attrSet('rel', 'noopener noreferrer')
  }
  return self.renderToken(tokens, idx, options)
}

export function renderMarkdown(source, linkMap = {}) {
  let html = md.render(source || '', { linkMap })
  const toc = []
  html = html.replace(/<h([1-4])>(.*?)<\/h\1>/g, (match, level, inner) => {
    const id = `h-${toc.length + 1}`
    const text = inner.replace(/<[^>]+>/g, '')
    toc.push({ level: Number(level), text, id })
    return `<h${level} id="${id}">${inner}<a class="heading-anchor" href="#${id}" aria-label="复制本节链接" title="复制本节链接">#</a></h${level}>`
  })
  html = html.replace(/<img /g, '<img loading="lazy" tabindex="0" ')
  return { html, toc }
}
