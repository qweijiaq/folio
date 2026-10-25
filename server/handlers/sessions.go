package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func deviceLabel(ua string) string {
	lower := strings.ToLower(ua)
	browser := ""
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		browser = "Opera"
	case strings.Contains(lower, "chrome/") || strings.Contains(lower, "crios/"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox/"):
		browser = "Firefox"
	case strings.Contains(lower, "safari/"):
		browser = "Safari"
	case strings.Contains(lower, "curl/"):
		browser = "curl"
	}
	osName := ""
	switch {
	case strings.Contains(lower, "iphone"):
		osName = "iPhone"
	case strings.Contains(lower, "ipad"):
		osName = "iPad"
	case strings.Contains(lower, "android"):
		osName = "Android"
	case strings.Contains(lower, "windows"):
		osName = "Windows"
	case strings.Contains(lower, "mac os") || strings.Contains(lower, "macintosh"):
		osName = "macOS"
	case strings.Contains(lower, "linux"):
		osName = "Linux"
	}
	if browser == "" && osName == "" {
		return "未知设备"
	}
	if browser == "" {
		return osName
	}
	if osName == "" {
		return browser
	}
	return browser + " · " + osName
}

// sessionAllowed 校验 token 对应的会话仍有效；旧版本签发、没有会话记录的 token
// 会在第一次使用时补一条记录，便于之后的设备管理。
func (a *API) sessionAllowed(c *gin.Context, u *models.User, claims *tokenClaims) bool {
	if claims == nil || claims.ID == "" {
		return false
	}
	var session models.UserSession
	err := a.DB.Where("token_id = ?", claims.ID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		if claims.ExpiresAt != nil {
			expiresAt = claims.ExpiresAt.Time
		}
		ua := c.GetHeader("User-Agent")
		session = models.UserSession{
			UserID:     u.ID,
			TokenID:    claims.ID,
			IP:         clipText(c.ClientIP(), 64),
			UserAgent:  clipText(ua, 255),
			Device:     clipText(deviceLabel(ua), 64),
			LastSeenAt: time.Now(),
			ExpiresAt:  expiresAt,
		}
		if err := a.DB.Create(&session).Error; err != nil {
			if a.DB.Where("token_id = ?", claims.ID).First(&session).Error != nil {
				return false
			}
		}
	} else if err != nil {
		return false
	}
	if session.UserID != u.ID || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return false
	}
	if time.Since(session.LastSeenAt) > time.Minute {
		a.DB.Model(&models.UserSession{}).Where("id = ?", session.ID).UpdateColumn("last_seen_at", time.Now())
	}
	return true
}

func sessionPayload(session models.UserSession, currentTokenID string) gin.H {
	return gin.H{
		"id":         session.ID,
		"ip":         session.IP,
		"userAgent":  session.UserAgent,
		"device":     session.Device,
		"current":    session.TokenID == currentTokenID,
		"lastSeenAt": session.LastSeenAt,
		"createdAt":  session.CreatedAt,
		"expiresAt":  session.ExpiresAt,
	}
}

func (a *API) MySessions(c *gin.Context) {
	me := currentUser(c)
	claims := currentClaims(c)
	currentTokenID := ""
	if claims != nil {
		currentTokenID = claims.ID
	}
	var sessions []models.UserSession
	a.DB.Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", me.ID, time.Now()).
		Order("last_seen_at DESC, id DESC").Find(&sessions)
	items := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, sessionPayload(session, currentTokenID))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) SessionRevoke(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	me := currentUser(c)
	claims := currentClaims(c)
	var session models.UserSession
	if err := a.DB.Where("id = ? AND user_id = ?", id, me.ID).First(&session).Error; err != nil {
		fail(c, http.StatusNotFound, "登录会话不存在")
		return
	}
	if session.RevokedAt == nil {
		a.DB.Model(&session).UpdateColumn("revoked_at", time.Now())
	}
	current := claims != nil && claims.ID == session.TokenID
	if current {
		a.Hub.disconnectToken(me.ID, session.TokenID)
		if claims != nil && claims.ExpiresAt != nil {
			_ = a.Cache.Blacklist(c.Request.Context(), claims.ID, time.Until(claims.ExpiresAt.Time))
		}
	} else {
		a.Hub.disconnectToken(me.ID, session.TokenID)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "current": current})
}

func (a *API) SessionsRevokeOthers(c *gin.Context) {
	me := currentUser(c)
	claims := currentClaims(c)
	if claims == nil {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	result := a.DB.Model(&models.UserSession{}).
		Where("user_id = ? AND token_id <> ? AND revoked_at IS NULL", me.ID, claims.ID).
		UpdateColumn("revoked_at", time.Now())
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, "退出其他设备失败")
		return
	}
	a.Hub.disconnectExcept(me.ID, claims.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "revoked": result.RowsAffected})
}

// revokeAllUserSessions 让用户所有旧 token 失效，包括没有会话记录的旧版 token。
func (a *API) revokeAllUserSessions(userID uint) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", userID).
			UpdateColumn("auth_epoch", gorm.Expr("auth_epoch + 1")).Error; err != nil {
			return err
		}
		return tx.Model(&models.UserSession{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			UpdateColumn("revoked_at", time.Now()).Error
	})
}

func (a *API) SessionsRevokeAll(c *gin.Context) {
	me := currentUser(c)
	if err := a.revokeAllUserSessions(me.ID); err != nil {
		fail(c, http.StatusInternalServerError, "退出所有设备失败")
		return
	}
	a.Hub.disconnectAll(me.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
