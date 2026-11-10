package art

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
)

// CoverPNG 生成 1200x630 的分享卡片底图。与 SVG 封面同一套极简索引卡语言，
// 不放标题文字，避免依赖字体，适合作为 og:image 的稳定图片。
func CoverPNG(seed, kind string) ([]byte, error) {
	if seed == "" {
		seed = "folio"
	}
	h := hash(seed + "|" + kind)
	bg := parseHex(tints[h%uint64(len(tints))])
	surface := blend(bg, parseHex("#fbfaf6"), 0.78)
	line := parseHex("#d9d6cb")
	accent := parseHex(accentColor)

	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	card := image.Rect(52, 44, 1148, 586)
	draw.Draw(img, card, &image.Uniform{C: surface}, image.Point{}, draw.Src)
	drawLine(img, 52, 44, 1148, 44, 2, line)
	drawLine(img, 52, 586, 1148, 586, 2, line)
	drawLine(img, 52, 44, 52, 586, 2, line)
	drawLine(img, 1148, 44, 1148, 586, 2, line)

	accentLen := 44 + float64(h%3)*14
	drawLine(img, 100, 96, 100+accentLen, 96, 3, accent)

	cx := 910 + float64(h%3)*36
	cy := 260 + float64(h%4)*20
	drawCircle(img, cx, cy, 120, 2, line)
	drawLine(img, 100, 482, 330, 480, 2, line)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func blend(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-t) + float64(y)*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 255}
}

func parseHex(hex string) color.RGBA {
	if len(hex) != 7 || hex[0] != '#' {
		return color.RGBA{R: 244, G: 243, B: 238, A: 255}
	}
	r, _ := strconv.ParseUint(hex[1:3], 16, 8)
	g, _ := strconv.ParseUint(hex[3:5], 16, 8)
	b, _ := strconv.ParseUint(hex[5:7], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func drawCircle(img *image.RGBA, cx, cy, radius float64, thickness int, c color.RGBA) {
	const segments = 160
	points := make([][2]float64, 0, segments+1)
	for i := 0; i <= segments; i++ {
		angle := 2 * math.Pi * float64(i) / segments
		points = append(points, [2]float64{cx + radius*math.Cos(angle), cy + radius*math.Sin(angle)})
	}
	drawPolyline(img, points, thickness, c)
}

func drawPolyline(img *image.RGBA, points [][2]float64, thickness int, c color.RGBA) {
	for i := 1; i < len(points); i++ {
		drawLine(img, points[i-1][0], points[i-1][1], points[i][0], points[i][1], thickness, c)
	}
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 float64, thickness int, c color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps < 1 {
		steps = 1
	}
	radius := float64(thickness) / 2
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		drawDot(img, x1+dx*t, y1+dy*t, radius, c)
	}
}

func drawDot(img *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	minX := int(math.Floor(cx - radius))
	maxX := int(math.Ceil(cx + radius))
	minY := int(math.Floor(cy - radius))
	maxY := int(math.Ceil(cy + radius))
	r2 := radius * radius
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
