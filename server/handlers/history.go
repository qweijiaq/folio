package handlers

import (
	"net/http"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (a *API) recordHistory(userID, docID uint) {
	if userID == 0 || docID == 0 {
		return
	}
	now := time.Now()
	a.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "doc_id"}},
		DoUpdates: clause.Assignments(map[string]any{"updated_at": now}),
	}).Create(&models.ViewHistory{
		Base:   models.Base{CreatedAt: now, UpdatedAt: now},
		UserID: userID,
		DocID:  docID,
	})
}

func (a *API) MyHistory(c *gin.Context) {
	me := currentUser(c)
	page, size := pageParams(c)
	var total int64
	a.DB.Model(&models.ViewHistory{}).Where("user_id = ?", me.ID).Count(&total)
	var rows []models.ViewHistory
	a.DB.Where("user_id = ?", me.ID).Order("updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&rows)

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.DocID)
	}
	docsByID := map[uint]models.Document{}
	if len(ids) > 0 {
		var docs []models.Document
		a.DB.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "username", "nickname", "bio", "role", "created_at")
		}).Where("id IN ? AND status = ?", ids, models.StatusPublished).Find(&docs)
		for _, doc := range docs {
			doc.Content = ""
			docsByID[doc.ID] = doc
		}
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		doc, ok := docsByID[row.DocID]
		if !ok {
			continue
		}
		items = append(items, gin.H{"doc": doc, "viewedAt": row.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

func (a *API) ClearHistory(c *gin.Context) {
	me := currentUser(c)
	if err := a.DB.Where("user_id = ?", me.ID).Delete(&models.ViewHistory{}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "清空失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
