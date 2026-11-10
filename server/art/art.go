package art

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	inkColor    = "#1e1e1a"
	accentColor = "#2f4f9e"
)

var tints = []string{"#e9ecf3", "#e8ece2", "#ecebe4", "#ece8ef"}

func hash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func firstGlyph(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "·"
	}
	r, _ := utf8.DecodeRuneInString(title)
	if r >= 'a' && r <= 'z' {
		r -= 'a' - 'A'
	}
	return string(r)
}

// smoothClosed 用二次曲线把一个略不规则的圆点集连成手绘感的闭合路径。
func smoothClosed(points [][2]float64) string {
	if len(points) < 3 {
		return ""
	}
	n := len(points)
	start := [2]float64{
		(points[n-1][0] + points[0][0]) / 2,
		(points[n-1][1] + points[0][1]) / 2,
	}
	var b strings.Builder
	fmt.Fprintf(&b, "M %.1f %.1f", start[0], start[1])
	for i := 0; i < n; i++ {
		p := points[i]
		next := points[(i+1)%n]
		midX := (p[0] + next[0]) / 2
		midY := (p[1] + next[1]) / 2
		fmt.Fprintf(&b, " Q %.1f %.1f %.1f %.1f", p[0], p[1], midX, midY)
	}
	b.WriteString(" Z")
	return b.String()
}

// smoothOpen 用二次曲线连接一条开放折线，适合画波浪线。
func smoothOpen(points [][2]float64) string {
	if len(points) < 2 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "M %.1f %.1f", points[0][0], points[0][1])
	for i := 1; i < len(points)-1; i++ {
		p := points[i]
		next := points[i+1]
		midX := (p[0] + next[0]) / 2
		midY := (p[1] + next[1]) / 2
		fmt.Fprintf(&b, " Q %.1f %.1f %.1f %.1f", p[0], p[1], midX, midY)
	}
	last := points[len(points)-1]
	fmt.Fprintf(&b, " T %.1f %.1f", last[0], last[1])
	return b.String()
}

func roughCircle(cx, cy, radius float64, h uint64) string {
	const n = 14
	points := make([][2]float64, 0, n)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / n
		jitter := 0.97 + float64((h>>(uint(i)*3))&7)/100
		r := radius * jitter
		points = append(points, [2]float64{cx + r*math.Cos(angle), cy + r*math.Sin(angle)})
	}
	return smoothClosed(points)
}

// CoverSVG 生成一张安静的索引卡封面：淡色纸面、细边框、一小笔强调色和一组淡淡的线。
// 不包含标题文字，适合在卡片中作为背景。
func CoverSVG(seed, title, kind string) string {
	if seed == "" {
		seed = title
	}
	h := hash(seed + "|" + kind)
	bg := tints[h%uint64(len(tints))]
	line := "#d9d6cb"
	surface := "#fbfaf6"

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 600" width="1200" height="600" role="img">`)
	if title != "" {
		fmt.Fprintf(&b, `<title>%s</title>`, esc(title))
	}
	fmt.Fprintf(&b, `<rect width="1200" height="600" fill="%s"/>`, bg)
	fmt.Fprintf(&b, `<rect x="90" y="56" width="1020" height="488" rx="18" fill="%s" opacity="0.72" stroke="%s" stroke-width="2"/>`, surface, line)

	accentLen := 44 + float64(h%3)*14
	fmt.Fprintf(&b, `<path d="M 140 100 L %.0f 100" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`, 140+accentLen, accentColor)

	cx := 930 + float64(h%3)*36
	cy := 250 + float64(h%4)*24
	fmt.Fprintf(&b, `<circle cx="%.0f" cy="%.0f" r="112" fill="none" stroke="%s" stroke-width="2" opacity="0.55"/>`, cx, cy, line)
	fmt.Fprintf(&b, `<path d="M 140 500 L 370 498" fill="none" stroke="%s" stroke-width="2" stroke-linecap="round" opacity="0.75"/>`, line)
	b.WriteString(`</svg>`)
	return b.String()
}

// AvatarSVG 生成一个极简手绘头像：淡色纸面、手绘圆、昵称首字。
func AvatarSVG(seed string) string {
	if seed == "" {
		seed = "folio"
	}
	h := hash(seed)
	bg := tints[h%uint64(len(tints))]
	color := inkColor
	if h%4 == 0 {
		color = accentColor
	}
	glyph := firstGlyph(seed)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 240" width="240" height="240" role="img">`)
	fmt.Fprintf(&b, `<title>%s</title>`, esc(seed))
	fmt.Fprintf(&b, `<rect width="240" height="240" fill="%s"/>`, bg)
	fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/>`,
		roughCircle(120, 120, 98, h), inkColor)
	fmt.Fprintf(&b, `<text x="120" y="145" text-anchor="middle" font-family="Hannotate SC, HanziPen SC, Kaiti SC, STKaiti, serif" font-size="108" font-weight="500" fill="%s">%s</text>`,
		color, esc(glyph))
	b.WriteString(`</svg>`)
	return b.String()
}
