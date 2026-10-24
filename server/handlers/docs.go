package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type docInput struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Abstract    string   `json:"abstract"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	ParentID    *uint    `json:"parentID"`
	CategoryID  *uint    `json:"categoryID"`
	Pinned      *bool    `json:"pinned"`
	OpenComment *bool    `json:"openComment"`
	Sort        int      `json:"sort"`
}

var (
	reFence  = regexp.MustCompile("(?s)```.*?```")
	reImage  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHTML   = regexp.MustCompile(`<[^>]+>`)
	reMarks  = regexp.MustCompile("[#>*_`~]+")
	reSpaces = regexp.MustCompile(`\s+`)
)

func plainText(md string) string {
	s := reFence.ReplaceAllString(md, " ")
	s = reImage.ReplaceAllString(s, " ")
	s = reLink.ReplaceAllString(s, "$1")
	s = reHTML.ReplaceAllString(s, " ")
	s = reMarks.ReplaceAllString(s, " ")
	s = reSpaces.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 120 {
		s = string([]rune(s)[:120]) + "…"
	}
	return s
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, 6)
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > 16 {
			t = string([]rune(t)[:16])
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func tagPattern(s string) string {
	s = strings.ReplaceAll(s, "!", "!!")
	s = strings.ReplaceAll(s, "%", "!%")
	s = strings.ReplaceAll(s, "_", "!_")
	return `%"` + s + `"%`
}

func (a *API) checkParent(kind string, parentID *uint, selfID uint) error {
	if kind != models.KindDoc {
		if parentID != nil {
			return errors.New("文章不能设置上级文档")
		}
		return nil
	}
	if parentID == nil {
		return nil
	}
	var parent models.Document
	if err := a.DB.First(&parent, *parentID).Error; err != nil {
		return errors.New("上级文档不存在")
	}
	if parent.Kind != models.KindDoc {
		return errors.New("上级必须是文档")
	}
	if selfID != 0 {
		cur := parent
		for i := 0; i < 20; i++ {
			if cur.ID == selfID {
				return errors.New("不能把文档移动到自身或子文档下")
			}
			if cur.ParentID == nil {
				break
			}
			var next models.Document
			if err := a.DB.Select("id", "parent_id").First(&next, *cur.ParentID).Error; err != nil {
				break
			}
			cur = next
		}
	}
	return nil
}

func (a *API) userByName(username string) (*models.User, error) {
	var u models.User
	if err := a.DB.Where("lower(username) = lower(?)", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *API) resolveCategory(kind string, categoryID *uint) (*uint, error) {
	if kind != models.KindArticle || categoryID == nil {
		return nil, nil
	}
	var category models.Category
	if err := a.DB.First(&category, *categoryID).Error; err != nil {
		return nil, errors.New("分类不存在")
	}
	return categoryID, nil
}

func blankContent(docs []models.Document) {
	for i := range docs {
		docs[i].Content = ""
	}
}

func (a *API) DocList(c *gin.Context) {
	page, size := pageParams(c)
	viewer := currentUser(c)
	kind := c.Query("kind")
	if kind != "" && kind != models.KindArticle && kind != models.KindDoc {
		bad(c, "未知的内容类型")
		return
	}
	author := trim(c.Query("author"))
	tag := trim(c.Query("tag"))
	keyword := trim(c.Query("q"))
	categoryID := trim(c.Query("categoryID"))
	mine := c.Query("mine") == "1"
	status := c.Query("status")

	if mine && viewer == nil {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	if author == "" && mine {
		author = viewer.Username
	}

	includeUnpublished := false
	if viewer != nil {
		if mine {
			includeUnpublished = true
		} else if author != "" && strings.EqualFold(author, viewer.Username) {
			includeUnpublished = true
		} else if isSuperAdmin(viewer) && status == "all" {
			includeUnpublished = true
		}
	}

	q := a.DB.Model(&models.Document{})
	if author != "" {
		au, err := a.userByName(author)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"items": []models.Document{}, "total": 0, "page": page, "pageSize": size})
			return
		}
		q = q.Where("author_id = ?", au.ID)
	}
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if includeUnpublished {
		if status == models.StatusDraft || status == models.StatusPublished {
			q = q.Where("status = ?", status)
		}
	} else {
		q = q.Where("status = ?", models.StatusPublished)
	}
	if tag != "" {
		q = q.Where(`tags LIKE ? ESCAPE '!'`, tagPattern(tag))
	}
	if keyword != "" {
		like := likePattern(keyword)
		q = q.Where(`title LIKE ? ESCAPE '!' OR abstract LIKE ? ESCAPE '!' OR content LIKE ? ESCAPE '!' OR tags LIKE ? ESCAPE '!'`, like, like, like, like)
	}
	if categoryID != "" {
		if id, err := strconv.ParseUint(categoryID, 10, 64); err == nil && id > 0 {
			q = q.Where("category_id = ?", uint(id))
		}
	}

	var total int64
	q.Count(&total)

	switch c.Query("sort") {
	case "popular":
		q = q.Order("view_count DESC, id DESC")
	case "likes":
		q = q.Order("like_count DESC, id DESC")
	case "oldest":
		q = q.Order("created_at ASC")
	default:
		q = q.Order("pinned DESC, COALESCE(published_at, created_at) DESC, id DESC")
	}

	var docs []models.Document
	if err := q.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Offset((page - 1) * size).Limit(size).Find(&docs).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取失败")
		return
	}
	blankContent(docs)
	c.JSON(http.StatusOK, gin.H{"items": docs, "total": total, "page": page, "pageSize": size})
}

