package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

const (
	emailPurposeRegister = "register"
	emailPurposeReset    = "reset"
	emailPurposeBind     = "bind"
	emailPurposeLogin    = "login"

	emailCodeTTL         = 10 * time.Minute
	emailCodeCooldown    = 60 * time.Second
	emailCodeDailyLimit  = 10
	emailCodeMaxAttempts = 5
)

func (a *API) emailEnabled() bool {
	return a.Mail != nil && a.Mail.Enabled()
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (a *API) emailCodeHash(email, purpose, code string) string {
	mac := hmac.New(sha256.New, []byte(a.Cfg.Secret+"|folio-email-code"))
	_, _ = mac.Write([]byte(email + "|" + purpose + "|" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func generateEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func emailPurposeAction(purpose string) string {
	switch purpose {
	case emailPurposeRegister:
		return "注册账号"
	case emailPurposeReset:
		return "重置密码"
	case emailPurposeBind:
		return "绑定邮箱"
	case emailPurposeLogin:
		return "邮箱登录"
	default:
		return "身份验证"
	}
}

func emailMessage(purpose, code, site string) (subject, body string) {
	action := emailPurposeAction(purpose)
	subject = fmt.Sprintf("【%s】%s验证码", site, action)
	body = fmt.Sprintf(`你正在%s，验证码是：

%s

验证码 10 分钟内有效，请勿转发给他人。

如果不是你本人操作，请忽略这封邮件，你的账号不会发生变化。
`, action, code)
	return subject, body
}

// EmailStatus 告诉前端当前是否启用了邮件能力，以及注册是否需要验证码。
func (a *API) EmailStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":         a.emailEnabled(),
		"requireRegister": a.emailEnabled(),
		"captcha":         a.loginCaptchaEnabled(),
		"expiresIn":       int(emailCodeTTL.Seconds()),
	})
}

func (a *API) SendEmailCode(c *gin.Context) {
	if !a.emailEnabled() {
		fail(c, http.StatusServiceUnavailable, "邮件服务未启用，请先配置 SMTP")
		return
	}
	var in struct {
		Email       string `json:"email"`
		Purpose     string `json:"purpose"`
		CaptchaID   string `json:"captchaId"`
		CaptchaCode string `json:"captchaCode"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	email := normalizeEmail(in.Email)
	purpose := strings.ToLower(trim(in.Purpose))
	if email == "" || !strings.Contains(email, "@") || !validEmail(email) {
		bad(c, "邮箱格式不正确")
		return
	}
	switch purpose {
	case emailPurposeRegister, emailPurposeReset, emailPurposeBind, emailPurposeLogin:
	default:
		bad(c, "验证码用途不正确")
		return
	}
	if purpose == emailPurposeRegister && !a.allowRegistration() {
		fail(c, http.StatusForbidden, "站点暂未开放注册")
		return
	}
	if !a.requireCaptcha(c, in.CaptchaID, in.CaptchaCode) {
		return
	}

	user := currentUser(c)
	switch purpose {
	case emailPurposeRegister, emailPurposeBind:
		query := a.DB.Model(&models.User{}).Where("lower(email) = ?", email)
		if purpose == emailPurposeBind {
			if user == nil {
				fail(c, http.StatusUnauthorized, "请先登录")
				return
			}
			query = query.Where("id <> ?", user.ID)
		}
		var used int64
		if err := query.Count(&used).Error; err != nil {
			fail(c, http.StatusInternalServerError, "检查邮箱失败")
			return
		}
		if used > 0 {
			bad(c, "该邮箱已被使用")
			return
		}
	case emailPurposeReset, emailPurposeLogin:
		var users int64
		if err := a.DB.Model(&models.User{}).Where("lower(email) = ?", email).Count(&users).Error; err != nil {
			fail(c, http.StatusInternalServerError, "检查邮箱失败")
			return
		}
		if users == 0 {
			bad(c, "该邮箱还没有注册账号")
			return
		}
		if users > 1 {
			bad(c, "该邮箱关联了多个账号，请使用用户名登录")
			return
		}
	}

	now := time.Now()
	var recent models.EmailCode
	if err := a.DB.Where("email = ? AND purpose = ? AND created_at > ?", email, purpose, now.Add(-emailCodeCooldown)).
		Order("id DESC").First(&recent).Error; err == nil {
		fail(c, http.StatusTooManyRequests, "验证码发送太频繁，请稍后再试")
		return
	}
	var today int64
	if err := a.DB.Model(&models.EmailCode{}).
		Where("email = ? AND created_at > ?", email, now.Add(-24*time.Hour)).Count(&today).Error; err != nil {
		fail(c, http.StatusInternalServerError, "检查发送频率失败")
		return
	}
	if today >= emailCodeDailyLimit {
		fail(c, http.StatusTooManyRequests, "今天发送的验证码太多了，请明天再试")
		return
	}

	code, err := generateEmailCode()
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成验证码失败")
		return
	}
	record := models.EmailCode{
		Email:     email,
		Purpose:   purpose,
		CodeHash:  a.emailCodeHash(email, purpose, code),
		ExpiresAt: now.Add(emailCodeTTL),
		IP:        c.ClientIP(),
	}
	if err := a.DB.Create(&record).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存验证码失败")
		return
	}

	subject, body := emailMessage(purpose, code, a.Cfg.SiteName)
	if err := a.Mail.Send(c.Request.Context(), email, subject, body); err != nil {
		log.Printf("发送验证码邮件失败: %v", err)
		a.DB.Delete(&record)
		fail(c, http.StatusBadGateway, "邮件发送失败，请稍后重试")
		return
	}
	// 顺手清理两天前的验证码，避免表无限增长。
	a.DB.Where("created_at < ?", now.Add(-48*time.Hour)).Delete(&models.EmailCode{})

	result := gin.H{"ok": true, "expiresIn": int(emailCodeTTL.Seconds())}
	if a.Cfg.MailDebug {
		// 仅开发/自测模式返回，方便没有 SMTP 时验证流程。
		result["debugCode"] = code
	}
	c.JSON(http.StatusOK, result)
}

// consumeEmailCode 校验并消费验证码；同一邮箱同一用途的旧验证码会一起作废。
func (a *API) consumeEmailCode(email, purpose, code string) error {
	code = trim(code)
	if code == "" {
		return errors.New("请输入验证码")
	}
	var record models.EmailCode
	if err := a.DB.Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, purpose).
		Order("id DESC").First(&record).Error; err != nil {
		return errors.New("请先获取验证码")
	}
	if time.Now().After(record.ExpiresAt) {
		return errors.New("验证码已过期，请重新获取")
	}
	if record.Attempts >= emailCodeMaxAttempts {
		return errors.New("验证码尝试次数过多，请重新获取")
	}
	expected := a.emailCodeHash(email, purpose, code)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(record.CodeHash)) != 1 {
		a.DB.Model(&record).UpdateColumn("attempts", record.Attempts+1)
		return errors.New("验证码不正确")
	}
	now := time.Now()
	result := a.DB.Model(&models.EmailCode{}).
		Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, purpose).
		UpdateColumn("consumed_at", now)
	if result.Error != nil || result.RowsAffected == 0 {
		return errors.New("验证码已失效，请重新获取")
	}
	return nil
}

func (a *API) EmailLogin(c *gin.Context) {
	if !a.emailEnabled() {
		fail(c, http.StatusServiceUnavailable, "邮件服务未启用")
		return
	}
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	email := normalizeEmail(in.Email)
	if email == "" || !validEmail(email) {
		bad(c, "邮箱格式不正确")
		return
	}
	var u models.User
	if err := a.DB.Where("lower(email) = ?", email).First(&u).Error; err != nil {
		bad(c, "该邮箱还没有注册账号")
		return
	}
	if err := a.consumeEmailCode(email, emailPurposeLogin, in.Code); err != nil {
		bad(c, err.Error())
		return
	}
	a.DB.Model(&u).UpdateColumn("last_login_at", time.Now())
	a.recordLog(c, &u, "登录", "user", u.ID, "邮箱验证码登录")
	token, err := a.issueToken(c, &u)
	if err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": a.mePayload(&u)})
}

func (a *API) ResetPassword(c *gin.Context) {
	if !a.emailEnabled() {
		fail(c, http.StatusServiceUnavailable, "邮件服务未启用")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	email := normalizeEmail(in.Email)
	if email == "" || !validEmail(email) {
		bad(c, "邮箱格式不正确")
		return
	}
	if runeLen(in.Password) < 6 || runeLen(in.Password) > 64 {
		bad(c, "新密码长度需在 6-64 位之间")
		return
	}
	var u models.User
	if err := a.DB.Where("lower(email) = ?", email).First(&u).Error; err != nil {
		bad(c, "该邮箱还没有注册账号")
		return
	}
	if err := a.consumeEmailCode(email, emailPurposeReset, in.Code); err != nil {
		bad(c, err.Error())
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "重置密码失败")
		return
	}
	a.DB.Model(&u).UpdateColumn("password", hash)
	if err := a.revokeAllUserSessions(u.ID); err != nil {
		fail(c, http.StatusInternalServerError, "重置密码失败")
		return
	}
	a.Hub.disconnectAll(u.ID)
	a.recordLog(c, &u, "重置密码", "user", u.ID, "通过邮箱验证码重置")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) BindEmail(c *gin.Context) {
	if !a.emailEnabled() {
		fail(c, http.StatusServiceUnavailable, "邮件服务未启用")
		return
	}
	u := currentUser(c)
	if u == nil {
		fail(c, http.StatusUnauthorized, "请先登录")
		return
	}
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	email := normalizeEmail(in.Email)
	if email == "" || !validEmail(email) {
		bad(c, "邮箱格式不正确")
		return
	}
	var used int64
	if err := a.DB.Model(&models.User{}).Where("lower(email) = ? AND id <> ?", email, u.ID).Count(&used).Error; err != nil {
		fail(c, http.StatusInternalServerError, "检查邮箱失败")
		return
	}
	if used > 0 {
		bad(c, "该邮箱已被其他账号使用")
		return
	}
	if err := a.consumeEmailCode(email, emailPurposeBind, in.Code); err != nil {
		bad(c, err.Error())
		return
	}
	if err := a.DB.Model(u).UpdateColumn("email", email).Error; err != nil {
		fail(c, http.StatusInternalServerError, "绑定邮箱失败")
		return
	}
	a.DB.First(u, u.ID)
	a.recordLog(c, u, "绑定邮箱", "user", u.ID, "邮箱："+email)
	c.JSON(http.StatusOK, gin.H{"user": a.mePayload(u)})
}
