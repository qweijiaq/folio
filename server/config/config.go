package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr      string
	Secret    string
	WebDist   string
	Demo      bool
	SiteName  string
	SiteDesc  string
	SiteOwner string

	// 数据库：sqlite（默认）| mysql | memory（进程内 SQLite，重启即清空）
	DBDriver string
	DBPath   string
	MySQLDSN string

	// 上传
	UploadDir   string
	MaxUploadMB int

	// Redis 可选：地址为空时自动降级为无缓存模式
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func Load() Config {
	secret := getenv("FOLIO_SECRET", "folio-dev-secret-change-me")
	driver := strings.ToLower(getenv("FOLIO_DB_DRIVER", "sqlite"))
	dbPath := getenv("FOLIO_DB", "./data/folio.db")

	if getenv("FOLIO_MEMORY", "0") == "1" || dbPath == ":memory:" {
		driver = "memory"
	}
	if driver != "mysql" && driver != "memory" {
		driver = "sqlite"
	}

	demo := getenv("FOLIO_DEMO", "1") != "0"
	if driver == "memory" {
		// 内存模式每次启动都是空库，必须写入演示数据
		demo = true
	}

	redisDB, _ := strconv.Atoi(getenv("FOLIO_REDIS_DB", "0"))
	uploadMB, _ := strconv.Atoi(getenv("FOLIO_UPLOAD_MAX_MB", "10"))
	if uploadMB < 1 {
		uploadMB = 10
	}
	return Config{
		Addr:      getenv("FOLIO_ADDR", ":8080"),
		Secret:    secret,
		WebDist:   getenv("FOLIO_WEB", "../web/dist"),
		Demo:      demo,
		SiteName:  getenv("FOLIO_SITE_NAME", "知微"),
		SiteDesc:  getenv("FOLIO_SITE_DESC", "一个人的知识花园，存放文章、手册与零散的思考。"),
		SiteOwner: getenv("FOLIO_SITE_OWNER", "weijia"),

		DBDriver: driver,
		DBPath:   dbPath,
		MySQLDSN: getenv("FOLIO_MYSQL_DSN", ""),

		UploadDir:   getenv("FOLIO_UPLOAD_DIR", "./data/uploads"),
		MaxUploadMB: uploadMB,

		RedisAddr:     getenv("FOLIO_REDIS_ADDR", ""),
		RedisPassword: getenv("FOLIO_REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
	}
}
