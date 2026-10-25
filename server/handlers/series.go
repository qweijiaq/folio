package handlers

import (
	"net/http"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type seriesInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (a *API) seriesPayload(series models.Series, count int64) gin.H {
	return gin.H{
		"id":          series.ID,
		"title":       series.Title,
		"description": series.Description,
		"authorID":    series.AuthorID,
		"author":      toBrief(series.Author),
		"count":       count,
		"createdAt":   series.CreatedAt,
		"updatedAt":   series.UpdatedAt,
	}
}

func (a *API) seriesCount(seriesID uint) int64 {
	var count int64
	a.DB.Model(&models.Document{}).
		Where("series_id = ? AND kind = ? AND status = ?", seriesID, models.KindArticle, models.StatusPublished).
		Count(&count)
	return count
}

func (a *API) SeriesList(c *gin.Context) {
	var series []models.Series
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).Order("created_at DESC, id DESC").Find(&series)
	items := make([]gin.H, 0, len(series))
	for _, item := range series {
		items = append(items, a.seriesPayload(item, a.seriesCount(item.ID)))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) SeriesDetail(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var series models.Series
	if err := a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).First(&series, id).Error; err != nil {
		fail(c, http.StatusNotFound, "合集不存在")
		return
	}
	var articles []models.Document
	a.DB.Preload("Category").Preload("Series").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Where("series_id = ? AND kind = ? AND status = ?", id, models.KindArticle, models.StatusPublished).
		Order("series_order ASC, COALESCE(published_at, created_at) ASC, id ASC").Find(&articles)
	blankContent(articles)
	c.JSON(http.StatusOK, gin.H{
		"series": a.seriesPayload(series, int64(len(articles))),
		"items":  articles,
	})
}

func (a *API) MySeries(c *gin.Context) {
	me := currentUser(c)
	var series []models.Series
	query := a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	})
	if !isSuperAdmin(me) {
		query = query.Where("author_id = ?", me.ID)
	}
	query.Order("created_at DESC, id DESC").Find(&series)
	items := make([]gin.H, 0, len(series))
	for _, item := range series {
		items = append(items, a.seriesPayload(item, a.seriesCount(item.ID)))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) validateSeriesInput(c *gin.Context) (seriesInput, bool) {
	var in seriesInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return in, false
	}
	in.Title = trim(in.Title)
	in.Description = trim(in.Description)
	if runeLen(in.Title) < 1 || runeLen(in.Title) > 120 {
		bad(c, "合集名称需在 1-120 个字符之间")
		return in, false
	}
	if runeLen(in.Description) > 500 {
		bad(c, "合集介绍最多 500 个字符")
		return in, false
	}
	return in, true
}

func (a *API) SeriesCreate(c *gin.Context) {
	in, ok := a.validateSeriesInput(c)
	if !ok {
		return
	}
	me := currentUser(c)
	series := models.Series{
		Title:       in.Title,
		Description: in.Description,
		AuthorID:    me.ID,
	}
	if err := a.DB.Create(&series).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建合集失败")
		return
	}
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).First(&series, series.ID)
	a.logAction(c, "创建合集", "series", series.ID, series.Title)
	c.JSON(http.StatusCreated, gin.H{"series": a.seriesPayload(series, 0)})
}

func (a *API) SeriesUpdate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var series models.Series
	if err := a.DB.First(&series, id).Error; err != nil {
		fail(c, http.StatusNotFound, "合集不存在")
		return
	}
	me := currentUser(c)
	if series.AuthorID != me.ID && !isSuperAdmin(me) {
		fail(c, http.StatusForbidden, "没有编辑这个合集的权限")
		return
	}
	in, ok := a.validateSeriesInput(c)
	if !ok {
		return
	}
	series.Title = in.Title
	series.Description = in.Description
	if err := a.DB.Save(&series).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存合集失败")
		return
	}
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).First(&series, series.ID)
	a.logAction(c, "更新合集", "series", series.ID, series.Title)
	c.JSON(http.StatusOK, gin.H{"series": a.seriesPayload(series, a.seriesCount(series.ID))})
}

func (a *API) SeriesDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var series models.Series
	if err := a.DB.First(&series, id).Error; err != nil {
		fail(c, http.StatusNotFound, "合集不存在")
		return
	}
	me := currentUser(c)
	if series.AuthorID != me.ID && !isSuperAdmin(me) {
		fail(c, http.StatusForbidden, "没有删除这个合集的权限")
		return
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Document{}).Where("series_id = ?", id).UpdateColumns(map[string]any{
			"series_id":    nil,
			"series_order": 0,
			"updated_at":   time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Delete(&series).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除合集失败")
		return
	}
	a.logAction(c, "删除合集", "series", id, series.Title)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
