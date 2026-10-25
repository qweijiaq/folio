package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

type docNeighbor struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// DocNeighbors 返回文章按发布时间排序的上一篇/下一篇。
func (a *API) DocNeighbors(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var current models.Document
	if err := a.DB.First(&current, id).Error; err != nil || current.Status != models.StatusPublished {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if current.Kind != models.KindArticle {
		c.JSON(http.StatusOK, gin.H{"prev": nil, "next": nil})
		return
	}
	viewer := currentUser(c)
	if !a.canViewDocument(viewer, &current) {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	var docs []models.Document
	a.DB.Select("id", "title", "kind", "published_at", "created_at").
		Where("kind = ? AND status = ?", models.KindArticle, models.StatusPublished).
		Order("COALESCE(published_at, created_at) ASC, id ASC").Find(&docs)
	index := -1
	for i, doc := range docs {
		if doc.ID == current.ID {
			index = i
			break
		}
	}
	var prev, next *docNeighbor
	if index > 0 {
		prev = &docNeighbor{ID: docs[index-1].ID, Title: docs[index-1].Title, Kind: docs[index-1].Kind}
	}
	if index >= 0 && index < len(docs)-1 {
		next = &docNeighbor{ID: docs[index+1].ID, Title: docs[index+1].Title, Kind: docs[index+1].Kind}
	}
	c.JSON(http.StatusOK, gin.H{"prev": prev, "next": next})
}