func (a *API) DocTree(c *gin.Context) {
	var docs []models.Document
	a.DB.Select("id", "title", "parent_id", "sort", "created_at", "updated_at", "view_count", "like_count").
		Where("kind = ? AND status = ?", models.KindDoc, models.StatusPublished).
		Order("sort ASC, id ASC").Find(&docs)
	items := make([]gin.H, 0, len(docs))
	for _, d := range docs {
		items = append(items, gin.H{
			"id":        d.ID,
			"title":     d.Title,
			"parentID":  d.ParentID,
			"sort":      d.Sort,
			"viewCount": d.ViewCount,
			"likeCount": d.LikeCount,
			"createdAt": d.CreatedAt,
			"updatedAt": d.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) DocDetail(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	viewer := currentUser(c)
	if doc.Status != models.StatusPublished && !canEdit(viewer, &doc) {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}

	liked, favorited, following := false, false, false
	if viewer != nil {
		var n int64
		a.DB.Model(&models.Like{}).Where("user_id = ? AND doc_id = ?", viewer.ID, doc.ID).Count(&n)
		liked = n > 0
		a.DB.Model(&models.Favorite{}).Where("user_id = ? AND doc_id = ?", viewer.ID, doc.ID).Count(&n)
		favorited = n > 0
		if viewer.ID != doc.AuthorID {
			a.DB.Model(&models.Follow{}).Where("follower_id = ? AND followee_id = ?", viewer.ID, doc.AuthorID).Count(&n)
			following = n > 0
		}
	}

	if doc.Status == models.StatusPublished && (viewer == nil || viewer.ID != doc.AuthorID) {
		viewerKey := "anon:" + c.ClientIP()
		if viewer != nil {
			viewerKey = fmt.Sprintf("user:%d", viewer.ID)
		}
		if a.Cache.MarkView(c.Request.Context(), doc.ID, viewerKey, 30*time.Minute) {
			a.DB.Model(&models.Document{}).Where("id = ?", doc.ID).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
			doc.ViewCount++
			now := time.Now()
			stat := models.DocDailyStat{DocID: doc.ID, Day: now.Format("2006-01-02"), Views: 1}
			if err := a.DB.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "doc_id"}, {Name: "day"}},
				DoUpdates: clause.Assignments(map[string]any{"views": gorm.Expr("views + 1"), "updated_at": now}),
			}).Create(&stat).Error; err != nil {
				log.Printf("写入每日阅读统计失败: %v", err)
			}
		}
	}
	if viewer != nil && viewer.ID != doc.AuthorID && doc.Status == models.StatusPublished {
		a.recordHistory(viewer.ID, doc.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"doc":             doc,
		"liked":           liked,
		"favorited":       favorited,
		"followingAuthor": following,
	})
}

func (a *API) DocEdit(c *gin.Context) {
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
		fail(c, http.StatusForbidden, "没有编辑权限")
		return
	}
	c.JSON(http.StatusOK, gin.H{"doc": doc})
}

