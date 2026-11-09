package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"folio/server/config"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 给所有响应补上基础安全头。CSP 允许站点自己的脚本、
// 内联样式（Vue 会写 style 属性）、站内 WebSocket，以及 Markdown 里的 https 图片。
func SecurityHeaders(cfg config.Config) gin.HandlerFunc {
	const csp = "default-src 'self'; img-src 'self' data: https:; media-src 'self' https:; " +
		"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' ws: wss:; " +
		"font-src 'self' data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'; object-src 'none'"
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Content-Security-Policy", csp)
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit 限制请求体大小：普通 JSON 2MB，图片上传为配置上限再加 1MB 的 multipart 余量。
func BodyLimit(maxUploadMB int) gin.HandlerFunc {
	if maxUploadMB < 1 {
		maxUploadMB = 10
	}
	jsonLimit := int64(2 << 20)
	uploadLimit := int64(maxUploadMB)*(1<<20) + (1 << 20)
	return func(c *gin.Context) {
		limit := jsonLimit
		if strings.HasPrefix(c.Request.URL.Path, "/api/images") {
			limit = uploadLimit
		}
		if c.Request.ContentLength > limit {
			fail(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("请求体不能超过 %dMB", limit>>20))
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// originAllowed 只放行站点自身、WebURL 和本地开发地址，避免反射任意 Origin。
func originAllowed(cfg config.Config, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	normalized := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
	for _, raw := range []string{cfg.SiteURL, cfg.WebURL} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		target, err := url.Parse(raw)
		if err != nil || target.Host == "" {
			continue
		}
		if strings.ToLower(target.Scheme+"://"+target.Host) == normalized {
			return true
		}
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
