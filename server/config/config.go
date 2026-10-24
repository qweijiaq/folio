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

	// 上传与对象存储：local（默认）| qiniu
	StorageProvider string
	UploadDir       string
	MaxUploadMB     int

	QiniuAccessKey string
	QiniuSecretKey string
	QiniuBucket    string
	QiniuDomain    string
	QiniuRegion    string
	QiniuPrefix    string

	// AI（可选，官方 Anthropic API）
	AIAPIKey  string
	AIModel   string
	AIBaseURL string

	// 邮件（可选，用于邮箱验证码、邮箱登录与找回密码）
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
	SMTPTLS      bool
	MailDebug    bool

	// 登录图形验证码（可选）
	LoginCaptcha bool
	CaptchaDebug bool

	// QQ 登录（可选）
	QQAppID    string
	QQAppKey   string
	QQRedirect string
	QQAPIBase  string
	WebURL     string

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

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (c Config) AIEnabled() bool {
	return strings.TrimSpace(c.AIAPIKey) != ""
}

func (c Config) QiniuEnabled() bool {
	return strings.TrimSpace(c.QiniuAccessKey) != "" &&
		strings.TrimSpace(c.QiniuSecretKey) != "" &&
		strings.TrimSpace(c.QiniuBucket) != "" &&
		strings.TrimSpace(c.QiniuDomain) != ""
}

func (c Config) QQEnabled() bool {
	return strings.TrimSpace(c.QQAppID) != "" &&
		strings.TrimSpace(c.QQAppKey) != "" &&
		strings.TrimSpace(c.QQRedirect) != ""
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
	provider := strings.ToLower(getenv("FOLIO_STORAGE", "local"))
	smtpPort, _ := strconv.Atoi(getenv("FOLIO_SMTP_PORT", "587"))
	if smtpPort <= 0 || smtpPort > 65535 {
		smtpPort = 587
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

		StorageProvider: provider,
		UploadDir:       getenv("FOLIO_UPLOAD_DIR", "./data/uploads"),
		MaxUploadMB:     uploadMB,

		QiniuAccessKey: getenv("FOLIO_QINIU_ACCESS_KEY", ""),
		QiniuSecretKey: getenv("FOLIO_QINIU_SECRET_KEY", ""),
		QiniuBucket:    getenv("FOLIO_QINIU_BUCKET", ""),
		QiniuDomain:    strings.TrimRight(getenv("FOLIO_QINIU_DOMAIN", ""), "/"),
		QiniuRegion:    getenv("FOLIO_QINIU_REGION", ""),
		QiniuPrefix:    strings.Trim(getenv("FOLIO_QINIU_PREFIX", "folio"), "/"),

		AIAPIKey:  getenv("FOLIO_AI_API_KEY", getenv("ANTHROPIC_API_KEY", "")),
		AIModel:   getenv("FOLIO_AI_MODEL", "claude-opus-5-5"),
		AIBaseURL: getenv("FOLIO_AI_BASE_URL", ""),

		SMTPHost:     getenv("FOLIO_SMTP_HOST", ""),
		SMTPPort:     smtpPort,
		SMTPUser:     getenv("FOLIO_SMTP_USER", ""),
		SMTPPassword: getenv("FOLIO_SMTP_PASSWORD", ""),
		SMTPFrom:     getenv("FOLIO_SMTP_FROM", ""),
		SMTPFromName: getenv("FOLIO_SMTP_FROM_NAME", ""),
		SMTPTLS:      envBool("FOLIO_SMTP_TLS", smtpPort == 465),
		MailDebug:    envBool("FOLIO_MAIL_DEBUG", false),

		LoginCaptcha: envBool("FOLIO_LOGIN_CAPTCHA", false),
		CaptchaDebug: envBool("FOLIO_CAPTCHA_DEBUG", false),

		QQAppID:    getenv("FOLIO_QQ_APP_ID", ""),
		QQAppKey:   getenv("FOLIO_QQ_APP_KEY", ""),
		QQRedirect: getenv("FOLIO_QQ_REDIRECT", ""),
		QQAPIBase:  getenv("FOLIO_QQ_API_BASE", "https://graph.qq.com"),
		WebURL:     strings.TrimRight(getenv("FOLIO_WEB_URL", ""), "/"),

		RedisAddr:     getenv("FOLIO_REDIS_ADDR", ""),
		RedisPassword: getenv("FOLIO_REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
	}
}