func (a *API) DocCreate(c *gin.Context) {
	var in docInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	u := currentUser(c)
	if in.Kind == "" {
		in.Kind = models.KindArticle
	}
	if in.Kind != models.KindArticle && in.Kind != models.KindDoc {
		bad(c, "未知的内容类型")
		return
	}
	if in.Status != models.StatusDraft && in.Status != models.StatusPublished {
		in.Status = models.StatusDraft
	}
	in.Title = trim(in.Title)
	if runeLen(in.Title) < 1 || runeLen(in.Title) > 120 {
		bad(c, "标题长度需在 1-120 个字符之间")
		return
	}
	if in.Status == models.StatusPublished && trim(in.Content) == "" {
		bad(c, "发布前请先填写正文")
		return
	}
	if err := a.checkParent(in.Kind, in.ParentID, 0); err != nil {
		bad(c, err.Error())
		return
	}
	categoryID, err := a.resolveCategory(in.Kind, in.CategoryID)
	if err != nil {
		bad(c, err.Error())
		return
	}
	abstract := trim(in.Abstract)
	if abstract == "" {
		abstract = plainText(in.Content)
	}
	openComment := true
	if in.OpenComment != nil {
		openComment = *in.OpenComment
	}
	doc := models.Document{
		Kind:        in.Kind,
		Title:       in.Title,
		Abstract:    abstract,
		Content:     in.Content,
		AuthorID:    u.ID,
		ParentID:    in.ParentID,
		CategoryID:  categoryID,
		Tags:        normalizeTags(in.Tags),
		Status:      in.Status,
		OpenComment: openComment,
		Sort:        in.Sort,
	}
	if in.Pinned != nil && *in.Pinned {
		doc.Pinned = true
	}
	if in.Status == models.StatusPublished {
		now := time.Now()
		doc.PublishedAt = &now
	}
	if err := a.DB.Create(&doc).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	if !openComment {
		// 带默认值的布尔字段用零值创建时会被数据库默认值覆盖，这里显式落一次关闭状态。
		a.DB.Model(&doc).UpdateColumn("open_comment", false)
		doc.OpenComment = false
	}
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).First(&doc, doc.ID)
	a.logAction(c, "创建内容", doc.Kind, doc.ID, doc.Title+"（"+doc.Status+"）")
	c.JSON(http.StatusCreated, gin.H{"doc": doc})
}

func (a *API) DocUpdate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return
	}
	u := currentUser(c)
	if !canEdit(u, &doc) {
		fail(c, http.StatusForbidden, "没有编辑权限")
		return
	}
	var in docInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if in.Kind != "" && in.Kind != doc.Kind {
		bad(c, "不能修改内容类型")
		return
	}
	in.Title = trim(in.Title)
	if runeLen(in.Title) < 1 || runeLen(in.Title) > 120 {
		bad(c, "标题长度需在 1-120 个字符之间")
		return
	}
	if in.Status != models.StatusDraft && in.Status != models.StatusPublished {
		in.Status = doc.Status
	}
	if in.Status == models.StatusPublished && trim(in.Content) == "" {
		bad(c, "发布前请先填写正文")
		return
	}
	if err := a.checkParent(doc.Kind, in.ParentID, doc.ID); err != nil {
		bad(c, err.Error())
		return
	}
	categoryID, err := a.resolveCategory(doc.Kind, in.CategoryID)
	if err != nil {
		bad(c, err.Error())
		return
	}
	abstract := trim(in.Abstract)
	if abstract == "" {
		abstract = plainText(in.Content)
	}
	if a.documentChanged(&doc, in, abstract, categoryID) {
		if err := a.saveRevision(&doc, u); err != nil {
			fail(c, http.StatusInternalServerError, "保存历史版本失败")
			return
		}
	}
	doc.Title = in.Title
	doc.Abstract = abstract
	doc.Content = in.Content
	doc.Tags = normalizeTags(in.Tags)
	doc.Status = in.Status
	doc.ParentID = in.ParentID
	doc.CategoryID = categoryID
	doc.Sort = in.Sort
	fields := []string{"title", "abstract", "content", "tags", "status", "parent_id", "category_id", "sort"}
	if in.OpenComment != nil {
		doc.OpenComment = *in.OpenComment
		fields = append(fields, "open_comment")
	}
	if in.Pinned != nil {
		doc.Pinned = *in.Pinned
		fields = append(fields, "pinned")
	}
	if in.Status == models.StatusPublished && doc.PublishedAt == nil {
		now := time.Now()
		doc.PublishedAt = &now
		fields = append(fields, "published_at")
	}
	if err := a.DB.Model(&doc).Select(fields).Updates(doc).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).First(&doc, doc.ID)
	a.logAction(c, "更新内容", doc.Kind, doc.ID, doc.Title+"（"+doc.Status+"）")
	c.JSON(http.StatusOK, gin.H{"doc": doc})
}

