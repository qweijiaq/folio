package handlers

import (
	"errors"
	"net/http"
	"strings"

	"folio/server/models"
	"folio/server/secure"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type aiSummaryConfig struct {
	Provider qaProviderDef
	APIKey   string
	Model    string
	Source   string
}

func (a *API) aiSummaryStatus() gin.H {
	var row models.AISummarySetting
	err := a.DB.First(&row, 1).Error
	status := gin.H{
		"enabled":   false,
		"provider":  "anthropic",
		"model":     a.Cfg.AIModel,
		"hasKey":    false,
		"source":    "none",
		"providers": a.aiProviderInfos(),
	}
	if err == nil {
		status["provider"] = row.Provider
		status["model"] = row.Model
		status["hasKey"] = strings.TrimSpace(row.APIKey) != ""
		status["source"] = "database"
		if row.Enabled && row.APIKey != "" {
			if _, err := secure.Decrypt(a.Cfg.Secret, row.APIKey); err != nil {
				status["error"] = "已保存的 API Key 无法解密，请重新填写"
			} else if _, ok := a.aiProviderByID(row.Provider); ok {
				status["enabled"] = true
			}
		}
		return status
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		status["error"] = "读取 AI 摘要配置失败"
		return status
	}
	if a.Cfg.AIEnabled() {
		status["enabled"] = true
		status["hasKey"] = true
		status["source"] = "env"
	}
	return status
}

func (a *API) aiSummaryConfig() (aiSummaryConfig, error) {
	var row models.AISummarySetting
	err := a.DB.First(&row, 1).Error
	if err == nil {
		if !row.Enabled {
			return aiSummaryConfig{}, errors.New("AI 摘要未启用")
		}
		if strings.TrimSpace(row.APIKey) == "" {
			return aiSummaryConfig{}, errors.New("AI 摘要还没有配置 API Key")
		}
		key, err := secure.Decrypt(a.Cfg.Secret, row.APIKey)
		if err != nil {
			return aiSummaryConfig{}, errors.New("已保存的 API Key 无法解密，请重新填写")
		}
		provider, ok := a.aiProviderByID(row.Provider)
		if !ok {
			return aiSummaryConfig{}, errors.New("AI 摘要服务商配置无效")
		}
		model := strings.TrimSpace(row.Model)
		if model == "" && len(provider.Models) > 0 {
			model = provider.Models[0].ID
		}
		return aiSummaryConfig{Provider: provider, APIKey: key, Model: model, Source: "database"}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return aiSummaryConfig{}, errors.New("读取 AI 摘要配置失败")
	}
	if a.Cfg.AIEnabled() {
		provider, _ := a.aiProviderByID("anthropic")
		model := strings.TrimSpace(a.Cfg.AIModel)
		if model == "" && len(provider.Models) > 0 {
			model = provider.Models[0].ID
		}
		return aiSummaryConfig{Provider: provider, APIKey: a.Cfg.AIAPIKey, Model: model, Source: "env"}, nil
	}
	return aiSummaryConfig{}, errors.New("AI 摘要未配置，请超级管理员在后台设置")
}

func (a *API) AdminAISummary(c *gin.Context) {
	c.JSON(http.StatusOK, a.aiSummaryStatus())
}

func (a *API) AdminAISummaryUpdate(c *gin.Context) {
	var in struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Model = strings.TrimSpace(in.Model)
	in.APIKey = strings.TrimSpace(in.APIKey)
	provider, ok := a.aiProviderByID(in.Provider)
	if !ok {
		bad(c, "不支持这个 AI 服务商")
		return
	}
	if in.Model == "" && len(provider.Models) > 0 {
		in.Model = provider.Models[0].ID
	}
	if !qaModelValid(in.Model) {
		bad(c, "模型名称不合法")
		return
	}
	if in.APIKey != "" && (len(in.APIKey) < 10 || len(in.APIKey) > 512) {
		bad(c, "API Key 长度不正确")
		return
	}

	var row models.AISummarySetting
	err := a.DB.First(&row, 1).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusInternalServerError, "读取 AI 摘要设置失败")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.AISummarySetting{Base: models.Base{ID: 1}}
	}
	if in.APIKey != "" {
		sealed, err := secure.Encrypt(a.Cfg.Secret, in.APIKey)
		if err != nil {
			fail(c, http.StatusInternalServerError, "加密 API Key 失败")
			return
		}
		row.APIKey = sealed
	}
	if in.Enabled && strings.TrimSpace(row.APIKey) == "" {
		bad(c, "开启 AI 摘要前请先填写 API Key")
		return
	}
	row.Provider = in.Provider
	row.Model = in.Model
	row.Enabled = in.Enabled
	if err := a.DB.Save(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存 AI 摘要设置失败")
		return
	}
	a.logAction(c, "更新 AI 摘要设置", "ai_summary", row.ID, "服务商："+provider.Name+"，状态："+map[bool]string{true: "开启", false: "关闭"}[row.Enabled])
	c.JSON(http.StatusOK, a.aiSummaryStatus())
}

func (a *API) AdminAISummaryDelete(c *gin.Context) {
	if err := a.DB.Where("id = ?", 1).Delete(&models.AISummarySetting{}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "清除 AI 摘要设置失败")
		return
	}
	a.logAction(c, "清除 AI 摘要设置", "ai_summary", 1, "删除网页配置的 AI 摘要服务")
	c.JSON(http.StatusOK, a.aiSummaryStatus())
}
