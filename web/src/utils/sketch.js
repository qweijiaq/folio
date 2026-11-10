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
    line: read('--line', '#d9d6cb'),
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

function seededRandom(seed) {
  let state = (seed >>> 0) || 0x9e3779b9
  return () => {
    state ^= state << 13
    state >>>= 0
    state ^= state >>> 17
    state ^= state << 5
    state >>>= 0
    return state / 4294967296
  }
}

function drawGrain(ctx, width, height, colors, random) {
  ctx.save()
  for (let i = 0; i < 34; i += 1) {
    const x = random() * width
    const y = random() * height
    const dot = 0.7 + random() * 1.5
    ctx.globalAlpha = 0.05 + random() * 0.07
    ctx.fillStyle = colors.ink
    ctx.beginPath()
    ctx.arc(x, y, dot, 0, Math.PI * 2)
    ctx.fill()
  }
  ctx.globalAlpha = 0.09
  ctx.strokeStyle = colors.ink
  ctx.lineWidth = 1
  for (let i = 0; i < 12; i += 1) {
    const x = 40 + random() * (width - 80)
    const y = 30 + random() * (height - 60)
    const length = 8 + random() * 22
    ctx.beginPath()
    ctx.moveTo(x, y)
    ctx.lineTo(x + length, y + (random() - 0.5) * 3)
    ctx.stroke()
  }
  ctx.restore()
}

