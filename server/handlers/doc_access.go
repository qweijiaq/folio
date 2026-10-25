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

func (a *API) docAccessInfo(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).First(&doc, id).Error; err != nil || doc.Status != models.StatusPublished {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if doc.Kind != models.KindDoc || !a.docIsPrivate(doc.ID) {
		c.JSON(http.StatusOK, gin.H{"private": false, "canView": true})
		return
	}
	viewer := currentUser(c)
	if a.canViewDocument(viewer, &doc) {
		c.JSON(http.StatusOK, gin.H{"private": true, "canView": true})
		return
	}
	guardianID := a.docGuardian(doc.ID)
	var guardian models.Document
	if guardianID == 0 || a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).First(&guardian, guardianID).Error != nil {
		c.JSON(http.StatusOK, gin.H{"private": true, "canView": false, "status": "none", "needLogin": viewer == nil})
		return
	}
	status := "none"
	requestID := uint(0)
	if viewer != nil {
		var request models.DocAccessRequest
		if err := a.DB.Where("doc_id = ? AND requester_id = ?", guardian.ID, viewer.ID).First(&request).Error; err == nil {
			status = request.Status
			requestID = request.ID
			if status == models.AccessCancelled {
				status = "none"
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"private":   true,
		"canView":   false,
		"needLogin": viewer == nil,
		"status":    status,
		"requestID": requestID,
		"owner": gin.H{
			"id":       guardian.Author.ID,
			"username": guardian.Author.Username,
			"nickname": guardian.Author.Nickname,
		},
	})
}

func (a *API) docAccessRequest(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil || doc.Status != models.StatusPublished {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	if doc.Kind != models.KindDoc || !a.docIsPrivate(doc.ID) {
		bad(c, "这篇文档不需要申请阅读")
		return
	}
	viewer := currentUser(c)
	if a.canViewDocument(viewer, &doc) {
		bad(c, "你已经可以阅读这篇文档")
		return
	}
	if ok, retry := a.Limiter.Allow(userRateKey("access", viewer.ID), 5, 10*time.Minute); !ok {
		tooMany(c, retry)
		return
	}
	guardianID := a.docGuardian(doc.ID)
	if guardianID == 0 {
		fail(c, http.StatusInternalServerError, "无法确定私密范围")
		return
	}
	var guardian models.Document
	if err := a.DB.First(&guardian, guardianID).Error; err != nil {
		fail(c, http.StatusNotFound, "私密文档不存在")
		return
	}
	var in struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Note = trim(in.Note)
	if runeLen(in.Note) > 200 {
		bad(c, "申请说明最多 200 个字符")
		return
	}
	var request models.DocAccessRequest
	err := a.DB.Where("doc_id = ? AND requester_id = ?", guardian.ID, viewer.ID).First(&request).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusInternalServerError, "读取申请失败")
		return
	}
	if err == nil && request.Status == models.AccessApproved {
		c.JSON(http.StatusOK, gin.H{"status": request.Status, "id": request.ID})
		return
	}
	wasPending := err == nil && request.Status == models.AccessPending
	if request.ID == 0 {
		request = models.DocAccessRequest{
			DocID:       guardian.ID,
			RequesterID: viewer.ID,
			OwnerID:     guardian.AuthorID,
			Status:      models.AccessPending,
			Note:        in.Note,
		}
	} else {
		request.OwnerID = guardian.AuthorID
		request.Status = models.AccessPending
		request.Note = in.Note
		request.HandledAt = nil
	}
	if err := a.DB.Save(&request).Error; err != nil {
		fail(c, http.StatusInternalServerError, "提交申请失败")
		return
	}
	if !wasPending {
		content := "申请阅读你的私密文档"
		if in.Note != "" {
			content = content + "：" + in.Note
		}
		a.notify(guardian.AuthorID, viewer.ID, models.NotifyAccessRequest, &guardian, content)
	}
	c.JSON(http.StatusOK, gin.H{"status": request.Status, "id": request.ID})
}

