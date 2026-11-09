package config

import (
	"errors"
	"os"
	"regexp"
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
	SiteURL   string

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

	// AI（可选，官方 Anthropic API；站点 Key 仅用于后台 AI 摘要）
	AIAPIKey  string
	AIModel   string
	AIBaseURL string

	// 问答服务商地址覆盖（可选，留空使用官方地址；代理部署时可改写）
	DeepSeekBaseURL string
	GLMBaseURL      string
	KimiBaseURL     string
	QwenBaseURL     string

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

	// 反向代理信任列表；只信任这些来源传来的 X-Forwarded-For。
	TrustedProxies string

	// 首次启动时可直接创建超级管理员，避免“第一个注册者成为超管”的竞态。
	AdminUsername string
	AdminPassword string
}

var adminUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,20}$`)

// Validate 拒绝不安全的默认配置，避免带着演示密钥或演示数据上公网。
func (c Config) Validate() error {
	if !c.Demo {
		secret := strings.TrimSpace(c.Secret)
		switch secret {
		case "", "folio-dev-secret-change-me", "please-change-this-secret", "change-me-to-a-long-random-string":
			return errors.New("非演示模式必须设置 FOLIO_SECRET，且不能使用示例默认值")
		}
		if len(secret) < 32 {
			return errors.New("非演示模式的 FOLIO_SECRET 至少需要 32 个字符")
		}
	}
	if c.AdminUsername != "" || c.AdminPassword != "" {
		if !adminUsernamePattern.MatchString(strings.TrimSpace(c.AdminUsername)) {
			return errors.New("FOLIO_ADMIN_USERNAME 需为 2-20 位字母、数字或下划线")
		}
		if len(c.AdminPassword) < 8 || len(c.AdminPassword) > 64 {
			return errors.New("FOLIO_ADMIN_PASSWORD 需为 8-64 位")
		}
	}
	return nil
}

// TrustedProxyList 返回允许携带 X-Forwarded-For 的代理地址。
// 默认只信任本机与 Docker 私网；公网直连时 Gin 会使用连接本身的 IP。
func (c Config) TrustedProxyList() []string {
	raw := strings.TrimSpace(c.TrustedProxies)
	if raw == "" {
		raw = "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	}
	items := make([]string, 0, 8)
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
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

	demo := getenv("FOLIO_DEMO", "0") != "0"
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
		SiteURL:   strings.TrimRight(getenv("FOLIO_SITE_URL", ""), "/"),

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

		DeepSeekBaseURL: getenv("FOLIO_DEEPSEEK_BASE_URL", ""),
		GLMBaseURL:      getenv("FOLIO_GLM_BASE_URL", ""),
		KimiBaseURL:     getenv("FOLIO_KIMI_BASE_URL", ""),
		QwenBaseURL:     getenv("FOLIO_QWEN_BASE_URL", ""),

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

		TrustedProxies: getenv("FOLIO_TRUSTED_PROXIES", ""),
		AdminUsername:  getenv("FOLIO_ADMIN_USERNAME", ""),
		AdminPassword:  os.Getenv("FOLIO_ADMIN_PASSWORD"),
	}
}
