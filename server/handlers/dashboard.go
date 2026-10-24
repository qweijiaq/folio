package handlers

import (
	"net/http"
	"runtime"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var serverStartedAt = time.Now()

func startOfLocalDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (a *API) AdminDashboard(c *gin.Context) {
	days := 7
	switch c.DefaultQuery("range", "7") {
	case "30":
		days = 30
	case "90":
		days = 90
	}
	now := time.Now()
	start := startOfLocalDay(now.AddDate(0, 0, -(days - 1)))
	dates := make([]string, days)
	dateIndex := make(map[string]int, days)
	for i := 0; i < days; i++ {
		dates[i] = start.AddDate(0, 0, i).Format("2006-01-02")
		dateIndex[dates[i]] = i
	}
	views := make([]int, days)
	content := make([]int, days)
	users := make([]int, days)
	logins := make([]int, days)

	var daily []models.DocDailyStat
	a.DB.Where("day >= ?", dates[0]).Find(&daily)
	for _, stat := range daily {
		if index, ok := dateIndex[stat.Day]; ok {
			views[index] += stat.Views
		}
	}

	var recentDocs []models.Document
	a.DB.Select("created_at").Where("created_at >= ?", start).Find(&recentDocs)
	for _, doc := range recentDocs {
		if index, ok := dateIndex[doc.CreatedAt.Format("2006-01-02")]; ok {
			content[index]++
		}
	}

	var recentUsers []models.User
	a.DB.Select("created_at").Where("created_at >= ?", start).Find(&recentUsers)
	for _, user := range recentUsers {
		if index, ok := dateIndex[user.CreatedAt.Format("2006-01-02")]; ok {
			users[index]++
		}
	}

	var loginLogs []models.OperationLog
	a.DB.Select("created_at").Where("action = ? AND created_at >= ?", "登录", start).Find(&loginLogs)
	for _, entry := range loginLogs {
		if index, ok := dateIndex[entry.CreatedAt.Format("2006-01-02")]; ok {
			logins[index]++
		}
	}

	var articles, docs, drafts, userCount, comments, likes, favorites, messages, totalViews int64
	a.DB.Model(&models.Document{}).Where("kind = ? AND status = ?", models.KindArticle, models.StatusPublished).Count(&articles)
	a.DB.Model(&models.Document{}).Where("kind = ? AND status = ?", models.KindDoc, models.StatusPublished).Count(&docs)
	a.DB.Model(&models.Document{}).Where("status = ?", models.StatusDraft).Count(&drafts)
	a.DB.Model(&models.User{}).Count(&userCount)
	a.DB.Model(&models.Comment{}).Count(&comments)
	a.DB.Model(&models.Like{}).Count(&likes)
	a.DB.Model(&models.Favorite{}).Count(&favorites)
	a.DB.Model(&models.Message{}).Count(&messages)
	a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished).
		Select("COALESCE(SUM(view_count), 0)").Scan(&totalViews)

	var todayLogins int64
	a.DB.Model(&models.OperationLog{}).Where("action = ? AND created_at >= ?", "登录", startOfLocalDay(now)).Count(&todayLogins)

	var imageCount int64
	var uploadBytes int64
	a.DB.Model(&models.Image{}).Count(&imageCount)
	a.DB.Model(&models.Image{}).Select("COALESCE(SUM(size), 0)").Scan(&uploadBytes)

	var top []models.Document
	a.DB.Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Preload("Category").
		Where("status = ?", models.StatusPublished).
		Order("view_count DESC, like_count DESC, id DESC").Limit(6).Find(&top)
	topItems := make([]gin.H, 0, len(top))
	for _, doc := range top {
		topItems = append(topItems, gin.H{
			"id":           doc.ID,
			"kind":         doc.Kind,
			"title":        doc.Title,
			"viewCount":    doc.ViewCount,
			"likeCount":    doc.LikeCount,
			"commentCount": doc.CommentCount,
			"category":     doc.Category,
			"author":       toBrief(doc.Author),
		})
	}

	var activity []models.OperationLog
	a.DB.Where("created_at >= ?", start).Order("id DESC").Limit(8).Find(&activity)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	redisStatus := "off"
	if a.Cache.Enabled() {
		redisStatus = "up"
		if err := a.Cache.Ping(c.Request.Context()); err != nil {
			redisStatus = "down"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"range": days,
		"summary": gin.H{
			"articles":    articles,
			"docs":        docs,
			"drafts":      drafts,
			"users":       userCount,
			"comments":    comments,
			"likes":       likes,
			"favorites":   favorites,
			"messages":    messages,
			"views":       totalViews,
			"viewsRange":  sumInts(views),
			"newUsers":    sumInts(users),
			"newContent":  sumInts(content),
			"todayLogins": todayLogins,
			"images":      imageCount,
			"uploadBytes": uploadBytes,
		},
		"trend": gin.H{
			"dates":   dates,
			"views":   views,
			"content": content,
			"users":   users,
			"logins":  logins,
		},
		"top":      topItems,
		"activity": activity,
		"system": gin.H{
			"db":          a.Cfg.DBDriver,
			"redis":       redisStatus,
			"storage":     a.Cfg.StorageProvider,
			"goVersion":   runtime.Version(),
			"goroutines":  runtime.NumGoroutine(),
			"memoryBytes": mem.Alloc,
			"uptimeSec":   int64(time.Since(serverStartedAt).Seconds()),
		},
		"generatedAt": now,
	})
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}