function cleanCoverText(value) {
  return String(value ?? '')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/!\[[^\]]*\]\[[^\]]*\]/g, ' ')
    .replace(/^\[[^\]]+\]:.*$/gm, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\[[^\]]*\]/g, '$1')
    .replace(/https?:\/\/\S+/g, ' ')
    .replace(/[`*_>#~]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function ellipsizeLines(ctx, lines, maxWidth) {
  const last = lines[lines.length - 1] || ''
  const chars = Array.from(last)
  while (chars.length && ctx.measureText(chars.join('') + '…').width > maxWidth) chars.pop()
  lines[lines.length - 1] = chars.join('') + '…'
  return lines
}

function wrapCanvasText(ctx, text, maxWidth, maxLines) {
  const chars = Array.from(text)
  const lines = []
  let line = ''
  for (let i = 0; i < chars.length; i += 1) {
    const test = line + chars[i]
    if (line && ctx.measureText(test).width > maxWidth) {
      lines.push(line)
      line = chars[i]
      if (lines.length === maxLines) return ellipsizeLines(ctx, lines, maxWidth)
    } else {
      line = test
    }
  }
  if (line) lines.push(line)
  if (lines.length > maxLines) {
    lines.length = maxLines
    return ellipsizeLines(ctx, lines, maxWidth)
  }
  return lines
}

function truncateCanvasText(ctx, text, maxWidth) {
  if (ctx.measureText(text).width <= maxWidth) return text
  const chars = Array.from(text)
  while (chars.length && ctx.measureText(chars.join('') + '…').width > maxWidth) chars.pop()
  return chars.join('') + '…'
}

const coverHandFont = '"Hannotate SC", "HanziPen SC", "Kaiti SC", "STKaiti", "KaiTi", "Songti SC", Georgia, serif'

function roundRectPath(ctx, x, y, w, h, radius) {
  const r = Math.max(0, Math.min(radius, w / 2, h / 2))
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

// 封面：一张安静的索引卡，只保留标题、摘要、标签和一笔强调色。
export function drawCover(canvas, {
  seed = 'folio',
  kind = 'article',
  label = '',
  abstract = '',
  tags = [],
  category = ''
} = {}) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const width = canvas.width
  const height = canvas.height
  const colors = cssColors(canvas)
  const h = hashSeed(`${seed}|${kind}`)
  const scale = displayScale(canvas)
  const random = seededRandom(h ^ 0x9e3779b9)

  ctx.clearRect(0, 0, width, height)
  ctx.fillStyle = colors.tints[h % colors.tints.length]
  ctx.fillRect(0, 0, width, height)
  drawGrain(ctx, width, height, colors, random)

  if (kind === 'user') {
    ctx.save()
    ctx.strokeStyle = colors.line
    ctx.globalAlpha = 0.55
    ctx.lineWidth = Math.max(1, 1.2 * scale)
    ctx.beginPath()
    ctx.arc(width * 0.8, height * 0.52, height * 0.3, 0, Math.PI * 2)
    ctx.stroke()
    ctx.globalAlpha = 1
    ctx.fillStyle = colors.accent
    ctx.fillRect(width * 0.08, height * 0.5, 46, Math.max(2, 1.6 * scale))
    ctx.restore()
    return
  }

  const titleText = cleanCoverText(label)
  const displayTitle = titleText || (kind === 'doc' ? '未命名文档' : '未命名文章')
  const abstractText = cleanCoverText(abstract)
  const tagList = (Array.isArray(tags) ? tags : [])
    .map(cleanCoverText)
    .filter(Boolean)
    .slice(0, 4)
  const categoryText = cleanCoverText(category)
  const typeLabel = kind === 'doc' ? '文档' : '文章'
  const eyebrow = categoryText ? `${categoryText} · ${typeLabel}` : typeLabel

  const cardX = 58
  const cardY = 38
  const cardW = width - cardX * 2
  const cardH = height - cardY * 2
  ctx.save()
  ctx.globalAlpha = 0.82
  ctx.fillStyle = colors.surface
  roundRectPath(ctx, cardX, cardY, cardW, cardH, 18)
  ctx.fill()
  ctx.globalAlpha = 0.7
  ctx.strokeStyle = colors.line
  ctx.lineWidth = Math.max(1, 1.1 * scale)
  ctx.stroke()
  ctx.restore()

  const textX = cardX + 52
  const textW = cardW - 104
  ctx.textAlign = 'left'
  ctx.textBaseline = 'alphabetic'

  ctx.fillStyle = colors.accent
  ctx.fillRect(textX, cardY + 40, 44, Math.max(2, 1.6 * scale))

  ctx.font = `500 17px ${coverHandFont}`
  ctx.fillStyle = colors.inkSoft
  ctx.fillText(truncateCanvasText(ctx, eyebrow, textW), textX, cardY + 82)

  const titleChars = Array.from(displayTitle).length
  let titleSize = 60
  if (titleChars > 11) titleSize = 54
  if (titleChars > 18) titleSize = 47
  if (titleChars > 27) titleSize = 41
  ctx.font = `600 ${titleSize}px ${coverHandFont}`
  ctx.fillStyle = colors.ink
  const titleLines = wrapCanvasText(ctx, displayTitle, textW, 2)
  const titleStart = cardY + 150
  const titleLineHeight = titleSize * 1.2
  titleLines.forEach((line, index) => {
    ctx.fillText(line, textX, titleStart + index * titleLineHeight)
  })
  const lastTitleBaseline = titleStart + (titleLines.length - 1) * titleLineHeight

  if (abstractText) {
    ctx.font = `19px ${coverHandFont}`
    ctx.fillStyle = colors.inkSoft
    ctx.globalAlpha = 0.86
    const abstractLines = wrapCanvasText(ctx, abstractText, textW, 2)
    const abstractStart = Math.max(lastTitleBaseline + 50, cardY + 226)
    abstractLines.forEach((line, index) => {
      ctx.fillText(line, textX, abstractStart + index * 27)
    })
    ctx.globalAlpha = 1
  }

  const tagY = cardY + cardH - 38
  if (tagList.length) {
    ctx.save()
    ctx.globalAlpha = 0.36
    ctx.strokeStyle = colors.line
    ctx.lineWidth = Math.max(1, scale)
    ctx.beginPath()
    ctx.moveTo(textX, tagY - 28)
    ctx.lineTo(textX + textW, tagY - 28)
    ctx.stroke()
    ctx.restore()

    ctx.font = `17px ${coverHandFont}`
    ctx.fillStyle = colors.inkSoft
    const tagLine = tagList.map((tag) => `#${tag}`).join('  ·  ')
    ctx.fillText(truncateCanvasText(ctx, tagLine, textW), textX, tagY)
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