func (a *API) docAndDescendants(id uint) ([]uint, error) {
	ids := []uint{id}
	frontier := []uint{id}
	for depth := 0; depth < 30 && len(frontier) > 0; depth++ {
		var children []uint
		if err := a.DB.Model(&models.Document{}).Where("parent_id IN ?", frontier).Pluck("id", &children).Error; err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		ids = append(ids, children...)
		frontier = children
	}
	return ids, nil
}

func deleteDocumentsTx(tx *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var commentIDs []uint
	tx.Model(&models.Comment{}).Where("doc_id IN ?", ids).Pluck("id", &commentIDs)
	if len(commentIDs) > 0 {
		tx.Where("comment_id IN ?", commentIDs).Delete(&models.CommentLike{})
	}
	tx.Where("doc_id IN ?", ids).Delete(&models.Comment{})
	tx.Where("doc_id IN ?", ids).Delete(&models.Like{})
	tx.Where("doc_id IN ?", ids).Delete(&models.Favorite{})
	tx.Where("doc_id IN ?", ids).Delete(&models.Notification{})
	tx.Where("doc_id IN ?", ids).Delete(&models.DocDailyStat{})
	tx.Where("doc_id IN ?", ids).Delete(&models.DocumentRevision{})
	return tx.Where("id IN ?", ids).Delete(&models.Document{}).Error
}

func (a *API) deleteDocuments(ids []uint) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		return deleteDocumentsTx(tx, ids)
	})
}

func (a *API) DocDelete(c *gin.Context) {
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
		fail(c, http.StatusForbidden, "没有删除权限")
		return
	}
	ids, err := a.docAndDescendants(doc.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := a.deleteDocuments(ids); err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	a.DB.Model(&models.UserSetting{}).Where("featured_doc_id = ?", doc.ID).UpdateColumn("featured_doc_id", nil)
	a.logAction(c, "删除内容", doc.Kind, doc.ID, doc.Title)
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": len(ids)})
}

func (a *API) MyDocs(c *gin.Context) {
	u := currentUser(c)
	page, size := pageParams(c)
	kind := c.Query("kind")
	status := c.Query("status")
	q := a.DB.Model(&models.Document{}).Where("author_id = ?", u.ID)
	if kind == models.KindArticle || kind == models.KindDoc {
		q = q.Where("kind = ?", kind)
	}
	if status == models.StatusDraft || status == models.StatusPublished {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var docs []models.Document
	q.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Order("updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&docs)
	blankContent(docs)
	c.JSON(http.StatusOK, gin.H{"items": docs, "total": total, "page": page, "pageSize": size})
}

func (a *API) MyFavorites(c *gin.Context) {
	u := currentUser(c)
	page, size := pageParams(c)
	folderID := trim(c.Query("folderID"))
	q := a.DB.Model(&models.Favorite{}).Where("user_id = ?", u.ID)
	if folderID != "" {
		if id, err := strconv.ParseUint(folderID, 10, 64); err == nil && id > 0 {
			q = q.Where("folder_id = ?", uint(id))
		}
	}
	var total int64
	q.Count(&total)
	var favs []models.Favorite
	q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&favs)
	ids := make([]uint, 0, len(favs))
	for _, f := range favs {
		ids = append(ids, f.DocID)
	}
	items := make([]models.Document, 0, len(ids))
	if len(ids) > 0 {
		var docs []models.Document
		a.DB.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "username", "nickname", "bio", "role", "created_at")
		}).Where("id IN ?", ids).Find(&docs)
		byID := map[uint]models.Document{}
		for _, d := range docs {
			byID[d.ID] = d
		}
		for _, id := range ids {
			if d, ok := byID[id]; ok {
				d.Content = ""
				items = append(items, d)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}
