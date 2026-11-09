package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"folio/server/cache"
	"folio/server/config"
	"folio/server/db"
	"folio/server/handlers"
	"folio/server/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置不安全，已拒绝启动: %v", err)
	}
	if !cfg.Demo && cfg.SiteURL == "" {
		log.Printf("警告：未设置 FOLIO_SITE_URL，RSS、sitemap 和邮件链接可能受请求 Host 影响")
	}
	if cfg.StorageProvider != "local" && cfg.StorageProvider != "qiniu" {
		log.Fatalf("FOLIO_STORAGE 只能是 local 或 qiniu，当前为 %q", cfg.StorageProvider)
	}
	if cfg.StorageProvider == "qiniu" && !cfg.QiniuEnabled() {
		log.Printf("FOLIO_STORAGE=qiniu 但环境变量凭据不完整；若后台也未保存配置，将回退本地存储")
	}
	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	log.Printf("数据库模式: %s", cfg.DBDriver)

	if created, err := db.EnsureBootstrapAdmin(gdb, cfg); err != nil {
		log.Fatalf("创建初始管理员失败: %v", err)
	} else if created {
		log.Printf("已根据 FOLIO_ADMIN_* 创建超级管理员: %s", cfg.AdminUsername)
	}
	if !cfg.Demo {
		if count, err := db.UserCount(gdb); err == nil && count == 0 {
			log.Printf("警告：数据库为空。若站点已对公网开放，第一位注册用户会成为超级管理员；建议设置 FOLIO_ADMIN_USERNAME/FOLIO_ADMIN_PASSWORD，或先在本机完成注册并关闭公开注册。")
		}
	}

	if cfg.Demo {
		if err := seed.Run(gdb, cfg); err != nil {
			log.Printf("演示数据初始化失败: %v", err)
		}
		if err := seed.EnsureDailyStats(gdb); err != nil {
			log.Printf("演示阅读统计初始化失败: %v", err)
		}
	}
	if err := db.EnsureFavoriteFolders(gdb); err != nil {
		log.Printf("收藏夹初始化失败: %v", err)
	}
	if err := db.EnsureSiteSetting(gdb, cfg); err != nil {
		log.Printf("站点设置初始化失败: %v", err)
	}
	if err := db.CleanupDailyStats(gdb); err != nil {
		log.Printf("清理阅读统计失败: %v", err)
	}
	if err := db.PublishScheduled(gdb); err != nil {
		log.Printf("发布定时内容失败: %v", err)
	}
	if err := db.CleanupSessions(gdb); err != nil {
		log.Printf("清理登录会话失败: %v", err)
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for tick := 1; ; tick++ {
			<-ticker.C
			if err := db.PublishScheduled(gdb); err != nil {
				log.Printf("发布定时内容失败: %v", err)
			}
			// 每小时清理一次过期会话和已不存在内容的统计数据。
			if tick%60 == 0 {
				if err := db.CleanupSessions(gdb); err != nil {
					log.Printf("清理登录会话失败: %v", err)
				}
				if err := db.CleanupDailyStats(gdb); err != nil {
					log.Printf("清理阅读统计失败: %v", err)
				}
			}
		}
	}()

	rcache := cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer rcache.Close()

	api := handlers.New(gdb, cfg, rcache)
	log.Printf("图片存储: %s", api.Storage.Provider())
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxyList()); err != nil {
		log.Fatalf("设置可信代理失败: %v", err)
	}
	// /api/ws 的凭据已改走子协议，但仍跳过整个路径，避免旧客户端把 token 带进日志。
	r.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/api/ws"}}),
		gin.Recovery(),
		handlers.SecurityHeaders(cfg),
		handlers.BodyLimit(cfg.MaxUploadMB),
		handlers.CORS(cfg),
	)
	api.Routes(r)

	uploadDir, err := filepath.Abs(cfg.UploadDir)
	if err != nil {
		log.Fatalf("上传目录无效: %v", err)
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}
	r.Static("/uploads", uploadDir)

	dist, _ := filepath.Abs(cfg.WebDist)
	serveWeb := false
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err == nil {
		serveWeb = true
	}

	if serveWeb {
		r.Static("/assets", filepath.Join(dist, "assets"))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
				return
			}
			if strings.HasPrefix(path, "/uploads/") {
				c.String(http.StatusNotFound, "not found")
				return
			}
			rel := strings.TrimPrefix(filepath.Clean(path), "/")
			p := filepath.Join(dist, rel)
			if p == dist || strings.HasPrefix(p, dist+string(filepath.Separator)) {
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					c.File(p)
					return
				}
			}
			api.ServeIndex(c, dist)
		})
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
		})
	}

	location := cfg.DBDriver
	if cfg.DBDriver == "sqlite" {
		location += " " + cfg.DBPath
	}
	displayAddr := cfg.Addr
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "http://localhost" + displayAddr
	} else {
		displayAddr = "http://" + displayAddr
	}
	log.Printf("知微已启动：%s（数据库：%s）", displayAddr, location)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
