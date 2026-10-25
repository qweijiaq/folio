import { describe, expect, it } from 'vitest'
import { splitMentions } from './mentions'

describe('mention parsing', () => {
  it('splits usernames and keeps surrounding text', () => {
    expect(splitMentions('你好 @bob 和 @carol_2')).toEqual([
      { type: 'text', text: '你好 ' },
      { type: 'mention', text: '@bob', username: 'bob' },
      { type: 'text', text: ' 和 ' },
      { type: 'mention', text: '@carol_2', username: 'carol_2' }
    ])
  })

  it('ignores email addresses and short tokens', () => {
    expect(splitMentions('foo@example.com @a')).toEqual([
      { type: 'text', text: 'foo@example.com @a' }
    ])
  })
})
