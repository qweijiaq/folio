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

function sealGlyph(value) {
  const chars = Array.from(String(value ?? '').trim())
  for (const char of chars) {
    const code = char.codePointAt(0)
    const han = code >= 0x3400 && code <= 0x9fff
    if (han || /[A-Za-z0-9]/.test(char)) return /[a-z]/.test(char) ? char.toUpperCase() : char
  }
  return ''
}

function drawSeal(ctx, rc, x, y, size, glyph, colors, seed) {
  if (!glyph) return
  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(-0.07)
  rc.rectangle(-size / 2, -size / 2, size, size, {
    stroke: colors.accent,
    strokeWidth: Math.max(1.2, size * 0.035),
    roughness: 1.1,
    bowing: 0.5,
    fill: colors.accent,
    fillStyle: 'solid',
    seed
  })
  ctx.fillStyle = colors.paper
  ctx.font = `600 ${Math.round(size * 0.54)}px "Songti SC", "STSong", "Noto Serif CJK SC", Georgia, serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(glyph, 0, size * 0.04)
  ctx.restore()
}

function fillWash(ctx, color, alpha, ellipse) {
  const [cx, cy, rx, ry, rotation] = ellipse
  ctx.save()
  ctx.globalAlpha = alpha
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.ellipse(cx, cy, rx, ry, rotation, 0, Math.PI * 2)
  ctx.fill()
  ctx.restore()
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
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
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

function coverMotif(tags, category, title, h) {
  const text = `${tags.join(' ')} ${category} ${title}`.toLowerCase()
  const rules = [
    [/(代码|编程|开发|技术|架构|工程|go|css|javascript|typescript|api)/i, 3],
    [/(知识|写作|阅读|读书|笔记|文档|整理|管理)/i, 2],
    [/(设计|排版|美学|艺术|视觉|色彩|界面|ui|ux)/i, 4],
    [/(ai|模型|智能|问答|数据|算法|claude|deepseek|kimi|glm)/i, 0],
    [/(旅行|山|自然|生活|城市|散步|季节)/i, 1],
    [/(情绪|时间|留白|思考|随想|关系|记忆)/i, 5]
  ]
  for (const [pattern, motif] of rules) {
    if (pattern.test(text)) return motif
  }
  return h % 6
}

function drawMotifOrbit(rc, box, s) {
  const { x, y, w, h } = box
  const cx = x + w * 0.5
  const cy = y + h * 0.47
  const d = Math.min(w, h) * 0.62
  rc.circle(cx, cy, d, s.ink())
  rc.circle(cx + d * 0.14, cy + d * 0.12, d * 0.62, s.fine())
  rc.line(x + w * 0.08, y + h * 0.84, x + w * 0.4, y + h * 0.83, s.ink())
  for (let i = 0; i < 3; i += 1) {
    rc.line(x + w * (0.4 + i * 0.03), y + h * 0.78, x + w * (0.4 + i * 0.03), y + h * 0.88, s.fine())
  }
  rc.line(x + w * 0.86, y + h * 0.13, x + w * 0.86, y + h * 0.87, s.fine())
  for (let i = 0; i < 4; i += 1) {
    rc.line(x + w * 0.83, y + h * (0.2 + i * 0.17), x + w * 0.89, y + h * (0.2 + i * 0.17), s.fine())
  }
  rc.circle(x + w * 0.9, y + h * 0.15, d * 0.13, s.solid())
  rc.curve(
    [[x + w * 0.1, y + h * 0.22], [x + w * 0.18, y + h * 0.14], [x + w * 0.26, y + h * 0.24]],
    s.accent({ strokeWidth: s.fineWidth })
  )
}

function drawMotifRidge(rc, box, s) {
  const { x, y, w, h } = box
  const d = Math.min(w, h) * 0.3
  const sunX = x + w * 0.72
  const sunY = y + h * 0.28
  rc.circle(sunX, sunY, d, s.accent())
  rc.circle(sunX + d * 0.12, sunY + d * 0.08, d * 0.72, s.fine())
  for (let i = 0; i < 4; i += 1) {
    rc.line(sunX - d * 0.28 + i * d * 0.12, sunY - d * 0.2, sunX - d * 0.34 + i * d * 0.12, sunY + d * 0.2, s.fine())
  }
  rc.linearPath(
    [
      [x + w * 0.02, y + h * 0.72],
      [x + w * 0.2, y + h * 0.42],
      [x + w * 0.36, y + h * 0.62],
      [x + w * 0.52, y + h * 0.34],
      [x + w * 0.7, y + h * 0.66],
      [x + w * 0.85, y + h * 0.48],
      [x + w * 0.98, y + h * 0.72]
    ],
    false,
    s.ink()
  )
  rc.linearPath(
    [
      [x + w * 0.02, y + h * 0.82],
      [x + w * 0.26, y + h * 0.66],
      [x + w * 0.48, y + h * 0.78],
      [x + w * 0.66, y + h * 0.62],
      [x + w * 0.82, y + h * 0.76],
      [x + w * 0.98, y + h * 0.82]
    ],
    false,
    s.fine()
  )
  rc.line(x + w * 0.04, y + h * 0.9, x + w * 0.96, y + h * 0.89, s.fine())
  rc.circle(x + w * 0.14, y + h * 0.16, d * 0.12, s.solid())
}

function drawMotifStack(rc, box, s) {
  const { x, y, w, h } = box
  const pages = [
    [[x + w * 0.18, y + h * 0.12], [x + w * 0.9, y + h * 0.07], [x + w * 0.94, y + h * 0.72], [x + w * 0.22, y + h * 0.78]],
    [[x + w * 0.12, y + h * 0.2], [x + w * 0.84, y + h * 0.15], [x + w * 0.88, y + h * 0.8], [x + w * 0.16, y + h * 0.86]],
    [[x + w * 0.06, y + h * 0.28], [x + w * 0.78, y + h * 0.23], [x + w * 0.82, y + h * 0.88], [x + w * 0.1, y + h * 0.94]]
  ]
  pages.forEach((points, index) => {
    rc.linearPath(
      points,
      true,
      s.ink({
        strokeWidth: index === 2 ? s.strokeWidth : s.fineWidth,
        fill: s.colors.surface,
        fillStyle: 'solid'
      })
    )
  })
  rc.line(x + w * 0.16, y + h * 0.48, x + w * 0.62, y + h * 0.45, s.fine())
  rc.line(x + w * 0.16, y + h * 0.57, x + w * 0.54, y + h * 0.54, s.fine())
  rc.line(x + w * 0.16, y + h * 0.66, x + w * 0.58, y + h * 0.63, s.fine())
  rc.line(x + w * 0.04, y + h * 0.72, x + w * 0.4, y + h * 0.2, s.accent())
  rc.circle(x + w * 0.9, y + h * 0.14, 22, s.solid())
}

function drawMotifGrid(rc, box, s) {
  const { x, y, w, h } = box
  const gx = x + w * 0.1
  const gy = y + h * 0.1
  const gap = w * 0.04
  const cell = (w * 0.8 - gap * 2) / 3
  for (let row = 0; row < 2; row += 1) {
    for (let column = 0; column < 3; column += 1) {
      const filled = row === 0 && column === 2
      const left = gx + column * (cell + gap)
      const top = gy + row * (cell + gap)
      rc.rectangle(
        left,
        top,
        cell,
        cell,
        filled ? s.accent({ fill: s.colors.accentSoft, fillStyle: 'solid' }) : s.fine()
      )
      if (filled) rc.circle(left + cell / 2, top + cell / 2, 22, s.solid())
    }
  }
  rc.line(x + w * 0.12, y + h * 0.86, x + w * 0.48, y + h * 0.74, s.ink())
  rc.line(x + w * 0.6, y + h * 0.84, x + w * 0.64, y + h * 0.84, s.accent())
  rc.line(x + w * 0.62, y + h * 0.8, x + w * 0.62, y + h * 0.88, s.accent())
}

function drawMotifArch(rc, box, s) {
  const { x, y, w, h } = box
  const cx = x + w * 0.5
  const base = y + h * 0.86
  rc.line(x + w * 0.06, base, x + w * 0.94, base, s.fine())
  rc.circle(cx, y + h * 1.02, w * 0.72, s.ink())
  rc.circle(cx, y + h * 1.02, w * 0.54, s.fine())
  rc.circle(cx, y + h * 1.02, w * 0.34, s.accent({ strokeWidth: s.fineWidth }))
  rc.line(cx, y + h * 0.14, cx, y + h * 0.6, s.ink())
  rc.circle(cx, y + h * 0.1, 16, s.solid())
  rc.circle(x + w * 0.12, y + h * 0.72, 24, s.solid())
  for (let i = 0; i < 3; i += 1) {
    rc.line(x + w * 0.1, y + h * (0.34 + i * 0.09), x + w * 0.3, y + h * (0.33 + i * 0.09), s.fine())
  }
}

function drawMotifBotanical(rc, box, s) {
  const { x, y, w, h } = box
  const moonD = Math.min(w, h) * 0.32
  rc.circle(x + w * 0.74, y + h * 0.26, moonD, s.ink())
  rc.circle(x + w * 0.78, y + h * 0.3, moonD * 0.72, s.fine())
  rc.curve(
    [[x + w * 0.16, y + h * 0.95], [x + w * 0.2, y + h * 0.7], [x + w * 0.18, y + h * 0.5], [x + w * 0.24, y + h * 0.3]],
    s.ink()
  )
  rc.curve(
    [[x + w * 0.26, y + h * 0.95], [x + w * 0.3, y + h * 0.74], [x + w * 0.28, y + h * 0.55], [x + w * 0.34, y + h * 0.42]],
    s.fine()
  )
  const leaves = [
    [[x + w * 0.2, y + h * 0.7], [x + w * 0.12, y + h * 0.66], [x + w * 0.1, y + h * 0.58], [x + w * 0.17, y + h * 0.6]],
    [[x + w * 0.18, y + h * 0.5], [x + w * 0.1, y + h * 0.46], [x + w * 0.09, y + h * 0.38], [x + w * 0.16, y + h * 0.4]],
    [[x + w * 0.3, y + h * 0.74], [x + w * 0.38, y + h * 0.7], [x + w * 0.4, y + h * 0.62], [x + w * 0.33, y + h * 0.64]],
    [[x + w * 0.28, y + h * 0.55], [x + w * 0.2, y + h * 0.51], [x + w * 0.19, y + h * 0.43], [x + w * 0.26, y + h * 0.45]]
  ]
  leaves.forEach((points, index) => {
    rc.curve(points, index === 1 ? s.accent({ strokeWidth: s.fineWidth }) : s.fine())
  })
  rc.circle(x + w * 0.24, y + h * 0.28, 16, s.solid())
  rc.line(x + w * 0.04, y + h * 0.96, x + w * 0.96, y + h * 0.95, s.fine())
}

const coverHandFont = '"Hannotate SC", "HanziPen SC", "Kaiti SC", "STKaiti", "KaiTi", "Songti SC", Georgia, serif'


// 文章封面：把标题、摘要、标签与分类排成一张手绘索引卡，再按内容关键词配一组线稿。
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
  const strokeWidth = Math.min(6.5, Math.max(1.8, 1.45 * scale))
  const fineWidth = Math.max(1, strokeWidth * 0.52)
  const random = seededRandom(h ^ 0x9e3779b9)

  ctx.clearRect(0, 0, width, height)
  ctx.fillStyle = colors.tints[h % colors.tints.length]
  ctx.fillRect(0, 0, width, height)

  fillWash(ctx, colors.accentSoft, 0.46, [width * 0.84, height * 0.12, 300 + (h % 3) * 26, 220 + (h % 4) * 18, -0.2])
  fillWash(ctx, colors.tints[(h + 2) % colors.tints.length], 0.6, [width * 0.11, height * 0.94, 250, 170, 0.18])

  const rc = rough.canvas(canvas)
  let lineSeed = (h % 90000) + 1
  const ink = (extra = {}) => ({
    stroke: colors.ink,
    strokeWidth,
    roughness: 1.65,
    bowing: 1.05,
    ...extra,
    seed: (lineSeed += 13)
  })
  const fine = (extra = {}) => ink({ strokeWidth: fineWidth, ...extra })
  const accent = (extra = {}) => ({
    stroke: colors.accent,
    strokeWidth: strokeWidth * 1.04,
    roughness: 1.5,
    bowing: 0.9,
    ...extra,
    seed: (lineSeed += 17)
  })
  const solidAccent = (extra = {}) => accent({ fill: colors.accent, fillStyle: 'solid', ...extra })

  if (kind === 'user') {
    drawGrain(ctx, width, height, colors, random)
    rc.circle(width * 0.78, height * 1.16, height * 1.25, ink())
    rc.circle(width * 0.78, height * 1.16, height * 0.9, fine())
    rc.line(78, 336, 410, 332, ink())
    rc.line(78, 374, 282, 370, fine())
    rc.circle(width * 0.89, 112, 26, solidAccent())
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

  const panelW = 566
  const panelH = 372
  const panelY = 54
  const pad = 32
  const textOnRight = h % 2 === 1
  const panelX = textOnRight ? width - panelW - 54 : 54
  const textX = panelX + pad
  const textW = panelW - pad * 2
  const artBox = textOnRight
    ? { x: 52, y: 60, w: 500, h: 360 }
    : { x: 648, y: 60, w: 500, h: 360 }

  ctx.save()
  ctx.globalAlpha = 0.72
  rc.rectangle(panelX, panelY, panelW, panelH, {
    stroke: colors.line,
    strokeWidth: fineWidth,
    roughness: 1.8,
    bowing: 0.8,
    fill: colors.surface,
    fillStyle: 'solid',
    seed: (h % 90000) + 3
  })
  ctx.restore()

  const dividerX = textOnRight ? panelX - 26 : panelX + panelW + 26
  ctx.save()
  ctx.globalAlpha = 0.5
  rc.line(dividerX, 90, dividerX, 390, fine())
  ctx.restore()

  drawGrain(ctx, width, height, colors, random)

  ctx.textAlign = 'left'
  ctx.textBaseline = 'alphabetic'

  rc.line(textX, 96, textX + 22, 96, accent({ strokeWidth: fineWidth * 1.2 }))
  ctx.font = `18px ${coverHandFont}`
  ctx.fillStyle = colors.inkSoft
  ctx.fillText(truncateCanvasText(ctx, eyebrow, textW - 34), textX + 32, 102)

  const titleChars = Array.from(displayTitle).length
  let titleSize = 64
  if (titleChars > 10) titleSize = 58
  if (titleChars > 16) titleSize = 52
  if (titleChars > 24) titleSize = 47
  if (titleChars > 32) titleSize = 43
  ctx.font = `600 ${titleSize}px ${coverHandFont}`
  ctx.fillStyle = colors.ink
  const titleLines = wrapCanvasText(ctx, displayTitle, textW, 2)
  const titleStart = 172
  const titleLineHeight = titleSize * 1.2
  titleLines.forEach((line, index) => {
    ctx.fillText(line, textX, titleStart + index * titleLineHeight)
  })
  const lastTitleBaseline = titleStart + (titleLines.length - 1) * titleLineHeight
  const lastTitleWidth = ctx.measureText(titleLines[titleLines.length - 1] || '').width
  rc.line(
    textX,
    lastTitleBaseline + 16,
    textX + Math.min(110, Math.max(48, lastTitleWidth * 0.32)),
    lastTitleBaseline + 16,
    accent({ strokeWidth: fineWidth * 1.2 })
  )

  if (abstractText) {
    ctx.font = `19px ${coverHandFont}`
    const abstractLines = wrapCanvasText(ctx, abstractText, textW - 18, 2)
    const abstractStart = lastTitleBaseline + 52
    ctx.save()
    ctx.globalAlpha = 0.7
    ctx.fillStyle = colors.accent
    ctx.fillRect(textX, abstractStart - 19, 3, abstractLines.length * 27 + 3)
    ctx.restore()
    ctx.save()
    ctx.globalAlpha = 0.86
    ctx.fillStyle = colors.inkSoft
    abstractLines.forEach((line, index) => {
      ctx.fillText(line, textX + 16, abstractStart + index * 27)
    })
    ctx.restore()
  }

  const tagY = panelY + panelH - 36
  ctx.font = `17px ${coverHandFont}`
  let tagCursor = textX
  let shownTags = 0
  for (const tag of tagList) {
    const tagText = `#${tag}`
    const boxWidth = ctx.measureText(tagText).width + 24
    if (shownTags >= 3 || tagCursor + boxWidth > textX + textW) break
    rc.rectangle(tagCursor, tagY - 22, boxWidth, 30, {
      stroke: colors.line,
      strokeWidth: fineWidth,
      roughness: 1.2,
      bowing: 0.7,
      seed: (lineSeed += 11)
    })
    ctx.fillStyle = colors.inkSoft
    ctx.fillText(tagText, tagCursor + 12, tagY)
    tagCursor += boxWidth + 9
    shownTags += 1
  }
  if (!tagList.length) {
    rc.line(textX, tagY - 7, textX + 84, tagY - 7, accent({ strokeWidth: fineWidth }))
  }

  const style = {
    rc,
    colors,
    ink,
    fine,
    accent,
    solid: solidAccent,
    strokeWidth,
    fineWidth
  }
  switch (coverMotif(tagList, categoryText, titleText, h)) {
    case 0:
      drawMotifOrbit(rc, artBox, style)
      break
    case 1:
      drawMotifRidge(rc, artBox, style)
      break
    case 2:
      drawMotifStack(rc, artBox, style)
      break
    case 3:
      drawMotifGrid(rc, artBox, style)
      break
    case 4:
      drawMotifArch(rc, artBox, style)
      break
    default:
      drawMotifBotanical(rc, artBox, style)
  }

  const sealX = textOnRight ? 110 : width - 110
  drawSeal(
    ctx,
    rc,
    sealX,
    400,
    56,
    sealGlyph(tagList[0] || categoryText || displayTitle),
    colors,
    (h % 90000) + 501
  )
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
