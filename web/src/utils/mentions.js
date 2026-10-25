export function splitMentions(source = '') {
  const text = source || ''
  const pattern = /(^|[^A-Za-z0-9_@])(@[A-Za-z0-9_]{2,20})/g
  const out = []
  let last = 0
  let match
  while ((match = pattern.exec(text))) {
    const start = match.index + match[1].length
    if (start > last) out.push({ type: 'text', text: text.slice(last, start) })
    out.push({ type: 'mention', text: match[2], username: match[2].slice(1) })
    last = start + match[2].length
    pattern.lastIndex = last
  }
  if (last < text.length) out.push({ type: 'text', text: text.slice(last) })
  return out
}
