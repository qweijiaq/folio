import rough from 'roughjs'

export function hashSeed(value) {
  const text = String(value ?? 'folio')
  let h = 2166136261
  for (let i = 0; i < text.length; i += 1) {
    h ^= text.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return h >>> 0
}

function cssColors(el) {
  const style = getComputedStyle(el)
  const read = (name, fallback) => (style.getPropertyValue(name) || '').trim() || fallback
  return {
    paper: read('--paper', '#f4f3ee'),
    surface: read('--surface', '#fbfaf6'),
    ink: read('--ink', '#1d1d1a'),
    inkSoft: read('--ink-soft', '#57564f'),
    accent: read('--accent', '#2f4f9e'),
    accentSoft: read('--accent-soft', '#e2e7f5'),
    tints: [1, 2, 3, 4].map((i) => read(`--tint-${i}`, '#efeee7'))
  }
}

function firstGlyph(value) {
  const chars = Array.from(String(value || '').trim())
  if (!chars.length) return '·'
  const char = chars[0]
  return /[a-z]/.test(char) ? char.toUpperCase() : char
}

function displayScale(canvas) {
  const width = canvas.getBoundingClientRect().width || canvas.clientWidth || canvas.width
  return canvas.width / Math.max(width, 1)
}

// 极简封面：一张底色 + 一个手绘几何图形，最多再加一小笔强调色。
export function drawCover(canvas, { seed = 'folio', kind = 'article' } = {}) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const width = canvas.width
  const height = canvas.height
  const colors = cssColors(canvas)
  const h = hashSeed(`${seed}|${kind}`)
  const scale = displayScale(canvas)
  const strokeWidth = Math.min(7, Math.max(1.4, 1.4 * scale))

  ctx.clearRect(0, 0, width, height)
  ctx.fillStyle = colors.tints[h % colors.tints.length]
  ctx.fillRect(0, 0, width, height)

  const rc = rough.canvas(canvas)
  const line = {
    stroke: colors.ink,
    strokeWidth,
    roughness: 1.8,
    bowing: 1.2,
    seed: (h % 90000) + 1
  }
  const accent = {
    ...line,
    stroke: colors.accent,
    strokeWidth: strokeWidth * 1.08,
    seed: (h % 90000) + 37
  }
  const cx = width / 2
  const cy = height / 2

  switch (h % 4) {
    case 0:
      rc.circle(cx, cy, 190, line)
      rc.line(cx + 58, cy + 108, cx + 178, cy + 96, accent)
      break
    case 1:
      rc.curve(
        [
          [cx - 190, cy + 34],
          [cx - 96, cy - 58],
          [cx, cy + 26],
          [cx + 96, cy - 58],
          [cx + 190, cy + 34]
        ],
        line
      )
      rc.circle(cx + 156, cy - 84, 22, accent)
      break
    case 2:
      rc.line(cx - 172, cy - 58, cx + 118, cy - 56, line)
      rc.line(cx - 172, cy, cx + 34, cy + 2, line)
      rc.line(cx - 172, cy + 58, cx - 48, cy + 56, line)
      rc.circle(cx + 148, cy + 56, 17, accent)
      break
    default:
      rc.linearPath(
        [
          [cx - 138, cy + 78],
          [cx, cy - 86],
          [cx + 138, cy + 78]
        ],
        false,
        line
      )
      rc.line(cx - 38, cy + 78, cx + 62, cy + 76, accent)
  }
}

// 极简头像：一个手绘圆 + 一个字。
export function drawAvatar(canvas, { seed = 'folio', label = '' } = {}) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const size = canvas.width
  const colors = cssColors(canvas)
  const h = hashSeed(seed || label || 'folio')

  ctx.clearRect(0, 0, size, size)
  ctx.fillStyle = colors.tints[h % colors.tints.length]
  ctx.fillRect(0, 0, size, size)

  const rc = rough.canvas(canvas)
  const scale = displayScale(canvas)
  const ringWidth = Math.min(18, Math.max(2.4, 1.25 * scale))
  rc.circle(size / 2, size / 2, size * 0.82, {
    stroke: colors.ink,
    strokeWidth: ringWidth,
    roughness: 1.5,
    bowing: 0.8,
    seed: (h % 90000) + 3
  })

  ctx.fillStyle = h % 4 === 0 ? colors.accent : colors.ink
  ctx.font = `500 ${Math.round(size * 0.44)}px "Hannotate SC", "HanziPen SC", "Kaiti SC", "STKaiti", Georgia, serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(firstGlyph(label || seed), size / 2, size * 0.53)
}

export function drawAccent(svg, variant = 'underline', tone = 'accent') {
  if (!svg) return
  svg.innerHTML = ''
  const colors = cssColors(svg)
  const stroke = tone === 'ink' ? colors.ink : colors.accent
  const rc = rough.svg(svg)
  const opts = { stroke, strokeWidth: 1.8, roughness: 2, bowing: 1.4, seed: 17 }

  if (variant === 'circle') {
    svg.appendChild(rc.ellipse(100, 40, 168, 60, opts))
    return
  }
  if (variant === 'squiggle') {
    svg.appendChild(
      rc.curve(
        [
          [4, 17],
          [32, 5],
          [64, 18],
          [96, 5],
          [128, 18],
          [160, 5],
          [196, 15]
        ],
        opts
      )
    )
    return
  }
  if (variant === 'arrow') {
    svg.appendChild(rc.line(6, 15, 174, 9, opts))
    svg.appendChild(rc.line(174, 9, 156, 3, opts))
    svg.appendChild(rc.line(174, 9, 158, 17, opts))
    return
  }
  svg.appendChild(rc.line(4, 12, 196, 7, opts))
}

export function drawBrandMark(svg) {
  if (!svg) return
  svg.innerHTML = ''
  const colors = cssColors(svg)
  const rc = rough.svg(svg)
  const base = { stroke: colors.ink, strokeWidth: 1.8, roughness: 1.6, bowing: 0.8, seed: 7 }
  svg.appendChild(rc.circle(20, 20, 26, base))
  svg.appendChild(rc.line(14, 27, 27, 12, { ...base, stroke: colors.accent, seed: 13 }))
}
