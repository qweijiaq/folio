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

// CoverSVG 生成一张极简手绘封面：一层淡色纸面、一个几何线稿和一小笔强调色。
// 不包含标题文字，适合在卡片中作为安静的装饰。
func CoverSVG(seed, title, kind string) string {
	if seed == "" {
		seed = title
	}
	h := hash(seed + "|" + kind)
	bg := tints[h%uint64(len(tints))]

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 480" width="1200" height="480" role="img">`)
	if title != "" {
		fmt.Fprintf(&b, `<title>%s</title>`, esc(title))
	}
	fmt.Fprintf(&b, `<rect width="1200" height="480" fill="%s"/>`, bg)

	stroke := fmt.Sprintf(`fill="none" stroke="%s" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"`, inkColor)
	accent := fmt.Sprintf(`fill="none" stroke="%s" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"`, accentColor)
	cx, cy := 600.0, 240.0

	switch h % 4 {
	case 0:
		fmt.Fprintf(&b, `<path d="%s" %s/>`, roughCircle(cx, cy, 96, h), stroke)
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" %s/>`, cx+58, cy+112, cx+178, cy+100, accent)
	case 1:
		wave := [][2]float64{
			{cx - 190, cy + 34}, {cx - 96, cy - 58}, {cx, cy + 26}, {cx + 96, cy - 58}, {cx + 190, cy + 34},
		}
		fmt.Fprintf(&b, `<path d="%s" %s/>`, smoothOpen(wave), stroke)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="11" %s/>`, cx+156, cy-84, accent)
	case 2:
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" %s/>`, cx-172, cy-58, cx+118, cy-56, stroke)
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" %s/>`, cx-172, cy, cx+34, cy+2, stroke)
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" %s/>`, cx-172, cy+58, cx-48, cy+56, stroke)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="9" %s/>`, cx+148, cy+56, accent)
	default:
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f L %.1f %.1f" %s/>`, cx-138, cy+78, cx, cy-86, cx+138, cy+78, stroke)
		fmt.Fprintf(&b, `<path d="M %.1f %.1f L %.1f %.1f" %s/>`, cx-38, cy+78, cx+62, cy+76, accent)
	}

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
