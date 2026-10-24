package art

import (
	"bytes"
	"image/png"
	"testing"
)

func TestCoverPNGSize(t *testing.T) {
	data, err := CoverPNG("article-1", "article")
	if err != nil {
		t.Fatalf("生成分享图失败: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("分享图不是有效 PNG: %v", err)
	}
	if cfg.Width != 1200 || cfg.Height != 630 {
		t.Fatalf("分享图尺寸不正确: %dx%d", cfg.Width, cfg.Height)
	}
}
