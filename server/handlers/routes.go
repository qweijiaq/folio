package handlers

import (
	"net/http"
	"time"

	"folio/server/art"
	"folio/server/config"

	"github.com/gin-gonic/gin"
)

func CORS(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (a *API) Routes(r *gin.Engine) {
	r.GET("/feed.xml", a.RSSFeed)
	r.HEAD("/feed.xml", a.RSSFeed)
	r.GET("/sitemap.xml", a.Sitemap)
	r.HEAD("/sitemap.xml", a.Sitemap)
	r.GET("/robots.txt", a.Robots)
	r.HEAD("/robots.txt", a.Robots)

	g := r.Group("/api")

	g.GET("/health", a.Health)
	g.GET("/ws", a.WebSocket)

	// 生成式视觉资源
	g.GET("/art/cover.svg", a.CoverArt)
	g.GET("/art/cover.png", a.CoverPNG)
	g.HEAD("/art/cover.png", a.CoverPNG)
	g.GET("/art/avatar.svg", a.AvatarArt)

	// 站点与发现
	g.GET("/site", a.SiteHome)
	g.GET("/tags", a.Tags)
	g.GET("/categories", a.CategoryList)
	g.GET("/doc-tree", a.DocTree)
	g.GET("/users", a.UserSearch)
	g.GET("/users/:username", a.OptionalAuth(), a.UserProfile)
	g.GET("/users/:username/followers", a.OptionalAuth(), a.Followers)
	g.GET("/users/:username/following", a.OptionalAuth(), a.Following)

	// 内容（统一文档表）。只有超级管理员和管理员可以创作。
	g.GET("/docs", a.OptionalAuth(), a.DocList)
	g.GET("/docs/:id", a.OptionalAuth(), a.DocDetail)
	g.GET("/docs/:id/related", a.RelatedDocs)
	g.GET("/docs/:id/edit", a.Auth(), a.StaffOnly(), a.DocEdit)
	g.GET("/docs/:id/comments", a.OptionalAuth(), a.CommentList)
	g.POST("/docs", a.Auth(), a.StaffOnly(), a.DocCreate)
	g.PUT("/docs/:id", a.Auth(), a.StaffOnly(), a.DocUpdate)
	g.DELETE("/docs/:id", a.Auth(), a.StaffOnly(), a.DocDelete)
	g.POST("/docs/:id/like", a.Auth(), a.DocLike)
	g.POST("/docs/:id/favorite", a.Auth(), a.DocFavorite)
	g.POST("/docs/:id/comments", a.Auth(), a.CommentCreate)

	// 图片：只有可以写作的角色能上传，图片本身通过 /uploads 静态访问
	g.POST("/images", a.Auth(), a.ImageUpload)
	g.GET("/images", a.Auth(), a.StaffOnly(), a.ImageList)
	g.DELETE("/images/:id", a.Auth(), a.StaffOnly(), a.ImageDelete)

	g.DELETE("/comments/:id", a.Auth(), a.CommentDelete)
	g.POST("/comments/:id/like", a.Auth(), a.CommentLike)

	// 文章分类
	g.POST("/categories", a.Auth(), a.StaffOnly(), a.CategoryCreate)
	g.PUT("/categories/:id", a.Auth(), a.StaffOnly(), a.CategoryUpdate)
	g.DELETE("/categories/:id", a.Auth(), a.StaffOnly(), a.CategoryDelete)

	// 认证
	g.GET("/auth/captcha", a.CaptchaImage)
	g.GET("/auth/qq/status", a.QQStatus)
	g.GET("/auth/qq/url", a.QQURL)
	g.GET("/auth/qq/callback", a.QQCallback)
	g.POST("/auth/qq/exchange", a.QQExchange)
	g.GET("/auth/email/status", a.EmailStatus)
	g.POST("/auth/email/code", a.OptionalAuth(), a.SendEmailCode)
	g.POST("/auth/email/login", a.EmailLogin)
	g.POST("/auth/password/reset", a.ResetPassword)
	g.POST("/auth/register", a.Register)
	g.POST("/auth/login", a.Login)

	authed := g.Group("")
	authed.Use(a.Auth())
	authed.POST("/auth/logout", a.Logout)
	authed.GET("/me", a.Me)
	authed.PUT("/me", a.MeUpdate)
	authed.PUT("/me/email", a.BindEmail)
	authed.PUT("/me/password", a.MePassword)
	authed.GET("/me/settings", a.MeSettings)
	authed.GET("/me/login-records", a.MyLoginRecords)
	authed.PUT("/me/settings", a.MeSettingsUpdate)
	authed.GET("/me/docs", a.MyDocs)
	authed.GET("/me/favorites", a.MyFavorites)
	authed.GET("/me/history", a.MyHistory)
	authed.DELETE("/me/history", a.ClearHistory)
	authed.GET("/favorite-folders", a.FavoriteFolderList)
	authed.POST("/favorite-folders", a.FavoriteFolderCreate)
	authed.PUT("/favorite-folders/:id", a.FavoriteFolderUpdate)
	authed.DELETE("/favorite-folders/:id", a.FavoriteFolderDelete)
	authed.GET("/ai/status", a.StaffOnly(), a.AIStatus)
	authed.POST("/ai/analyze", a.StaffOnly(), a.AIAnalyze)
	authed.GET("/inbox", a.Inbox)

	authed.POST("/users/:username/follow", a.FollowUser)
	authed.DELETE("/users/:username/follow", a.UnfollowUser)

	authed.GET("/notifications", a.Notifications)
	authed.POST("/notifications/read", a.NotificationsRead)
	authed.GET("/notices", a.Notices)
	authed.POST("/notices/read", a.NoticesRead)

	authed.GET("/messages/conversations", a.Conversations)
	authed.GET("/messages/with/:username", a.MessageThread)
	authed.POST("/messages/with/:username", a.MessageSend)

	admin := g.Group("/admin")
	admin.Use(a.Auth())
	admin.GET("/stats", a.StaffOnly(), a.AdminStats)
	admin.GET("/dashboard", a.StaffOnly(), a.AdminDashboard)
	admin.GET("/settings", a.SuperAdminOnly(), a.AdminSettings)
	admin.PUT("/settings", a.SuperAdminOnly(), a.AdminSettingsUpdate)
	admin.GET("/logs", a.SuperAdminOnly(), a.AdminLogs)
	admin.GET("/notices", a.StaffOnly(), a.AdminNotices)
	admin.POST("/notices", a.StaffOnly(), a.AdminNoticeCreate)
	admin.DELETE("/notices/:id", a.StaffOnly(), a.AdminNoticeDelete)
	admin.GET("/docs", a.StaffOnly(), a.AdminDocs)
	admin.GET("/users", a.SuperAdminOnly(), a.AdminUsers)
	admin.POST("/users", a.SuperAdminOnly(), a.AdminCreateUser)
	admin.GET("/invites", a.SuperAdminOnly(), a.AdminInvites)
	admin.POST("/invites", a.SuperAdminOnly(), a.AdminInviteCreate)
	admin.DELETE("/invites/:id", a.SuperAdminOnly(), a.AdminInviteDelete)
	admin.DELETE("/users/:id", a.SuperAdminOnly(), a.AdminDeleteUser)
	admin.PUT("/users/:id/role", a.SuperAdminOnly(), a.AdminUpdateRole)
}

func (a *API) Health(c *gin.Context) {
	dbStatus := "up"
	if sqlDB, err := a.DB.DB(); err != nil {
		dbStatus = "down"
	} else if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		dbStatus = "down"
	}
	redisStatus := "off"
	if a.Cache.Enabled() {
		if err := a.Cache.Ping(c.Request.Context()); err != nil {
			redisStatus = "down"
		} else {
			redisStatus = "up"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       dbStatus == "up",
		"db":       a.Cfg.DBDriver,
		"dbStatus": dbStatus,
		"redis":    redisStatus,
		"time":     time.Now(),
	})
}

func (a *API) CoverArt(c *gin.Context) {
	seed := c.Query("seed")
	title := c.Query("title")
	kind := c.Query("kind")
	if seed == "" {
		seed = title
	}
	if runeLen(seed) > 120 {
		seed = string([]rune(seed)[:120])
	}
	if runeLen(title) > 80 {
		title = string([]rune(title)[:80])
	}
	c.Header("Content-Type", "image/svg+xml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.String(http.StatusOK, art.CoverSVG(seed, title, kind))
}

func (a *API) AvatarArt(c *gin.Context) {
	c.Header("Content-Type", "image/svg+xml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=604800")
	c.String(http.StatusOK, art.AvatarSVG(c.Query("seed")))
}
