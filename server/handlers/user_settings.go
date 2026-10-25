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
		OpenFollowMessage:  true,
		OpenAccessMessage:  true,
		EmailNotify:        false,
		OpenCollect:        false,
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
		"openCollect":     setting.OpenCollect,
		"featuredDocID":   setting.FeaturedDocID,
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
		OpenFollowMessage  *bool  `json:"openFollowMessage"`
		OpenAccessMessage  *bool  `json:"openAccessMessage"`
		EmailNotify        *bool  `json:"emailNotify"`
		OpenCollect        bool   `json:"openCollect"`
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
	if in.OpenFollowMessage != nil {
		setting.OpenFollowMessage = *in.OpenFollowMessage
	}
	if in.OpenAccessMessage != nil {
		setting.OpenAccessMessage = *in.OpenAccessMessage
	}
	if in.EmailNotify != nil {
		setting.EmailNotify = *in.EmailNotify
	}
	setting.OpenCollect = in.OpenCollect
	setting.HomeStyle = in.HomeStyle
	followMessage := setting.OpenFollowMessage
	accessMessage := setting.OpenAccessMessage
	emailNotify := setting.EmailNotify
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存隐私设置失败")
		return
	}
	// 这两个字段带数据库默认值 true，GORM 创建时会把 false 当作零值省略并回填默认值，
	// 因此用保存前的目标值显式回写。
	if err := a.DB.Model(&setting).UpdateColumns(map[string]any{
		"open_follow_message": followMessage,
		"open_access_message": accessMessage,
		"email_notify":        emailNotify,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存通知设置失败")
		return
	}
	setting.OpenFollowMessage = followMessage
	setting.OpenAccessMessage = accessMessage
	setting.EmailNotify = emailNotify
	c.JSON(http.StatusOK, gin.H{"settings": setting})
}

// SetFeaturedDoc 设置个人主页置顶文章；传 0 取消置顶。
func (a *API) SetFeaturedDoc(c *gin.Context) {
	me := currentUser(c)
	var in struct {
		DocID uint `json:"docID"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	setting := a.userSetting(me.ID)
	if in.DocID == 0 {
		setting.FeaturedDocID = nil
	} else {
		var doc models.Document
		if err := a.DB.Where("id = ? AND author_id = ? AND kind = ? AND status = ?",
			in.DocID, me.ID, models.KindArticle, models.StatusPublished).First(&doc).Error; err != nil {
			bad(c, "只能置顶自己已发布的文章")
			return
		}
		id := doc.ID
		setting.FeaturedDocID = &id
	}
	if err := a.DB.Save(&setting).Error; err != nil {
		fail(c, http.StatusInternalServerError, "设置置顶失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": setting})
}
