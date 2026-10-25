package handlers

import (
	"context"
	"net/http"
	"strings"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *API) AdminStats(c *gin.Context) {
	u := currentUser(c)
	var articles, docs, users, comments, likes, favorites, views int64
	a.DB.Model(&models.Document{}).Where("kind = ? AND status = ?", models.KindArticle, models.StatusPublished).Count(&articles)
	a.DB.Model(&models.Document{}).Where("kind = ? AND status = ?", models.KindDoc, models.StatusPublished).Count(&docs)
	a.DB.Model(&models.User{}).Count(&users)
	a.DB.Model(&models.Comment{}).Count(&comments)
	a.DB.Model(&models.Like{}).Count(&likes)
	a.DB.Model(&models.Favorite{}).Count(&favorites)
	a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished).
		Select("COALESCE(SUM(view_count), 0)").Scan(&views)

	var myArticles, myDocs, myDrafts int64
	a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ?", u.ID, models.KindArticle).Count(&myArticles)
	a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ?", u.ID, models.KindDoc).Count(&myDocs)
	a.DB.Model(&models.Document{}).Where("author_id = ? AND status = ?", u.ID, models.StatusDraft).Count(&myDrafts)

	c.JSON(http.StatusOK, gin.H{
		"role": u.Role,
		"site": gin.H{
			"articles":  articles,
			"docs":      docs,
			"users":     users,
			"comments":  comments,
			"likes":     likes,
			"favorites": favorites,
			"views":     views,
		},
		"mine": gin.H{
			"articles": myArticles,
			"docs":     myDocs,
			"drafts":   myDrafts,
		},
	})
}