func (a *API) myAccessRequests(c *gin.Context) {
	me := currentUser(c)
	query := a.DB.Preload("Requester", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname, bio, role, created_at")
	}).Preload("Doc", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, kind, title, private, author_id, created_at, updated_at")
	})
	if !isSuperAdmin(me) {
		query = query.Where("owner_id = ?", me.ID)
	}
	var requests []models.DocAccessRequest
	query.Order("id DESC").Limit(100).Find(&requests)
	items := make([]gin.H, 0, len(requests))
	for _, request := range requests {
		items = append(items, gin.H{
			"id":        request.ID,
			"status":    request.Status,
			"note":      request.Note,
			"createdAt": request.CreatedAt,
			"handledAt": request.HandledAt,
			"requester": toBrief(request.Requester),
			"doc": gin.H{
				"id":    request.Doc.ID,
				"kind":  request.Doc.Kind,
				"title": request.Doc.Title,
			},
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) handleAccessRequest(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var in struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	if in.Action != "approve" && in.Action != "deny" {
		bad(c, "操作只能是 approve 或 deny")
		return
	}
	var request models.DocAccessRequest
	if err := a.DB.First(&request, id).Error; err != nil {
		fail(c, http.StatusNotFound, "申请不存在")
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, request.DocID).Error; err != nil {
		fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	me := currentUser(c)
	if doc.AuthorID != me.ID && !isSuperAdmin(me) {
		fail(c, http.StatusForbidden, "只有文档所有者可以处理申请")
		return
	}
	now := time.Now()
	wasApproved := request.Status == models.AccessApproved
	if in.Action == "approve" {
		request.Status = models.AccessApproved
	} else {
		request.Status = models.AccessDenied
	}
	request.HandledAt = &now
	if err := a.DB.Save(&request).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存申请结果失败")
		return
	}
	if request.Status == models.AccessApproved {
		a.notify(request.RequesterID, me.ID, models.NotifyAccessResult, &doc, "你的私密阅读申请已通过")
	} else if wasApproved {
		a.notify(request.RequesterID, me.ID, models.NotifyAccessResult, nil, "你的私密阅读权限已被作者撤销")
	} else {
		a.notify(request.RequesterID, me.ID, models.NotifyAccessResult, nil, "你的私密阅读申请未通过")
	}
	c.JSON(http.StatusOK, gin.H{"status": request.Status})
}

func (a *API) myOutgoingAccessRequests(c *gin.Context) {
	me := currentUser(c)
	var requests []models.DocAccessRequest
	a.DB.Preload("Doc", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, kind, title, private, author_id")
	}).Preload("Doc.Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username, nickname")
	}).Where("requester_id = ?", me.ID).Order("id DESC").Limit(100).Find(&requests)
	items := make([]gin.H, 0, len(requests))
	for _, request := range requests {
		items = append(items, gin.H{
			"id":        request.ID,
			"docID":     request.DocID,
			"status":    request.Status,
			"note":      request.Note,
			"createdAt": request.CreatedAt,
			"handledAt": request.HandledAt,
			"owner": gin.H{
				"username": request.Doc.Author.Username,
				"nickname": request.Doc.Author.Nickname,
			},
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) cancelAccessRequest(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var request models.DocAccessRequest
	if err := a.DB.First(&request, id).Error; err != nil {
		fail(c, http.StatusNotFound, "申请不存在")
		return
	}
	me := currentUser(c)
	if request.RequesterID != me.ID {
		fail(c, http.StatusForbidden, "只能撤回自己的申请")
		return
	}
	if request.Status != models.AccessPending {
		bad(c, "只能撤回待处理的申请")
		return
	}
	now := time.Now()
	request.Status = models.AccessCancelled
	request.HandledAt = &now
	if err := a.DB.Save(&request).Error; err != nil {
		fail(c, http.StatusInternalServerError, "撤回申请失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": request.Status})
}
