package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

func defaultUserSetting(userID uint) models.UserSetting {
	return models.UserSetting{
		UserID:             userID,
		OpenFollow:         true,
		OpenFans:           true,
		OpenPrivateChat:    true,
		OpenCommentMessage: true,
		OpenDiggMessage:    true,
		HomeStyle:          "cover",
	}
}

// userSetting 在没有记录时返回默认值，保证老用户与刚注册用户都有合理设置。
func (a *API) userSetting(userID uint) models.UserSetting {
	var setting models.UserSetting
	if err := a.DB.Where("user_id = ?", userID).First(&setting).Error; err == nil {
		if setting.HomeStyle == "" {
			setting.HomeStyle = "cover"
		}
		return setting
	}
	return defaultUserSetting(userID)
}

func publicUserSettings(setting models.UserSetting) gin.H {
	return gin.H{
		"openFollow":      setting.OpenFollow,
		"openFans":        setting.OpenFans,
		"openPrivateChat": setting.OpenPrivateChat,
		"homeStyle":       setting.HomeStyle,
	}
}

func (a *API) MeSettings(c *gin.Context) {
	me := currentUser(c)
	c.JSON(http.StatusOK, gin.H{"settings": a.userSetting(me.ID)})
}

func (a *API) MeSettingsUpdate(c *gin.Context) {
	me := currentUser(c)
	var in struct {
		OpenFollow         bool   `json:"openFollow"`
		OpenFans           bool   `json:"openFans"`
		OpenPrivateChat    bool   `json:"openPrivateChat"`
		OpenCommentMessage bool   `json:"openCommentMessage"`
		OpenDiggMessage    bool   `json:"openDiggMessage"`
		HomeStyle          string `json:"homeStyle"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.HomeStyle = trim(in.HomeStyle)
	if in.HomeStyle == "" {
		in.HomeStyle = "cover"
	}
	if in.HomeStyle != "cover" && in.HomeStyle != "compact" && in.HomeStyle != "plain" {
		bad(c, "主页样式不正确")
		return
	}
	setting := a.userSetting(me.ID)
	setting.OpenFollow = in.OpenFollow
	setting.OpenFans = in.OpenFans
	setting.OpenPrivateChat = in.OpenPrivateChat
	setting.OpenCommentMessage = in.OpenCommentMessage
	setting.OpenDiggMessage = in.OpenDiggMessage
	setting.HomeStyle = in.HomeStyle
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存隐私设置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": setting})
}
