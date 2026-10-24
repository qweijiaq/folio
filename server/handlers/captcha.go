package handlers

import (
	"image/color"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)

// CaptchaImage 生成一张 4 位图形验证码。关闭时返回 enabled=false，前端据此隐藏输入框。
func (a *API) CaptchaImage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !a.loginCaptchaEnabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	driver := base64Captcha.NewDriverString(
		44,
		132,
		2,
		base64Captcha.OptionShowSlimeLine,
		4,
		"23456789abcdefghjkmnpqrstuvwxyz",
		&color.RGBA{R: 246, G: 243, B: 234, A: 255},
		nil,
		nil,
	)
	captcha := base64Captcha.NewCaptcha(driver, a.Captcha)
	id, image, _, err := captcha.Generate()
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成图形验证码失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":   true,
		"captchaId": id,
		"image":     image,
	})
}

// requireCaptcha 在开启图形验证码时校验并消费一次验证码。
func (a *API) requireCaptcha(c *gin.Context, id, code string) bool {
	if !a.loginCaptchaEnabled() {
		return true
	}
	id = strings.TrimSpace(id)
	code = strings.TrimSpace(code)
	if id == "" || code == "" {
		bad(c, "请先完成图形验证码")
		return false
	}
	if a.Captcha == nil || !a.Captcha.Verify(id, code, true) {
		bad(c, "图形验证码错误，请重试")
		return false
	}
	return true
}
