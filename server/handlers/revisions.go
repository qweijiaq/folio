package handlers

import (
	"net/http"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *API) saveRevision(doc *models.Document, editor *models.User) error {
	editorName := ""
	editorID := uint(0)
	if editor != nil {
		editorName = editor.Nickname
		editorID = editor.ID
	}
	return a.DB.Create(&models.DocumentRevision{
		DocID:       doc.ID,
		EditorID:    editorID,
		EditorName:  editorName,
		Title:       doc.Title,
		Abstract:    doc.Abstract,
		Content:     doc.Content,
		Tags:        append([]string{}, doc.Tags...),
		Status:      doc.Status,
		Private:     doc.Private,
		ParentID:    doc.ParentID,
		CategoryID:  doc.CategoryID,
		SeriesID:    doc.SeriesID,
		SeriesOrder: doc.SeriesOrder,
		Pinned:      doc.Pinned,
		OpenComment: doc.OpenComment,
		Sort:        doc.Sort,
	}).Error
}

func sameUintPtr(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a *API) documentChanged(doc *models.Document, in docInput, abstract string, categoryID *uint) bool {
	if doc.Title != in.Title || doc.Abstract != abstract || doc.Content != in.Content ||
		doc.Status != in.Status || !sameUintPtr(doc.ParentID, in.ParentID) ||
		!sameUintPtr(doc.CategoryID, categoryID) || doc.Sort != in.Sort ||
		!sameTags(doc.Tags, normalizeTags(in.Tags)) {
		return true
	}
	if in.Pinned != nil && doc.Pinned != *in.Pinned {
		return true
	}
	if in.OpenComment != nil && doc.OpenComment != *in.OpenComment {
		return true
	}
	if doc.Kind == models.KindDoc && in.Private != nil && doc.Private != *in.Private {
		return true
	}
	if doc.Kind == models.KindArticle && (!sameUintPtr(doc.SeriesID, in.SeriesID) || doc.SeriesOrder != maxInt(0, in.SeriesOrder)) {
		return true
	}
	return false
}

func (a *API) RevisionList(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if !canEdit(currentUser(c), &doc) {
		fail(c, http.StatusForbidden, "没有查看历史的权限")
		return
	}
	var revisions []models.DocumentRevision
	a.DB.Where("doc_id = ?", doc.ID).Order("id DESC").Limit(50).Find(&revisions)
	items := make([]gin.H, 0, len(revisions))
	for _, revision := range revisions {
		items = append(items, gin.H{
			"id":         revision.ID,
			"title":      revision.Title,
			"status":     revision.Status,
			"editorID":   revision.EditorID,
			"editorName": revision.EditorName,
			"createdAt":  revision.CreatedAt,
			"excerpt":    clipText(plainText(revision.Content), 120),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) RevisionDetail(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	revisionID, ok := parseParam(c, "revisionID")
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if !canEdit(currentUser(c), &doc) {
		fail(c, http.StatusForbidden, "没有查看历史的权限")
		return
	}
	var revision models.DocumentRevision
	if err := a.DB.Where("id = ? AND doc_id = ?", revisionID, doc.ID).First(&revision).Error; err != nil {
		fail(c, http.StatusNotFound, "历史版本不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"revision": revision})
}

func (a *API) RevisionRestore(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	revisionID, ok := parseParam(c, "revisionID")
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	editor := currentUser(c)
	if !canEdit(editor, &doc) {
		fail(c, http.StatusForbidden, "没有恢复历史的权限")
		return
	}
	var revision models.DocumentRevision
	if err := a.DB.Where("id = ? AND doc_id = ?", revisionID, doc.ID).First(&revision).Error; err != nil {
		fail(c, http.StatusNotFound, "历史版本不存在")
		return
	}
	if err := a.saveRevision(&doc, editor); err != nil {
		fail(c, http.StatusInternalServerError, "保存当前版本失败")
		return
	}
	doc.Title = revision.Title
	doc.Abstract = revision.Abstract
	doc.Content = revision.Content
	doc.Tags = append([]string{}, revision.Tags...)
	doc.Status = revision.Status
	doc.Private = revision.Private
	doc.ParentID = revision.ParentID
	doc.CategoryID = revision.CategoryID
	doc.SeriesID = revision.SeriesID
	doc.SeriesOrder = revision.SeriesOrder
	doc.Pinned = revision.Pinned
	doc.OpenComment = revision.OpenComment
	doc.Sort = revision.Sort
	fields := []string{"title", "abstract", "content", "tags", "status", "private", "parent_id", "category_id", "series_id", "series_order", "pinned", "open_comment", "sort"}
	if doc.Status == models.StatusPublished && doc.PublishedAt == nil {
		now := time.Now()
		doc.PublishedAt = &now
		fields = append(fields, "published_at")
	}
	if err := a.DB.Model(&doc).Select(fields).Updates(&doc).Error; err != nil {
		fail(c, http.StatusInternalServerError, "恢复历史版本失败")
		return
	}
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Preload("Category").First(&doc, doc.ID)
	a.logAction(c, "恢复版本", doc.Kind, doc.ID, doc.Title)
	c.JSON(http.StatusOK, gin.H{"doc": doc})
}
