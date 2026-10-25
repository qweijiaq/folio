import { describe, expect, it } from 'vitest'
import { buildLineDiff } from './diff'

describe('line diff', () => {
  it('marks added, removed and context lines', () => {
    const { rows, stats } = buildLineDiff('第一行\n旧内容\n最后一行', '第一行\n新内容\n最后一行')
    expect(stats).toEqual({ added: 1, removed: 1 })
    expect(rows).toEqual([
      { type: 'context', text: '第一行' },
      { type: 'removed', text: '旧内容' },
      { type: 'added', text: '新内容' },
      { type: 'context', text: '最后一行' }
    ])
  })

  it('handles empty text', () => {
    const { rows, stats } = buildLineDiff('', '只有一行')
    expect(stats).toEqual({ added: 1, removed: 0 })
    expect(rows).toEqual([{ type: 'added', text: '只有一行' }])
  })
})
