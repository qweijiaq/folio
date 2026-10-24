package handlers

import (
	"errors"
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *API) notify(userID, actorID uint, typ string, doc *models.Document, content string) {
	if userID == 0 || userID == actorID {
		return
	}
	n := models.Notification{
		UserID:  userID,
		ActorID: actorID,
		Type:    typ,
		Content: content,
	}
	if doc != nil {
		id := doc.ID
		n.DocID = &id
		n.DocKind = doc.Kind
		n.DocTitle = doc.Title
	}
	a.DB.Create(&n)
}

func (a *API) recalcLikes(docID uint) int64 {
	var n int64
	a.DB.Model(&models.Like{}).Where("doc_id = ?", docID).Count(&n)
	a.DB.Model(&models.Document{}).Where("id = ?", docID).UpdateColumn("like_count", n)
	return n
}

func (a *API) recalcFavorites(docID uint) int64 {
	var n int64
	a.DB.Model(&models.Favorite{}).Where("doc_id = ?", docID).Count(&n)
	a.DB.Model(&models.Document{}).Where("id = ?", docID).UpdateColumn("favorite_count", n)
	return n
}

func (a *API) recalcComments(docID uint) int64 {
	var n int64
	a.DB.Model(&models.Comment{}).Where("doc_id = ?", docID).Count(&n)
	a.DB.Model(&models.Document{}).Where("id = ?", docID).UpdateColumn("comment_count", n)
	return n
}

func (a *API) visibleDoc(c *gin.Context, id uint) (*models.Document, bool) {
	var doc models.Document
	if err := a.DB.First(&doc, id).Error; err != nil {
		fail(c, http.StatusNotFound, "内容不存在")
		return nil, false
	}
	if doc.Status != models.StatusPublished && !canEdit(currentUser(c), &doc) {
		fail(c, http.StatusNotFound, "内容不存在")
		return nil, false
	}
	return &doc, true
}

func (a *API) DocLike(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	doc, ok := a.visibleDoc(c, id)
	if !ok {
		return
	}
	viewer := currentUser(c)
	liked := false
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.Like
		err := tx.Where("user_id = ? AND doc_id = ?", viewer.ID, doc.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&models.Like{UserID: viewer.ID, DocID: doc.ID}).Error; err != nil {
				return err
			}
			liked = true
		case err != nil:
			return err
		default:
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
		}
		var n int64
		tx.Model(&models.Like{}).Where("doc_id = ?", doc.ID).Count(&n)
		return tx.Model(&models.Document{}).Where("id = ?", doc.ID).UpdateColumn("like_count", n).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "操作失败")
		return
	}
	if liked {
		content := ""
		a.notify(doc.AuthorID, viewer.ID, models.NotifyLike, doc, content)
	} else {
		a.DB.Where("user_id = ? AND actor_id = ? AND type = ? AND doc_id = ?", doc.AuthorID, viewer.ID, models.NotifyLike, doc.ID).Delete(&models.Notification{})
	}
	n := a.recalcLikes(doc.ID)
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likeCount": n})
}

func (a *API) DocFavorite(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	doc, ok := a.visibleDoc(c, id)
	if !ok {
		return
	}
	viewer := currentUser(c)
	favorited := false
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.Favorite
		err := tx.Where("user_id = ? AND doc_id = ?", viewer.ID, doc.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&models.Favorite{UserID: viewer.ID, DocID: doc.ID}).Error; err != nil {
				return err
			}
			favorited = true
		case err != nil:
			return err
		default:
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
		}
		var n int64
		tx.Model(&models.Favorite{}).Where("doc_id = ?", doc.ID).Count(&n)
		return tx.Model(&models.Document{}).Where("id = ?", doc.ID).UpdateColumn("favorite_count", n).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "操作失败")
		return
	}
	n := a.recalcFavorites(doc.ID)
	c.JSON(http.StatusOK, gin.H{"favorited": favorited, "favoriteCount": n})
}
