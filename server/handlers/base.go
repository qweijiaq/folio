package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"folio/server/cache"
	"folio/server/config"
	foliomail "folio/server/mail"
	"folio/server/models"
	"folio/server/storage"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mojocn/base64Captcha"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type API struct {
	DB      *gorm.DB
	Cfg     config.Config
	Cache   *cache.Cache
	Mail    foliomail.Mailer
	Captcha base64Captcha.Store
	Hub     *wsHub
	Storage *storage.Manager
	Limiter *rateLimiter
}

func New(db *gorm.DB, cfg config.Config, rc *cache.Cache) *API {
	if rc == nil {
		rc = &cache.Cache{}
	}
	api := &API{
		DB:      db,
		Cfg:     cfg,
		Cache:   rc,
		Mail:    foliomail.New(cfg),
		Captcha: base64Captcha.NewMemoryStore(20240, 5*time.Minute),
		Hub:     newWSHub(),
		Storage: storage.New(cfg),
		Limiter: newRateLimiter(),
	}
	api.loadStorageSetting()
	return api
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

func bad(c *gin.Context, msg string) { fail(c, http.StatusBadRequest, msg) }

func pageParams(c *gin.Context) (page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("pageSize", "12"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 12
	}
	return
}

type tokenClaims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	Epoch  int    `json:"epoch"`
	jwt.RegisteredClaims
}

func randomTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func (a *API) issueToken(c *gin.Context, u *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour)
	claims := tokenClaims{
		UserID: u.ID,
		Role:   u.Role,
		Epoch:  u.AuthEpoch,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomTokenID(),
			Subject:   u.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.Cfg.Secret))
	if err != nil {
		return "", err
	}
	ua := c.GetHeader("User-Agent")
	session := models.UserSession{
		UserID:     u.ID,
		TokenID:    claims.ID,
		IP:         clipText(c.ClientIP(), 64),
		UserAgent:  clipText(ua, 255),
		Device:     clipText(deviceLabel(ua), 64),
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
	}
	if err := a.DB.Create(&session).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (a *API) userFromToken(token string, ctx context.Context) (*models.User, *tokenClaims) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	var claims tokenClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(a.Cfg.Secret), nil
	})
	if err != nil || !t.Valid {
		return nil, nil
	}
	if a.Cache.IsBlacklisted(ctx, claims.ID) {
		return nil, nil
	}
	var u models.User
	if err := a.DB.First(&u, claims.UserID).Error; err != nil {
		return nil, &claims
	}
	if claims.Epoch != u.AuthEpoch {
		return nil, &claims
	}
	return &u, &claims
}

func (a *API) userFromRequest(c *gin.Context) *models.User {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}
	u, claims := a.userFromToken(strings.TrimPrefix(header, "Bearer "), c.Request.Context())
	if u == nil || claims == nil || !a.sessionAllowed(c, u, claims) {
		return nil
	}
	c.Set("claims", claims)
	return u
}

func (a *API) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := a.userFromRequest(c)
		if u == nil {
			fail(c, http.StatusUnauthorized, "请先登录")
			return
		}
		c.Set("user", u)
		c.Next()
	}
}

func (a *API) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := a.userFromRequest(c); u != nil {
			c.Set("user", u)
		}
		c.Next()
	}
}

func (a *API) SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperAdmin(currentUser(c)) {
			fail(c, http.StatusForbidden, "需要超级管理员权限")
			return
		}
		c.Next()
	}
}

func (a *API) StaffOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isStaff(currentUser(c)) {
			fail(c, http.StatusForbidden, "只有管理员可以发布内容")
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) *models.User {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}

func currentClaims(c *gin.Context) *tokenClaims {
	v, ok := c.Get("claims")
	if !ok {
		return nil
	}
	claims, _ := v.(*tokenClaims)
	return claims
}

func paramID(c *gin.Context) (uint, bool) {
	return parseParam(c, "id")
}

func parseParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		bad(c, "参数错误")
		return 0, false
	}
	return uint(id), true
}

func hashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}

func validEmail(s string) bool {
	if s == "" {
		return true
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func trim(s string) string { return strings.TrimSpace(s) }

func likePattern(s string) string {
	s = strings.ReplaceAll(s, "!", "!!")
	s = strings.ReplaceAll(s, "%", "!%")
	s = strings.ReplaceAll(s, "_", "!_")
	return "%" + s + "%"
}

func isSuperAdmin(u *models.User) bool {
	return u != nil && u.Role == models.RoleSuperAdmin
}

func isStaff(u *models.User) bool {
	return u != nil && (u.Role == models.RoleSuperAdmin || u.Role == models.RoleAdmin)
}

func canEdit(u *models.User, d *models.Document) bool {
	return u != nil && (d.AuthorID == u.ID || isSuperAdmin(u))
}
