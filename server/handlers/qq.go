package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var qqHTTPClient = &http.Client{Timeout: 12 * time.Second}

type qqTokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Error            int    `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type qqOpenIDResponse struct {
	ClientID         string `json:"client_id"`
	OpenID           string `json:"openid"`
	Error            int    `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type qqUserInfoResponse struct {
	Ret       int    `json:"ret"`
	Msg       string `json:"msg"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"figureurl_qq_1"`
}

func (a *API) QQStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": a.qqLoginEnabled()})
}

func (a *API) issueQQState() (string, error) {
	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomTokenID(),
			Subject:   "qq-state",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.Cfg.Secret))
}

func (a *API) issueQQTicket(user *models.User) (string, error) {
	claims := tokenClaims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomTokenID(),
			Subject:   "qq-ticket",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.Cfg.Secret))
}

func (a *API) parseQQClaims(raw, subject string) (*tokenClaims, bool) {
	var claims tokenClaims
	token, err := jwt.ParseWithClaims(strings.TrimSpace(raw), &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(a.Cfg.Secret), nil
	})
	if err != nil || !token.Valid || claims.Subject != subject {
		return nil, false
	}
	return &claims, true
}

func (a *API) QQURL(c *gin.Context) {
	if !a.qqLoginEnabled() {
		fail(c, http.StatusServiceUnavailable, "QQ 登录未启用")
		return
	}
	state, err := a.issueQQState()
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成登录状态失败")
		return
	}
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", a.Cfg.QQAppID)
	query.Set("redirect_uri", a.Cfg.QQRedirect)
	query.Set("state", state)
	query.Set("scope", "get_user_info")
	base := strings.TrimRight(a.Cfg.QQAPIBase, "/")
	c.JSON(http.StatusOK, gin.H{"url": base + "/oauth2.0/authorize?" + query.Encode()})
}

func (a *API) qqFrontendURL(path string, query url.Values) string {
	return strings.TrimRight(a.Cfg.WebURL, "/") + path + "?" + query.Encode()
}

func (a *API) qqRedirectError(c *gin.Context, message string) {
	query := url.Values{}
	query.Set("qq_error", message)
	c.Redirect(http.StatusFound, a.qqFrontendURL("/login", query))
}

func fetchQQJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := qqHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(body))
	// 兼容 QQ 偶尔返回 callback( {...} ); 的 JSONP 形态。
	if strings.HasPrefix(text, "callback(") {
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start >= 0 && end > start {
			text = text[start : end+1]
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("QQ 接口返回 HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal([]byte(text), out)
}

func (a *API) QQCallback(c *gin.Context) {
	if !a.qqLoginEnabled() {
		a.qqRedirectError(c, "QQ 登录未启用")
		return
	}
	if _, ok := a.parseQQClaims(c.Query("state"), "qq-state"); !ok {
		a.qqRedirectError(c, "QQ 登录状态已失效，请重试")
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		a.qqRedirectError(c, "QQ 没有返回授权码")
		return
	}
	base := strings.TrimRight(a.Cfg.QQAPIBase, "/")

	tokenQuery := url.Values{}
	tokenQuery.Set("grant_type", "authorization_code")
	tokenQuery.Set("client_id", a.Cfg.QQAppID)
	tokenQuery.Set("client_secret", a.Cfg.QQAppKey)
	tokenQuery.Set("code", code)
	tokenQuery.Set("redirect_uri", a.Cfg.QQRedirect)
	tokenQuery.Set("fmt", "json")
	var tokenResp qqTokenResponse
	if err := fetchQQJSON(c.Request.Context(), base+"/oauth2.0/token?"+tokenQuery.Encode(), &tokenResp); err != nil {
		a.qqRedirectError(c, "连接 QQ 登录服务失败")
		return
	}
	if tokenResp.Error != 0 || tokenResp.AccessToken == "" {
		a.qqRedirectError(c, "QQ 授权失败，请重试")
		return
	}

	openQuery := url.Values{}
	openQuery.Set("access_token", tokenResp.AccessToken)
	openQuery.Set("fmt", "json")
	var openResp qqOpenIDResponse
	if err := fetchQQJSON(c.Request.Context(), base+"/oauth2.0/me?"+openQuery.Encode(), &openResp); err != nil {
		a.qqRedirectError(c, "读取 QQ 账号信息失败")
		return
	}
	if openResp.Error != 0 || openResp.OpenID == "" {
		a.qqRedirectError(c, "QQ 账号信息不完整")
		return
	}

	infoQuery := url.Values{}
	infoQuery.Set("access_token", tokenResp.AccessToken)
	infoQuery.Set("oauth_consumer_key", a.Cfg.QQAppID)
	infoQuery.Set("openid", openResp.OpenID)
	var info qqUserInfoResponse
	if err := fetchQQJSON(c.Request.Context(), base+"/user/get_user_info?"+infoQuery.Encode(), &info); err != nil {
		a.qqRedirectError(c, "读取 QQ 昵称失败")
		return
	}
	if info.Ret != 0 {
		a.qqRedirectError(c, "QQ 没有返回可用的用户信息")
		return
	}

	var user models.User
	err := a.DB.Where("qq_open_id = ?", openResp.OpenID).First(&user).Error
	isNew := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		nickname := trim(info.Nickname)
		if nickname == "" {
			nickname = "QQ 用户"
		}
		if runeLen(nickname) > 24 {
			nickname = string([]rune(nickname)[:24])
		}
		var total int64
		a.DB.Model(&models.User{}).Count(&total)
		role := models.RoleUser
		if total == 0 {
			role = models.RoleSuperAdmin
		}
		password, err := hashPassword(randomTokenID())
		if err != nil {
			a.qqRedirectError(c, "创建账号失败")
			return
		}
		user = models.User{
			Username: a.uniqueQQUsername(),
			Nickname: nickname,
			Password: password,
			Role:     role,
			QQOpenID: openResp.OpenID,
		}
		if err := a.DB.Create(&user).Error; err != nil {
			a.qqRedirectError(c, "创建账号失败")
			return
		}
		_, _ = a.ensureDefaultFolder(user.ID)
		isNew = true
	} else if err != nil {
		a.qqRedirectError(c, "读取账号失败")
		return
	}

	now := time.Now()
	a.DB.Model(&user).UpdateColumn("last_login_at", now)
	action := "登录"
	detail := "QQ 登录"
	if isNew {
		action = "注册账号"
		detail = "通过 QQ 登录自动创建账号"
	}
	a.recordLog(c, &user, action, "user", user.ID, detail)

	ticket, err := a.issueQQTicket(&user)
	if err != nil {
		a.qqRedirectError(c, "登录失败，请重试")
		return
	}
	query := url.Values{}
	query.Set("qq_ticket", ticket)
	c.Redirect(http.StatusFound, a.qqFrontendURL("/login", query))
}

func (a *API) QQExchange(c *gin.Context) {
	var in struct {
		Ticket string `json:"ticket"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	claims, ok := a.parseQQClaims(in.Ticket, "qq-ticket")
	if !ok {
		bad(c, "登录凭证已失效，请重新登录")
		return
	}
	var user models.User
	if err := a.DB.First(&user, claims.UserID).Error; err != nil {
		bad(c, "账号不存在")
		return
	}
	token, err := a.issueToken(&user)
	if err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": a.mePayload(&user)})
}

func (a *API) uniqueQQUsername() string {
	for i := 0; i < 20; i++ {
		name := "qq_" + randomTokenID()[:8]
		var count int64
		a.DB.Model(&models.User{}).Where("lower(username) = lower(?)", name).Count(&count)
		if count == 0 {
			return name
		}
	}
	return "qq_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
