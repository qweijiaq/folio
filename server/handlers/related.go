package handlers

import (
	"net/http"
	"sort"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RelatedDocs 根据共同标签、同一分类或文档的父子/兄弟关系推荐内容。
func (a *API) RelatedDocs(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var current models.Document
	if err := a.DB.First(&current, id).Error; err != nil || current.Status != models.StatusPublished {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	viewer := currentUser(c)
	if !a.canViewDocument(viewer, &current) {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	var candidates []models.Document
	query := a.DB.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Where("kind = ? AND status = ? AND id <> ?", current.Kind, models.StatusPublished, current.ID)
	if current.Kind == models.KindDoc {
		if hidden := a.hiddenDocIDs(viewer); len(hidden) > 0 {
			query = query.Where("id NOT IN ?", hidden)
		}
	}
	query.Order("view_count DESC").Limit(80).Find(&candidates)

	currentTags := map[string]bool{}
	for _, tag := range current.Tags {
		currentTags[tag] = true
	}

	type scored struct {
		doc   models.Document
		score int
	}
	ranked := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		score := 0
		for _, tag := range candidate.Tags {
			if currentTags[tag] {
				score += 2
			}
		}
		if current.CategoryID != nil && candidate.CategoryID != nil && *current.CategoryID == *candidate.CategoryID {
			score++
		}
		if current.ParentID != nil && candidate.ParentID != nil && *current.ParentID == *candidate.ParentID {
			score++
		}
		if current.ParentID != nil && *current.ParentID == candidate.ID {
			score += 2
		}
		if candidate.ParentID != nil && *candidate.ParentID == current.ID {
			score += 2
		}
		if score > 0 {
			candidate.Content = ""
			ranked = append(ranked, scored{doc: candidate, score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].doc.ViewCount > ranked[j].doc.ViewCount
		}
		return ranked[i].score > ranked[j].score
	})
	items := make([]models.Document, 0, 4)
	for i := 0; i < len(ranked) && i < 4; i++ {
		items = append(items, ranked[i].doc)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
