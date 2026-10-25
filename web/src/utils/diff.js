import { diffLines } from 'diff'

export function buildLineDiff(oldText = '', newText = '') {
  const parts = diffLines(oldText || '', newText || '')
  const rows = []
  let added = 0
  let removed = 0
  for (const part of parts) {
    const type = part.added ? 'added' : part.removed ? 'removed' : 'context'
    const lines = part.value.split('\n')
    if (lines[lines.length - 1] === '') lines.pop()
    for (const text of lines) {
      rows.push({ type, text })
      if (type === 'added') added += 1
      if (type === 'removed') removed += 1
    }
  }
  return { rows, stats: { added, removed } }
}
