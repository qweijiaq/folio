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

func (a *API) storedAIKey(userID uint, provider string) (string, bool, error) {
	var row models.AIProviderKey
	if err := a.DB.Where("user_id = ? AND provider = ?", userID, provider).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	key, err := secure.Decrypt(a.Cfg.Secret, row.Key)
	if err != nil {
		return "", false, err
	}
	return key, true, nil
}

func (a *API) AIKeys(c *gin.Context) {
	user := currentUser(c)
	var rows []models.AIProviderKey
	a.DB.Select("provider").Where("user_id = ?", user.ID).Find(&rows)
	saved := make(map[string]bool, len(rows))
	for _, row := range rows {
		saved[row.Provider] = true
	}
	c.JSON(http.StatusOK, gin.H{"saved": saved})
}

func (a *API) AISaveKey(c *gin.Context) {
	var in struct {
		Provider string `json:"provider"`
		APIKey   string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.APIKey = strings.TrimSpace(in.APIKey)
	if _, ok := a.aiProviderByID(in.Provider); !ok {
		bad(c, "不支持这个服务商")
		return
	}
	if len(in.APIKey) < 10 || len(in.APIKey) > 512 {
		bad(c, "API Key 长度不正确")
		return
	}
	sealed, err := secure.Encrypt(a.Cfg.Secret, in.APIKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, "加密 API Key 失败")
		return
	}
	user := currentUser(c)
	var row models.AIProviderKey
	err = a.DB.Where("user_id = ? AND provider = ?", user.ID, in.Provider).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusInternalServerError, "读取 API Key 失败")
		return
	}
	if row.ID == 0 {
		row = models.AIProviderKey{UserID: user.ID, Provider: in.Provider, Key: sealed}
	} else {
		row.Key = sealed
	}
	if err := a.DB.Save(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存 API Key 失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": in.Provider, "saved": true})
}

func (a *API) AIDeleteKey(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	if _, ok := a.aiProviderByID(provider); !ok {
		bad(c, "不支持这个服务商")
		return
	}
	user := currentUser(c)
	if err := a.DB.Where("user_id = ? AND provider = ?", user.ID, provider).Delete(&models.AIProviderKey{}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除 API Key 失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": provider, "saved": false})
}
