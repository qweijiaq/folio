package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Timeline 返回按发布时间倒序排列的公开内容，用于「发文时间线」页面。
// 文档会排除搜索隐藏的目录页和私密文档，文章与文档可以混合展示。
func (a *API) Timeline(c *gin.Context) {
	page, size := pageParams(c)
	kind := c.Query("kind")
	if kind != "" && kind != models.KindArticle && kind != models.KindDoc {
		bad(c, "未知的内容类型")
		return
	}

	q := a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished)
	switch kind {
	case models.KindArticle:
		q = q.Where("kind = ?", models.KindArticle)
	case models.KindDoc:
		q = q.Where("kind = ? AND search_hidden = ?", models.KindDoc, false)
	default:
		q = q.Where("(kind = ? OR (kind = ? AND search_hidden = ?))",
			models.KindArticle, models.KindDoc, false)
	}
	if hidden := a.hiddenDocIDs(nil); len(hidden) > 0 {
		q = q.Where("id NOT IN ?", hidden)
	}

	var total int64
	q.Count(&total)

	var docs []models.Document
	err := q.Preload("Category").Preload("Series").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).
		Order("COALESCE(published_at, created_at) DESC, id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&docs).Error
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取时间线失败")
		return
	}
	blankContent(docs)
	c.JSON(http.StatusOK, gin.H{"items": docs, "total": total, "page": page, "pageSize": size})
}