// AdminDocs 返回管理后台的内容列表。超管看全站，管理员只看自己的内容。
func (a *API) AdminDocs(c *gin.Context) {
	u := currentUser(c)
	page, size := pageParams(c)
	q := a.DB.Model(&models.Document{})
	if !isSuperAdmin(u) {
		q = q.Where("author_id = ?", u.ID)
	}
	kind := c.Query("kind")
	if kind == models.KindArticle || kind == models.KindDoc {
		q = q.Where("kind = ?", kind)
	}
	status := c.Query("status")
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

func (a *API) AdminUpdateRole(c *gin.Context) {
	me := currentUser(c)
	if !isSuperAdmin(me) {
		fail(c, http.StatusForbidden, "需要超级管理员权限")
		return
	}
	id, ok := paramID(c)
	if !ok {
		return
	}
	var target models.User
	if err := a.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if target.ID == me.ID || target.Role == models.RoleSuperAdmin {
		bad(c, "不能修改超级管理员或自己的角色")
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if in.Role != models.RoleAdmin && in.Role != models.RoleUser {
		bad(c, "角色只能是 admin 或 user")
		return
	}
	if err := a.DB.Model(&target).UpdateColumn("role", in.Role).Error; err != nil {
		fail(c, http.StatusInternalServerError, "修改失败")
		return
	}
	target.Role = in.Role
	a.logAction(c, "设置角色", "user", target.ID, target.Username+" → "+target.Role)
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": toBrief(target)})
}

func (a *API) AdminUsers(c *gin.Context) {
	page, size := pageParams(c)
	q := trim(c.Query("q"))
	db := a.DB.Model(&models.User{})
	if q != "" {
		like := likePattern(q)
		db = db.Where(`username LIKE ? ESCAPE '!' OR nickname LIKE ? ESCAPE '!' OR email LIKE ? ESCAPE '!'`, like, like, like)
	}
	var total int64
	db.Count(&total)
	var users []models.User
	db.Select("id", "username", "nickname", "email", "bio", "role", "created_at", "last_login_at").
		Order("id ASC").Offset((page - 1) * size).Limit(size).Find(&users)
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		var articles, docs, comments int64
		a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ?", u.ID, models.KindArticle).Count(&articles)
		a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ?", u.ID, models.KindDoc).Count(&docs)
		a.DB.Model(&models.Comment{}).Where("user_id = ?", u.ID).Count(&comments)
		items = append(items, gin.H{
			"id":          u.ID,
			"username":    u.Username,
			"nickname":    u.Nickname,
			"email":       u.Email,
			"role":        u.Role,
			"createdAt":   u.CreatedAt,
			"lastLoginAt": u.LastLoginAt,
			"articles":    articles,
			"docs":        docs,
			"comments":    comments,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

func (a *API) AdminDeleteUser(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	me := currentUser(c)
	if id == me.ID {
		bad(c, "不能删除自己")
		return
	}
	var target models.User
	if err := a.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if target.Role == models.RoleSuperAdmin {
		bad(c, "不能删除超级管理员")
		return
	}
	if err := a.purgeUser(target.ID); err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	a.logAction(c, "删除用户", "user", target.ID, target.Username)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// purgeUser 删除用户及其全部痕迹：文档树、评论、互动、关注、私信和通知。
func (a *API) purgeUser(userID uint) error {
	var images []models.Image
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		// 该用户创建的文档（含子文档）
		var ownDocs []uint
		tx.Model(&models.Document{}).Where("author_id = ?", userID).Pluck("id", &ownDocs)
		docSet := map[uint]bool{}
		frontier := append([]uint{}, ownDocs...)
		for i := 0; i < 30 && len(frontier) > 0; i++ {
			var children []uint
			tx.Model(&models.Document{}).Where("parent_id IN ?", frontier).Pluck("id", &children)
			frontier = nil
			for _, cid := range children {
				if !docSet[cid] {
					docSet[cid] = true
					frontier = append(frontier, cid)
				}
			}
		}
		docIDs := make([]uint, 0, len(docSet)+len(ownDocs))
		for _, id := range ownDocs {
			docIDs = append(docIDs, id)
		}
		for id := range docSet {
			docIDs = append(docIDs, id)
		}

		// 该用户创建的合集：解除文章归属后删除合集
		var seriesIDs []uint
		tx.Model(&models.Series{}).Where("author_id = ?", userID).Pluck("id", &seriesIDs)
		if len(seriesIDs) > 0 {
			tx.Model(&models.Document{}).Where("series_id IN ?", seriesIDs).UpdateColumns(map[string]any{
				"series_id":    nil,
				"series_order": 0,
			})
			tx.Where("id IN ?", seriesIDs).Delete(&models.Series{})
		}

		// 该用户发表的评论（删根评论时连同回复）
		var myComments []models.Comment
		tx.Where("user_id = ?", userID).Find(&myComments)
		commentSet := map[uint]bool{}
		affectedDocs := map[uint]bool{}
		for _, cm := range myComments {
			commentSet[cm.ID] = true
			affectedDocs[cm.DocID] = true
			if cm.ParentID == nil {
				var replyIDs []uint
				tx.Model(&models.Comment{}).Where("parent_id = ?", cm.ID).Pluck("id", &replyIDs)
				for _, rid := range replyIDs {
					commentSet[rid] = true
				}
			}
		}
		commentIDs := make([]uint, 0, len(commentSet))
		for id := range commentSet {
			commentIDs = append(commentIDs, id)
		}
		if len(commentIDs) > 0 {
			tx.Where("comment_id IN ?", commentIDs).Delete(&models.CommentLike{})
			tx.Where("id IN ?", commentIDs).Delete(&models.Comment{})
		}

		// 该用户的点赞与收藏会影响其他文档的计数
		var likedDocIDs, favDocIDs []uint
		tx.Model(&models.Like{}).Where("user_id = ?", userID).Pluck("doc_id", &likedDocIDs)
		tx.Model(&models.Favorite{}).Where("user_id = ?", userID).Pluck("doc_id", &favDocIDs)
		for _, id := range likedDocIDs {
			affectedDocs[id] = true
		}
		for _, id := range favDocIDs {
			affectedDocs[id] = true
		}
		tx.Where("user_id = ?", userID).Delete(&models.Like{})
		tx.Where("user_id = ?", userID).Delete(&models.Favorite{})
		var likedCommentIDs []uint
		tx.Model(&models.CommentLike{}).Where("user_id = ?", userID).Pluck("comment_id", &likedCommentIDs)
		tx.Where("user_id = ?", userID).Delete(&models.CommentLike{})

		// 文档树
		if err := deleteDocumentsTx(tx, docIDs); err != nil {
			return err
		}

		// 关注、私信、通知、收藏夹、浏览历史与偏好
		tx.Where("follower_id = ? OR followee_id = ?", userID, userID).Delete(&models.Follow{})
		tx.Where("sender_id = ? OR receiver_id = ?", userID, userID).Delete(&models.Message{})
		tx.Where("user_id = ? OR actor_id = ?", userID, userID).Delete(&models.Notification{})
		tx.Where("requester_id = ? OR owner_id = ?", userID, userID).Delete(&models.DocAccessRequest{})
		if len(docIDs) > 0 {
			tx.Where("doc_id IN ?", docIDs).Delete(&models.DocAccessRequest{})
		}
		tx.Where("user_id = ?", userID).Delete(&models.FavoriteFolder{})
		tx.Where("user_id = ?", userID).Delete(&models.ViewHistory{})
		tx.Where("user_id = ?", userID).Delete(&models.UserSetting{})
		tx.Where("user_id = ?", userID).Delete(&models.AIProviderKey{})
		tx.Where("user_id = ?", userID).Delete(&models.NoticeRead{})
		var email string
		tx.Model(&models.User{}).Where("id = ?", userID).Pluck("email", &email)
		if email != "" {
			tx.Where("email = ?", strings.ToLower(email)).Delete(&models.EmailCode{})
		}

		// 重新计算受影响文档的计数
		for docID := range affectedDocs {
			var exists int64
			tx.Model(&models.Document{}).Where("id = ?", docID).Count(&exists)
			if exists == 0 {
				continue
			}
			var likes, favorites, comments int64
			tx.Model(&models.Like{}).Where("doc_id = ?", docID).Count(&likes)
			tx.Model(&models.Favorite{}).Where("doc_id = ?", docID).Count(&favorites)
			tx.Model(&models.Comment{}).Where("doc_id = ?", docID).Count(&comments)
			tx.Model(&models.Document{}).Where("id = ?", docID).Updates(map[string]any{
				"like_count":     likes,
				"favorite_count": favorites,
				"comment_count":  comments,
			})
		}

		// 被该用户点过赞、但仍存在的评论，需要重算点赞数
		for _, commentID := range likedCommentIDs {
			var exists int64
			tx.Model(&models.Comment{}).Where("id = ?", commentID).Count(&exists)
			if exists == 0 {
				continue
			}
			var likes int64
			tx.Model(&models.CommentLike{}).Where("comment_id = ?", commentID).Count(&likes)
			tx.Model(&models.Comment{}).Where("id = ?", commentID).UpdateColumn("like_count", likes)
		}

		tx.Model(&models.Image{}).Where("user_id = ?", userID).Find(&images)
		tx.Where("user_id = ?", userID).Delete(&models.Image{})

		return tx.Delete(&models.User{}, userID).Error
	})
	if err != nil {
		return err
	}
	for _, img := range images {
		a.removeImageIfUnused(context.Background(), img.Provider, img.Path)
	}
	return nil
}
