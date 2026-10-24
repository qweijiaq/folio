package handlers

import (
	"net/http"
	"strings"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *API) noticesFor(userID uint, limit int) ([]gin.H, int64) {
	var notices []models.GlobalNotice
	a.DB.Order("id DESC").Limit(limit).Find(&notices)
	ids := make([]uint, 0, len(notices))
	for _, n := range notices {
		ids = append(ids, n.ID)
	}
	read := map[uint]bool{}
	if len(ids) > 0 {
		var rows []models.NoticeRead
		a.DB.Where("user_id = ? AND notice_id IN ?", userID, ids).Find(&rows)
		for _, row := range rows {
			read[row.NoticeID] = true
		}
	}
	items := make([]gin.H, 0, len(notices))
	for _, n := range notices {
		items = append(items, gin.H{
			"id":            n.ID,
			"title":         n.Title,
			"content":       n.Content,
			"link":          n.Link,
			"createdBy":     n.CreatedBy,
			"createdByName": n.CreatedByName,
			"createdAt":     n.CreatedAt,
			"read":          read[n.ID],
		})
	}
	return items, a.unreadNoticeCount(userID)
}

func (a *API) unreadNoticeCount(userID uint) int64 {
	var readIDs []uint
	a.DB.Model(&models.NoticeRead{}).Where("user_id = ?", userID).Pluck("notice_id", &readIDs)
	query := a.DB.Model(&models.GlobalNotice{})
	if len(readIDs) > 0 {
		query = query.Where("id NOT IN ?", readIDs)
	}
	var unread int64
	query.Count(&unread)
	return unread
}

func (a *API) Notices(c *gin.Context) {
	me := currentUser(c)
	items, unread := a.noticesFor(me.ID, 20)
	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unread})
}

func (a *API) NoticesRead(c *gin.Context) {
	me := currentUser(c)
	var ids []uint
	a.DB.Model(&models.GlobalNotice{}).Pluck("id", &ids)
	for _, id := range ids {
		var existing models.NoticeRead
		a.DB.Where("notice_id = ? AND user_id = ?", id, me.ID).FirstOrCreate(&existing, models.NoticeRead{
			NoticeID: id,
			UserID:   me.ID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) AdminNotices(c *gin.Context) {
	var notices []models.GlobalNotice
	a.DB.Order("id DESC").Limit(100).Find(&notices)
	c.JSON(http.StatusOK, gin.H{"items": notices})
}

type noticeInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link"`
}

func (a *API) AdminNoticeCreate(c *gin.Context) {
	var in noticeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Title = trim(in.Title)
	in.Content = trim(in.Content)
	in.Link = trim(in.Link)
	if runeLen(in.Title) < 1 || runeLen(in.Title) > 80 {
		bad(c, "公告标题需在 1-80 个字符之间")
		return
	}
	if runeLen(in.Content) < 1 || runeLen(in.Content) > 1000 {
		bad(c, "公告内容需在 1-1000 个字符之间")
		return
	}
	if runeLen(in.Link) > 255 {
		bad(c, "公告链接最多 255 个字符")
		return
	}
	if in.Link != "" &&
		!strings.HasPrefix(in.Link, "/") &&
		!strings.HasPrefix(in.Link, "https://") &&
		!strings.HasPrefix(in.Link, "http://") {
		bad(c, "公告链接需以 / 或 http(s):// 开头")
		return
	}
	me := currentUser(c)
	notice := models.GlobalNotice{
		Title:         in.Title,
		Content:       in.Content,
		Link:          in.Link,
		CreatedBy:     me.ID,
		CreatedByName: me.Nickname,
	}
	if err := a.DB.Create(&notice).Error; err != nil {
		fail(c, http.StatusInternalServerError, "发布公告失败")
		return
	}
	a.logAction(c, "发布公告", "notice", notice.ID, notice.Title)
	c.JSON(http.StatusCreated, gin.H{"notice": notice})
}

func (a *API) AdminNoticeDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var notice models.GlobalNotice
	if err := a.DB.First(&notice, id).Error; err != nil {
		fail(c, http.StatusNotFound, "公告不存在")
		return
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("notice_id = ?", notice.ID).Delete(&models.NoticeRead{}).Error; err != nil {
			return err
		}
		return tx.Delete(&notice).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除公告失败")
		return
	}
	a.logAction(c, "删除公告", "notice", notice.ID, notice.Title)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
