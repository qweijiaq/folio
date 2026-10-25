package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

func (a *API) AIFeedback(c *gin.Context) {
	me := currentUser(c)
	var in struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Helpful  bool   `json:"helpful"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Provider = trim(in.Provider)
	in.Model = trim(in.Model)
	in.Question = trim(in.Question)
	in.Answer = trim(in.Answer)
	if in.Provider == "" || in.Model == "" || in.Question == "" || in.Answer == "" {
		bad(c, "反馈内容不完整")
		return
	}
	feedback := models.AIFeedback{
		UserID:   me.ID,
		Provider: clipText(in.Provider, 32),
		Model:    clipText(in.Model, 100),
		Question: clipText(in.Question, 500),
		Answer:   clipText(in.Answer, 800),
		Helpful:  in.Helpful,
	}
	if err := a.DB.Create(&feedback).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存反馈失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
