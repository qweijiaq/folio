package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"folio/server/cache"
	"folio/server/config"
	"folio/server/db"
	"folio/server/handlers"
	"folio/server/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	gdb, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	log.Printf("数据库模式: %s", cfg.DBDriver)

	if cfg.Demo {
		if err := seed.Run(gdb, cfg); err != nil {
			log.Printf("演示数据初始化失败: %v", err)
		}
	}
	if err := db.EnsureFavoriteFolders(gdb); err != nil {
		log.Printf("收藏夹初始化失败: %v", err)
	}

	rcache := cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer rcache.Close()

	api := handlers.New(gdb, cfg, rcache)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), handlers.CORS(cfg))
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
			c.File(filepath.Join(dist, "index.html"))
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
	log.Printf("知微已启动：http://localhost%s（数据库：%s）", cfg.Addr, location)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}
