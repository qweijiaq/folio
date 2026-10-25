package handlers

import (
	"net/http"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type commentOut struct {
	ID          uint         `json:"id"`
	CreatedAt   time.Time    `json:"createdAt"`
	Content     string       `json:"content"`
	User        userBrief    `json:"user"`
	LikeCount   int          `json:"likeCount"`
	Liked       bool         `json:"liked"`
	ReplyToName string       `json:"replyToName"`
	Replies     []commentOut `json:"replies"`
}

func (a *API) CommentList(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	if _, ok := a.visibleDoc(c, id); !ok {
		return
	}
	page, size := pageParams(c)
	if size > 30 {
		size = 30
	}
	viewer := currentUser(c)

	var total int64
	a.DB.Model(&models.Comment{}).Where("doc_id = ? AND parent_id IS NULL", id).Count(&total)

	var roots []models.Comment
	a.DB.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Where("doc_id = ? AND parent_id IS NULL", id).
		Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&roots)

	rootIDs := make([]uint, 0, len(roots))
	for _, r := range roots {
		rootIDs = append(rootIDs, r.ID)
	}
	var replies []models.Comment
	if len(rootIDs) > 0 {
		a.DB.Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "username", "nickname", "bio", "role", "created_at")
		}).Where("parent_id IN ?", rootIDs).Order("id ASC").Find(&replies)
	}
	repliesByRoot := map[uint][]models.Comment{}
	allIDs := append([]uint{}, rootIDs...)
	for _, r := range replies {
		repliesByRoot[*r.ParentID] = append(repliesByRoot[*r.ParentID], r)
		allIDs = append(allIDs, r.ID)
	}

	likedSet := map[uint]bool{}
	if viewer != nil && len(allIDs) > 0 {
		var rows []models.CommentLike
		a.DB.Where("user_id = ? AND comment_id IN ?", viewer.ID, allIDs).Find(&rows)
		for _, r := range rows {
			likedSet[r.CommentID] = true
		}
	}

	convert := func(cm models.Comment) commentOut {
		out := commentOut{
			ID:          cm.ID,
			CreatedAt:   cm.CreatedAt,
			Content:     cm.Content,
			User:        toBrief(cm.User),
			LikeCount:   cm.LikeCount,
			Liked:       likedSet[cm.ID],
			ReplyToName: cm.ReplyToName,
			Replies:     []commentOut{},
		}
		return out
	}

	items := make([]commentOut, 0, len(roots))
	for _, r := range roots {
		out := convert(r)
		for _, rep := range repliesByRoot[r.ID] {
			out.Replies = append(out.Replies, convert(rep))
		}
		items = append(items, out)
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

func (a *API) CommentCreate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	doc, ok := a.visibleDoc(c, id)
	if !ok {
		return
	}
	if !doc.OpenComment {
		fail(c, http.StatusForbidden, "这篇内容已关闭评论")
		return
	}
	var in struct {
		Content  string `json:"content"`
		ParentID *uint  `json:"parentID"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Content = trim(in.Content)
	if runeLen(in.Content) < 1 || runeLen(in.Content) > 800 {
		bad(c, "评论长度需在 1-800 个字符之间")
		return
	}
	u := currentUser(c)
	if ok, retry := a.Limiter.Allow(userRateKey("comment", u.ID), 10, time.Minute); !ok {
		tooMany(c, retry)
		return
	}
	cm := models.Comment{DocID: doc.ID, UserID: u.ID, Content: in.Content}
	recipient := doc.AuthorID
	if in.ParentID != nil {
		var parent models.Comment
		if err := a.DB.Preload("User").First(&parent, *in.ParentID).Error; err != nil || parent.DocID != doc.ID {
			bad(c, "回复的评论不存在")
			return
		}
		rootID := parent.ID
		if parent.ParentID != nil {
			rootID = *parent.ParentID
		}
		cm.ParentID = &rootID
		cm.ReplyToName = parent.User.Nickname
		recipient = parent.UserID
	}
	if err := a.DB.Create(&cm).Error; err != nil {
		fail(c, http.StatusInternalServerError, "评论失败")
		return
	}
	a.recalcComments(doc.ID)
	a.notify(recipient, u.ID, models.NotifyComment, doc, ellipsisRunes(in.Content, 80))
	a.DB.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).First(&cm, cm.ID)
	c.JSON(http.StatusCreated, gin.H{"comment": commentOut{
		ID:          cm.ID,
		CreatedAt:   cm.CreatedAt,
		Content:     cm.Content,
		User:        toBrief(cm.User),
		ReplyToName: cm.ReplyToName,
		Replies:     []commentOut{},
	}})
}

func (a *API) CommentDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var cm models.Comment
	if err := a.DB.First(&cm, id).Error; err != nil {
		fail(c, http.StatusNotFound, "评论不存在")
		return
	}
	u := currentUser(c)
	if cm.UserID != u.ID && !isSuperAdmin(u) {
		fail(c, http.StatusForbidden, "没有删除权限")
		return
	}
	ids := []uint{cm.ID}
	if cm.ParentID == nil {
		var replyIDs []uint
		a.DB.Model(&models.Comment{}).Where("parent_id = ?", cm.ID).Pluck("id", &replyIDs)
		ids = append(ids, replyIDs...)
	}
	a.DB.Transaction(func(tx *gorm.DB) error {
		tx.Where("comment_id IN ?", ids).Delete(&models.CommentLike{})
		return tx.Where("id IN ?", ids).Delete(&models.Comment{}).Error
	})
	a.recalcComments(cm.DocID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) CommentLike(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var cm models.Comment
	if err := a.DB.First(&cm, id).Error; err != nil {
		fail(c, http.StatusNotFound, "评论不存在")
		return
	}
	viewer := currentUser(c)
	liked := false
	var err error
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.CommentLike
		e := tx.Where("user_id = ? AND comment_id = ?", viewer.ID, cm.ID).First(&existing).Error
		if e == gorm.ErrRecordNotFound {
			if err := tx.Create(&models.CommentLike{UserID: viewer.ID, CommentID: cm.ID}).Error; err != nil {
				return err
			}
			liked = true
		} else if e != nil {
			return e
		} else if err := tx.Delete(&existing).Error; err != nil {
			return err
		}
		var n int64
		tx.Model(&models.CommentLike{}).Where("comment_id = ?", cm.ID).Count(&n)
		return tx.Model(&models.Comment{}).Where("id = ?", cm.ID).UpdateColumn("like_count", n).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "操作失败")
		return
	}
	var n int64
	a.DB.Model(&models.CommentLike{}).Where("comment_id = ?", cm.ID).Count(&n)
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likeCount": n})
}

func ellipsisRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
