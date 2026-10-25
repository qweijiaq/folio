package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

func emailSubjectText(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}

func notificationKindLabel(doc *models.Document) string {
	if doc != nil && doc.Kind == models.KindDoc {
		return "文档"
	}
	return "文章"
}

func notificationActorName(actor models.User) string {
	if strings.TrimSpace(actor.Nickname) != "" {
		return actor.Nickname
	}
	if strings.TrimSpace(actor.Username) != "" {
		return actor.Username
	}
	return "有人"
}

// queueNotificationEmail 在站内通知创建后异步发送邮件，失败不会影响互动本身。
func (a *API) queueNotificationEmail(c *gin.Context, userID, actorID uint, typ string, doc *models.Document, content string) {
	if !a.Mail.Enabled() || typ == models.NotifyLike {
		return
	}
	var recipient models.User
	if err := a.DB.Select("id", "email").First(&recipient, userID).Error; err != nil || strings.TrimSpace(recipient.Email) == "" {
		return
	}
	var actor models.User
	_ = a.DB.Select("id", "username", "nickname").First(&actor, actorID).Error
	actorName := notificationActorName(actor)
	kind := notificationKindLabel(doc)
	docTitle := ""
	if doc != nil {
		docTitle = strings.TrimSpace(doc.Title)
	}

	subject := ""
	body := ""
	path := ""
	switch typ {
	case models.NotifyComment:
		subject = fmt.Sprintf("%s 评论了你的%s", actorName, kind)
		if docTitle != "" {
			subject += "《" + docTitle + "》"
			body = fmt.Sprintf("%s 评论了你的%s《%s》：\n\n%s", actorName, kind, docTitle, content)
		} else {
			body = fmt.Sprintf("%s 评论了你的%s：\n\n%s", actorName, kind, content)
		}
		if doc != nil {
			path = fmt.Sprintf("/%s/%d", map[string]string{"doc": "docs", "article": "articles"}[doc.Kind], doc.ID)
		}
	case models.NotifyFollow:
		subject = fmt.Sprintf("%s 关注了你", actorName)
		body = fmt.Sprintf("%s 关注了你。", actorName)
		if actor.Username != "" {
			path = "/u/" + actor.Username
		}
	case models.NotifyAccessRequest:
		subject = fmt.Sprintf("%s 申请阅读你的私密%s", actorName, kind)
		if docTitle != "" {
			subject += "《" + docTitle + "》"
			body = fmt.Sprintf("%s 申请阅读你的私密%s《%s》。\n\n申请说明：%s", actorName, kind, docTitle, content)
		} else {
			body = fmt.Sprintf("%s 申请阅读你的私密%s。\n\n申请说明：%s", actorName, kind, content)
		}
		path = "/admin?tab=requests"
	case models.NotifyAccessResult:
		subject = "你的私密阅读申请有新的处理结果"
		body = content
		if docTitle != "" {
			body += fmt.Sprintf("\n\n文档：《%s》", docTitle)
		}
		if doc != nil {
			path = fmt.Sprintf("/docs/%d", doc.ID)
		} else {
			path = "/messages?tab=notifications"
		}
	default:
		return
	}

	if path != "" {
		if base := strings.TrimRight(a.publicBaseURL(c), "/"); base != "" {
			body += "\n\n查看：" + base + path
		}
	}
	body += "\n\n不想收到这类邮件？可在「我的空间 → 隐私与消息」关闭邮件通知。"
	subject = "[知微] " + emailSubjectText(subject)
	to := recipient.Email

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := a.Mail.Send(ctx, to, subject, body); err != nil {
			log.Printf("通知邮件发送失败 user=%d type=%s: %v", userID, typ, err)
		}
	}()
}
