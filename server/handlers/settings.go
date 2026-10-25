package handlers

import (
	"net/http"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

// siteSetting 返回当前站点设置；老库还没有记录时回退到环境变量。
func (a *API) siteSetting() models.SiteSetting {
	var setting models.SiteSetting
	if err := a.DB.First(&setting, 1).Error; err == nil && setting.ID != 0 {
		return setting
	}
	return models.SiteSetting{
		Base:          models.Base{ID: 1},
		SiteName:      a.Cfg.SiteName,
		SiteDesc:      a.Cfg.SiteDesc,
		SiteOwner:     a.Cfg.SiteOwner,
		AllowRegister: true,
		LoginCaptcha:  a.Cfg.LoginCaptcha,
	}
}

func settingText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (a *API) loginCaptchaEnabled() bool {
	return a.siteSetting().LoginCaptcha
}

// qqLoginEnabled 同时要求环境变量凭据完整，并且后台开关没有关闭。
// 老库没有 qq_login 字段值时，沿用“配置了凭据就启用”的行为。
func (a *API) qqLoginEnabled() bool {
	if !a.Cfg.QQEnabled() {
		return false
	}
	setting := a.siteSetting()
	if setting.QQLogin == nil {
		return true
	}
	return *setting.QQLogin
}

func (a *API) allowRegistration() bool {
	return a.siteSetting().AllowRegister
}

func (a *API) siteInfo() gin.H {
	setting := a.siteSetting()
	return gin.H{
		"name":          settingText(setting.SiteName, a.Cfg.SiteName),
		"description":   settingText(setting.SiteDesc, a.Cfg.SiteDesc),
		"owner":         settingText(setting.SiteOwner, a.Cfg.SiteOwner),
		"year":          time.Now().Year(),
		"allowRegister": setting.AllowRegister,
		"icpBeian":      setting.ICPBeian,
		"policeBeian":   setting.PoliceBeian,
	}
}

func (a *API) AdminSettings(c *gin.Context) {
	setting := a.siteSetting()
	c.JSON(http.StatusOK, gin.H{
		"settings":     setting,
		"qqConfigured": a.Cfg.QQEnabled(),
		"qqLogin":      a.qqLoginEnabled(),
	})
}

func (a *API) AdminAccess(c *gin.Context) {
	setting := a.siteSetting()
	c.JSON(http.StatusOK, gin.H{
		"allowRegister": setting.AllowRegister,
		"qqLogin":       a.qqLoginEnabled(),
		"qqConfigured":  a.Cfg.QQEnabled(),
	})
}

func (a *API) AdminAccessUpdate(c *gin.Context) {
	var in struct {
		AllowRegister bool `json:"allowRegister"`
		QQLogin       bool `json:"qqLogin"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if in.QQLogin && !a.Cfg.QQEnabled() {
		bad(c, "请先在环境变量配置 QQ 互联凭据，再开启 QQ 登录")
		return
	}
	setting := a.siteSetting()
	setting.AllowRegister = in.AllowRegister
	setting.QQLogin = &in.QQLogin
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存注册设置失败")
		return
	}
	a.logAction(c, "更新注册设置", "site", setting.ID, "更新公开注册与 QQ 登录开关")
	c.JSON(http.StatusOK, gin.H{
		"access": gin.H{
			"allowRegister": setting.AllowRegister,
			"qqLogin":       a.qqLoginEnabled(),
			"qqConfigured":  a.Cfg.QQEnabled(),
		},
		"site": a.siteInfo(),
	})
}

func (a *API) AdminSettingsUpdate(c *gin.Context) {
	var in struct {
		SiteName      string  `json:"siteName"`
		SiteDesc      string  `json:"siteDesc"`
		SiteOwner     string  `json:"siteOwner"`
		AllowRegister *bool   `json:"allowRegister"`
		LoginCaptcha  bool    `json:"loginCaptcha"`
		ICPBeian      *string `json:"icpBeian"`
		PoliceBeian   *string `json:"policeBeian"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.SiteName = trim(in.SiteName)
	in.SiteDesc = trim(in.SiteDesc)
	in.SiteOwner = trim(in.SiteOwner)
	if runeLen(in.SiteName) < 1 || runeLen(in.SiteName) > 32 {
		bad(c, "站点名称需在 1-32 个字符之间")
		return
	}
	if runeLen(in.SiteDesc) > 120 {
		bad(c, "站点描述最多 120 个字符")
		return
	}
	if runeLen(in.SiteOwner) > 32 {
		bad(c, "维护者名称最多 32 个字符")
		return
	}
	if in.ICPBeian != nil {
		*in.ICPBeian = trim(*in.ICPBeian)
		if runeLen(*in.ICPBeian) > 64 {
			bad(c, "ICP 备案号最多 64 个字符")
			return
		}
	}
	if in.PoliceBeian != nil {
		*in.PoliceBeian = trim(*in.PoliceBeian)
		if runeLen(*in.PoliceBeian) > 64 {
			bad(c, "公安备案号最多 64 个字符")
			return
		}
	}

	var setting models.SiteSetting
	if err := a.DB.First(&setting, 1).Error; err != nil {
		setting = models.SiteSetting{Base: models.Base{ID: 1}}
	}
	setting.SiteName = in.SiteName
	setting.SiteDesc = in.SiteDesc
	setting.SiteOwner = in.SiteOwner
	if in.AllowRegister != nil {
		setting.AllowRegister = *in.AllowRegister
	}
	setting.LoginCaptcha = in.LoginCaptcha
	if in.ICPBeian != nil {
		setting.ICPBeian = *in.ICPBeian
	}
	if in.PoliceBeian != nil {
		setting.PoliceBeian = *in.PoliceBeian
	}
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存站点设置失败")
		return
	}
	a.logAction(c, "更新设置", "site", setting.ID, "更新站点名称、注册开关或图形验证码")
	c.JSON(http.StatusOK, gin.H{"settings": setting, "site": a.siteInfo()})
}
