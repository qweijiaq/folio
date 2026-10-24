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
	}
}

func (a *API) AdminSettings(c *gin.Context) {
	setting := a.siteSetting()
	c.JSON(http.StatusOK, gin.H{"settings": setting})
}

func (a *API) AdminSettingsUpdate(c *gin.Context) {
	var in struct {
		SiteName      string `json:"siteName"`
		SiteDesc      string `json:"siteDesc"`
		SiteOwner     string `json:"siteOwner"`
		AllowRegister bool   `json:"allowRegister"`
		LoginCaptcha  bool   `json:"loginCaptcha"`
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

	var setting models.SiteSetting
	if err := a.DB.First(&setting, 1).Error; err != nil {
		setting = models.SiteSetting{Base: models.Base{ID: 1}}
	}
	setting.SiteName = in.SiteName
	setting.SiteDesc = in.SiteDesc
	setting.SiteOwner = in.SiteOwner
	setting.AllowRegister = in.AllowRegister
	setting.LoginCaptcha = in.LoginCaptcha
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存站点设置失败")
		return
	}
	a.logAction(c, "更新设置", "site", setting.ID, "更新站点名称、注册开关或图形验证码")
	c.JSON(http.StatusOK, gin.H{"settings": setting, "site": a.siteInfo()})
}
