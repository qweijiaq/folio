package handlers

import (
	"log"
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

func clipText(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

// recordLog 写入一条操作日志。日志失败不影响主流程。
func (a *API) recordLog(c *gin.Context, actor *models.User, action, targetType string, targetID uint, detail string) {
	entry := models.OperationLog{
		Action:     clipText(trim(action), 32),
		TargetType: clipText(trim(targetType), 32),
		TargetID:   targetID,
		Detail:     clipText(trim(detail), 500),
		IP:         clipText(c.ClientIP(), 64),
	}
	if actor != nil {
		entry.UserID = actor.ID
		entry.Username = actor.Username
	}
	if userAgent := c.Request.UserAgent(); userAgent != "" {
		entry.UserAgent = clipText(userAgent, 255)
	}
	if err := a.DB.Create(&entry).Error; err != nil {
		log.Printf("写入操作日志失败: %v", err)
	}
}

func (a *API) logAction(c *gin.Context, action, targetType string, targetID uint, detail string) {
	a.recordLog(c, currentUser(c), action, targetType, targetID, detail)
}

func (a *API) AdminLogs(c *gin.Context) {
	page, size := pageParams(c)
	query := a.DB.Model(&models.OperationLog{})
	if action := trim(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if keyword := trim(c.Query("q")); keyword != "" {
		like := likePattern(keyword)
		query = query.Where(
			`username LIKE ? ESCAPE '!' OR action LIKE ? ESCAPE '!' OR detail LIKE ? ESCAPE '!'`,
			like, like, like,
		)
	}
	var total int64
	query.Count(&total)
	var logs []models.OperationLog
	query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&logs)
	c.JSON(http.StatusOK, gin.H{
		"items":    logs,
		"total":    total,
		"page":     page,
		"pageSize": size,
	})
}

// MyLoginRecords 让用户查看自己的登录与安全动作，不暴露其他用户的日志。
func (a *API) MyLoginRecords(c *gin.Context) {
	me := currentUser(c)
	var logs []models.OperationLog
	a.DB.Where("user_id = ? AND action IN ?", me.ID, []string{"登录", "注册账号", "重置密码", "修改密码", "绑定邮箱"}).
		Order("id DESC").Limit(12).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"items": logs})
}
