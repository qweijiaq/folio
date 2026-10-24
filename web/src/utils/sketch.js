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

// 手绘封面：纸面底色、柔和色块、颗粒，再叠一组不对称线稿；标题首字做成小印章。
export function drawCover(canvas, { seed = 'folio', kind = 'article', label = '' } = {}) {
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
  drawGrain(ctx, width, height, colors, random)

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
    rc.circle(width * 0.78, height * 1.16, height * 1.25, ink())
    rc.circle(width * 0.78, height * 1.16, height * 0.9, fine())
    rc.line(78, 336, 410, 332, ink())
    rc.line(78, 374, 282, 370, fine())
    rc.circle(width * 0.89, 112, 26, solidAccent())
    return
  }

  switch (h % 8) {
    case 0: {
      rc.circle(790, 220, 240, ink())
      rc.circle(835, 250, 160, fine())
      rc.line(100, 338, 430, 334, ink())
      for (let i = 0; i < 3; i += 1) rc.line(430 + i * 24, 316, 430 + i * 24, 352, fine())
      rc.line(980, 90, 980, 380, fine())
      for (let i = 0; i < 4; i += 1) rc.line(968, 120 + i * 62, 992, 120 + i * 62, fine())
      rc.circle(1050, 110, 34, solidAccent())
      rc.curve([[120, 150], [190, 110], [250, 170]], accent({ strokeWidth: fineWidth }))
      break
    }
    case 1: {
      rc.circle(900, 145, 150, accent())
      rc.circle(925, 160, 110, fine())
      for (let i = 0; i < 4; i += 1) rc.line(870 + i * 16, 120, 855 + i * 16, 170, fine())
      rc.linearPath(
        [[60, 365], [250, 225], [380, 315], [530, 180], [690, 330], [830, 245], [1140, 365]],
        false,
        ink()
      )
      rc.linearPath(
        [[60, 400], [300, 290], [480, 360], [650, 260], [820, 355], [1000, 300], [1140, 400]],
        false,
        fine()
      )
      rc.line(60, 410, 1140, 408, fine())
      rc.circle(160, 130, 22, solidAccent())
      break
    }
    case 2: {
      const pages = [
        [[700, 112], [1044, 96], [1058, 350], [714, 366]],
        [[668, 134], [1012, 118], [1026, 372], [682, 388]],
        [[636, 156], [980, 140], [994, 394], [650, 410]]
      ]
      pages.forEach((points, index) => {
        rc.linearPath(points, true, ink({
          strokeWidth: index === 2 ? strokeWidth : fineWidth,
          fill: colors.surface,
          fillStyle: 'solid'
        }))
      })
      rc.line(690, 222, 900, 212, fine())
      rc.line(690, 262, 846, 254, fine())
      rc.line(690, 302, 884, 294, fine())
      rc.line(110, 380, 560, 122, ink())
      rc.circle(320, 250, 60, fine())
      rc.circle(1040, 90, 26, solidAccent())
      for (let i = 0; i < 3; i += 1) rc.line(110, 170 + i * 42, 300 - i * 26, 168 + i * 42, fine())
      break
    }
    case 3: {
      const wave = (y, amp, options, phase) => {
        const points = []
        for (let i = 0; i <= 6; i += 1) points.push([70 + i * 176, y + Math.sin(i * 0.95 + phase) * amp])
        rc.curve(points, options)
      }
      wave(150, 16, fine(), 0.2)
      wave(215, 22, ink(), 1.1)
      wave(285, 18, accent(), 2)
      wave(345, 14, fine(), 0.7)
      rc.circle(1010, 135, 84, ink())
      rc.circle(1060, 250, 26, solidAccent())
      for (let i = 0; i < 4; i += 1) rc.line(90, 120 + i * 70, 126, 118 + i * 70, fine())
      break
    }
    case 4: {
      rc.line(80, 420, 1120, 418, fine())
      rc.circle(820, 480, 420, ink())
      rc.circle(820, 480, 300, fine())
      rc.circle(820, 480, 180, accent({ strokeWidth: fineWidth }))
      rc.line(820, 80, 820, 300, ink())
      rc.circle(820, 70, 18, solidAccent())
      rc.circle(300, 330, 28, solidAccent())
      for (let i = 0; i < 3; i += 1) rc.line(100, 190 + i * 44, 280 - i * 30, 188 + i * 44, fine())
      break
    }
    case 5: {
      const gx = 680
      const gy = 105
      const cell = 112
      const gap = 18
      for (let row = 0; row < 2; row += 1) {
        for (let column = 0; column < 3; column += 1) {
          const x = gx + column * (cell + gap)
          const y = gy + row * (cell + gap)
          const filled = row === 0 && column === 2
          rc.rectangle(
            x,
            y,
            cell,
            cell,
            filled
              ? accent({ fill: colors.accentSoft, fillStyle: 'solid' })
              : fine()
          )
          if (filled) rc.circle(x + cell / 2, y + cell / 2, 26, solidAccent())
        }
      }
      rc.line(110, 380, 560, 118, ink())
      for (let i = 0; i < 3; i += 1) rc.circle(170 + i * 90, 330 - i * 55, 10, fine())
      rc.circle(260, 210, 70, fine())
      rc.line(470, 330, 520, 330, accent())
      rc.line(495, 305, 495, 355, accent())
      break
    }
    case 6: {
      rc.circle(930, 150, 160, ink())
      rc.circle(955, 170, 120, fine())
      rc.curve([[180, 420], [210, 330], [195, 240], [235, 150]], ink())
      rc.curve([[270, 420], [300, 340], [280, 260], [315, 190]], fine())
      const leaves = [
        [[210, 330], [170, 315], [160, 280], [195, 285]],
        [[195, 240], [155, 228], [150, 195], [185, 200]],
        [[300, 340], [340, 325], [350, 290], [315, 295]],
        [[280, 260], [245, 248], [238, 218], [270, 222]]
      ]
      leaves.forEach((points, index) => {
        rc.curve(points, index === 1 ? accent({ strokeWidth: fineWidth }) : fine())
      })
      rc.circle(235, 150, 18, solidAccent())
      rc.line(90, 420, 1100, 418, fine())
      break
    }
    default: {
      rc.rectangle(610, 90, 400, 300, ink())
      rc.rectangle(640, 120, 340, 240, fine())
      const glyph = sealGlyph(label)
      if (glyph) {
        ctx.save()
        ctx.globalAlpha = 0.16
        ctx.fillStyle = colors.ink
        ctx.font = '600 190px "Songti SC", "STSong", "Noto Serif CJK SC", Georgia, serif'
        ctx.textAlign = 'center'
        ctx.textBaseline = 'middle'
        ctx.fillText(glyph, 810, 244)
        ctx.restore()
      } else {
        rc.circle(810, 245, 100, fine())
      }
      for (let i = 0; i < 4; i += 1) {
        rc.line(100, 180 + i * 50, 360 - i * 38, 178 + i * 50, i === 0 ? ink() : fine())
      }
      rc.line(100, 390, 330, 388, accent())
      rc.circle(1010, 130, 24, solidAccent())
    }
  }

  drawSeal(ctx, rc, 1082, 400, 58, sealGlyph(label), colors, (h % 90000) + 501)
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
