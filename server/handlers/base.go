package handlers

import (
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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type API struct {
	DB    *gorm.DB
	Cfg   config.Config
	Cache *cache.Cache
	AI    *anthropic.Client
	Mail  foliomail.Mailer
}

func New(db *gorm.DB, cfg config.Config, rc *cache.Cache) *API {
	if rc == nil {
		rc = &cache.Cache{}
	}
	var aiClient *anthropic.Client
	if cfg.AIEnabled() {
		options := []option.RequestOption{option.WithAPIKey(cfg.AIAPIKey)}
		if strings.TrimSpace(cfg.AIBaseURL) != "" {
			options = append(options, option.WithBaseURL(strings.TrimRight(cfg.AIBaseURL, "/")))
		}
		client := anthropic.NewClient(options...)
		aiClient = &client
	}
	return &API{DB: db, Cfg: cfg, Cache: rc, AI: aiClient, Mail: foliomail.New(cfg)}
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
	jwt.RegisteredClaims
}

func randomTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func (a *API) issueToken(u *models.User) (string, error) {
	claims := tokenClaims{
		UserID: u.ID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomTokenID(),
			Subject:   u.Username,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.Cfg.Secret))
}

func (a *API) userFromRequest(c *gin.Context) *models.User {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return nil
	}
	var claims tokenClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(a.Cfg.Secret), nil
	})
	if err != nil || !t.Valid {
		return nil
	}
	if a.Cache.IsBlacklisted(c.Request.Context(), claims.ID) {
		return nil
	}
	c.Set("claims", &claims)
	var u models.User
	if err := a.DB.First(&u, claims.UserID).Error; err != nil {
		return nil
	}
	return &u
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
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
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
