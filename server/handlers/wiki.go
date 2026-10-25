package handlers

import (
	"net/http"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

type docLinkItem struct {
	ID          uint       `json:"id"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Abstract    string     `json:"abstract,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// DocLinks 返回可见的已发布内容标题，供前端把 [[标题]] 解析成站内链接。
func (a *API) DocLinks(c *gin.Context) {
	viewer := currentUser(c)
	q := a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished)
	if hidden := a.hiddenDocIDs(viewer); len(hidden) > 0 {
		q = q.Where("id NOT IN ?", hidden)
	}
	var docs []models.Document
	if err := q.Select("id", "kind", "title").Order("title ASC, id ASC").Limit(5000).Find(&docs).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取链接索引失败")
		return
	}
	items := make([]docLinkItem, 0, len(docs))
	for _, doc := range docs {
		items = append(items, docLinkItem{ID: doc.ID, Kind: doc.Kind, Title: doc.Title})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// DocBacklinks 查找正文里出现 [[当前标题]] 的其他已发布内容。
func (a *API) DocBacklinks(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	viewer := currentUser(c)
	if doc.Status != models.StatusPublished && !canEdit(viewer, &doc) {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if !a.canViewDocument(viewer, &doc) {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		c.JSON(http.StatusOK, gin.H{"items": []docLinkItem{}})
		return
	}
	q := a.DB.Model(&models.Document{}).
		Where("status = ? AND id <> ?", models.StatusPublished, doc.ID).
		Where("content LIKE ? ESCAPE '!'", likePattern("[["+title+"]]"))
	if hidden := a.hiddenDocIDs(viewer); len(hidden) > 0 {
		q = q.Where("id NOT IN ?", hidden)
	}
	var docs []models.Document
	if err := q.Select("id", "kind", "title", "abstract", "published_at", "created_at").
		Order("COALESCE(published_at, created_at) DESC, id DESC").Limit(50).Find(&docs).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取反向链接失败")
		return
	}
	items := make([]docLinkItem, 0, len(docs))
	for _, item := range docs {
		items = append(items, docLinkItem{
			ID: item.ID, Kind: item.Kind, Title: item.Title, Abstract: item.Abstract,
			PublishedAt: item.PublishedAt, CreatedAt: item.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
