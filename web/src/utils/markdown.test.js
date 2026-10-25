import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

describe('markdown rendering', () => {
  it('adds heading ids and a table of contents', () => {
    const { html, toc } = renderMarkdown('# 标题\n\n## 小节\n\n正文')
    expect(html).toContain('<h1 id="h-1">标题<a class="heading-anchor"')
    expect(html).toContain('<h2 id="h-2">小节<a class="heading-anchor"')
    expect(toc).toEqual([
      { level: 1, text: '标题', id: 'h-1' },
      { level: 2, text: '小节', id: 'h-2' }
    ])
  })

  it('renders collapsible highlighted code blocks', () => {
    const { html } = renderMarkdown('```js\nconst answer = 42\n```')
    expect(html).toContain('class="code-block"')
    expect(html).toContain('class="code-lang">js')
    expect(html).toContain('class="code-toggle"')
    expect(html).toContain('class="code-copy"')
    expect(html).toContain('hljs')
    expect(html).toContain('answer')
  })

  it('opens external links safely and lazily loads images', () => {
    const { html } = renderMarkdown('[示例](https://example.com)\n\n![图](image.png)')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
    expect(html).toContain('loading="lazy"')
    expect(html).toContain('tabindex="0"')
  })

  it('resolves wiki links and marks missing targets', () => {
    const { html } = renderMarkdown('[[测试文档]] 和 [[不存在]]', {
      '测试文档': { id: 7, kind: 'doc' }
    })
    expect(html).toContain('class="wiki-link" href="/docs/7"')
    expect(html).toContain('测试文档')
    expect(html).toContain('wiki-link--missing')
    expect(html).toContain('[[不存在]]')
  })

  it('escapes raw HTML', () => {
    const { html } = renderMarkdown('<script>alert(1)</script>')
    expect(html).toContain('&lt;script&gt;')
    expect(html).not.toContain('<script>')
  })
})
