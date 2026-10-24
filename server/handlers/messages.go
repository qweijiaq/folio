package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type conversationOut struct {
	User        userBrief `json:"user"`
	LastMessage string    `json:"lastMessage"`
	LastAt      time.Time `json:"lastAt"`
	Unread      int       `json:"unread"`
}

func (a *API) Conversations(c *gin.Context) {
	me := currentUser(c)
	var msgs []models.Message
	a.DB.Where("sender_id = ? OR receiver_id = ?", me.ID, me.ID).Order("id DESC").Limit(1000).Find(&msgs)

	type agg struct {
		last   models.Message
		unread int
	}
	byPeer := map[uint]*agg{}
	order := []uint{}
	for _, m := range msgs {
		peer := m.ReceiverID
		if m.ReceiverID == me.ID {
			peer = m.SenderID
		}
		item, ok := byPeer[peer]
		if !ok {
			item = &agg{last: m}
			byPeer[peer] = item
			order = append(order, peer)
		}
		if m.ReceiverID == me.ID && m.ReadAt == nil {
			item.unread++
		}
	}
	items := make([]conversationOut, 0, len(order))
	for _, peerID := range order {
		var u models.User
		if err := a.DB.Select("id", "username", "nickname", "bio", "role", "created_at").First(&u, peerID).Error; err != nil {
			continue
		}
		item := byPeer[peerID]
		items = append(items, conversationOut{
			User:        toBrief(u),
			LastMessage: item.last.Content,
			LastAt:      item.last.CreatedAt,
			Unread:      item.unread,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].LastAt.After(items[j].LastAt) })
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) MessageThread(c *gin.Context) {
	me := currentUser(c)
	peer, err := a.userByName(c.Param("username"))
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if peer.ID == me.ID {
		bad(c, "不能和自己聊天")
		return
	}
	q := a.DB.Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		me.ID, peer.ID, peer.ID, me.ID)
	if before, _ := strconv.ParseUint(c.Query("before"), 10, 64); before > 0 {
		q = q.Where("id < ?", before)
	}
	var msgs []models.Message
	q.Order("id DESC").Limit(50).Find(&msgs)
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	now := time.Now()
	a.DB.Model(&models.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL", peer.ID, me.ID).
		UpdateColumn("read_at", now)

	c.JSON(http.StatusOK, gin.H{"peer": toBrief(*peer), "items": msgs})
}

func (a *API) MessageSend(c *gin.Context) {
	me := currentUser(c)
	peer, err := a.userByName(c.Param("username"))
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if peer.ID == me.ID {
		bad(c, "不能给自己发私信")
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Content = trim(in.Content)
	if runeLen(in.Content) < 1 || runeLen(in.Content) > 1000 {
		bad(c, "消息长度需在 1-1000 个字符之间")
		return
	}
	m := models.Message{SenderID: me.ID, ReceiverID: peer.ID, Content: in.Content}
	if err := a.DB.Create(&m).Error; err != nil {
		fail(c, http.StatusInternalServerError, "发送失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": m})
}

func (a *API) Inbox(c *gin.Context) {
	me := currentUser(c)
	var messages, notifications int64
	a.DB.Model(&models.Message{}).Where("receiver_id = ? AND read_at IS NULL", me.ID).Count(&messages)
	a.DB.Model(&models.Notification{}).Where("user_id = ? AND read = ?", me.ID, false).Count(&notifications)
	c.JSON(http.StatusOK, gin.H{"messages": messages, "notifications": notifications})
}

func (a *API) Notifications(c *gin.Context) {
	me := currentUser(c)
	var items []models.Notification
	a.DB.Preload("Actor", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Where("user_id = ?", me.ID).Order("id DESC").Limit(60).Find(&items)
	var unread int64
	a.DB.Model(&models.Notification{}).Where("user_id = ? AND read = ?", me.ID, false).Count(&unread)
	c.JSON(http.StatusOK, gin.H{"items": items, "unread": unread})
}

func (a *API) NotificationsRead(c *gin.Context) {
	me := currentUser(c)
	a.DB.Model(&models.Notification{}).Where("user_id = ? AND read = ?", me.ID, false).UpdateColumn("read", true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
