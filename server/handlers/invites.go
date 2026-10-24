package handlers

import (
	"errors"
	"net/http"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

func (a *API) findValidInvite(code string) (*models.InviteCode, error) {
	var invite models.InviteCode
	if err := a.DB.Where("lower(code) = lower(?)", trim(code)).First(&invite).Error; err != nil {
		return nil, errors.New("邀请码无效")
	}
	if invite.UsedAt != nil {
		return nil, errors.New("邀请码已被使用")
	}
	if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
		return nil, errors.New("邀请码已过期")
	}
	return &invite, nil
}

func inviteStatus(invite models.InviteCode) string {
	if invite.UsedAt != nil {
		return "used"
	}
	if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
		return "expired"
	}
	return "active"
}

func (a *API) AdminInvites(c *gin.Context) {
	var invites []models.InviteCode
	a.DB.Order("id DESC").Limit(100).Find(&invites)
	items := make([]gin.H, 0, len(invites))
	for _, invite := range invites {
		items = append(items, gin.H{
			"id":            invite.ID,
			"code":          invite.Code,
			"note":          invite.Note,
			"createdBy":     invite.CreatedBy,
			"createdByName": invite.CreatedByName,
			"expiresAt":     invite.ExpiresAt,
			"usedBy":        invite.UsedBy,
			"usedAt":        invite.UsedAt,
			"status":        inviteStatus(invite),
			"createdAt":     invite.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) AdminInviteCreate(c *gin.Context) {
	var in struct {
		Note          string `json:"note"`
		ExpiresInDays int    `json:"expiresInDays"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Note = trim(in.Note)
	if runeLen(in.Note) > 100 {
		bad(c, "备注最多 100 个字符")
		return
	}
	if in.ExpiresInDays <= 0 {
		in.ExpiresInDays = 7
	}
	if in.ExpiresInDays > 90 {
		bad(c, "邀请码有效期最多 90 天")
		return
	}
	now := time.Now()
	expiresAt := now.AddDate(0, 0, in.ExpiresInDays)
	me := currentUser(c)
	code := ""
	for attempt := 0; attempt < 20; attempt++ {
		candidate := randomTokenID()[:16]
		var count int64
		a.DB.Model(&models.InviteCode{}).Where("code = ?", candidate).Count(&count)
		if count == 0 {
			code = candidate
			break
		}
	}
	if code == "" {
		fail(c, http.StatusInternalServerError, "生成邀请码失败")
		return
	}
	invite := models.InviteCode{
		Code:          code,
		Note:          in.Note,
		CreatedBy:     me.ID,
		CreatedByName: me.Nickname,
		ExpiresAt:     &expiresAt,
	}
	if err := a.DB.Create(&invite).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建邀请码失败")
		return
	}
	a.logAction(c, "创建邀请码", "invite", invite.ID, invite.Note)
	c.JSON(http.StatusCreated, gin.H{"invite": invite})
}

func (a *API) AdminInviteDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var invite models.InviteCode
	if err := a.DB.First(&invite, id).Error; err != nil {
		fail(c, http.StatusNotFound, "邀请码不存在")
		return
	}
	if invite.UsedAt != nil {
		bad(c, "已使用的邀请码不能删除")
		return
	}
	if err := a.DB.Delete(&invite).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除邀请码失败")
		return
	}
	a.logAction(c, "删除邀请码", "invite", invite.ID, invite.Note)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) AdminCreateUser(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Username = trim(in.Username)
	in.Nickname = trim(in.Nickname)
	in.Email = normalizeEmail(in.Email)
	in.Role = trim(in.Role)
	if !usernamePattern.MatchString(in.Username) {
		bad(c, "用户名需为 2-20 位字母、数字或下划线")
		return
	}
	if runeLen(in.Password) < 6 || runeLen(in.Password) > 64 {
		bad(c, "密码长度需在 6-64 位之间")
		return
	}
	if in.Email != "" && !validEmail(in.Email) {
		bad(c, "邮箱格式不正确")
		return
	}
	if in.Nickname == "" {
		in.Nickname = in.Username
	}
	if runeLen(in.Nickname) > 24 {
		bad(c, "昵称最多 24 个字符")
		return
	}
	if in.Role == "" {
		in.Role = models.RoleUser
	}
	if in.Role != models.RoleUser && in.Role != models.RoleAdmin {
		bad(c, "角色只能是 user 或 admin")
		return
	}
	var exists int64
	a.DB.Model(&models.User{}).Where("lower(username) = lower(?)", in.Username).Count(&exists)
	if exists > 0 {
		bad(c, "用户名已被占用")
		return
	}
	if in.Email != "" {
		a.DB.Model(&models.User{}).Where("lower(email) = ?", in.Email).Count(&exists)
		if exists > 0 {
			bad(c, "该邮箱已被使用")
			return
		}
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "创建用户失败")
		return
	}
	user := models.User{
		Username: in.Username,
		Nickname: in.Nickname,
		Email:    in.Email,
		Password: hash,
		Role:     in.Role,
	}
	if err := a.DB.Create(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建用户失败")
		return
	}
	_, _ = a.ensureDefaultFolder(user.ID)
	a.logAction(c, "创建用户", "user", user.ID, user.Username+"（"+user.Role+"）")
	c.JSON(http.StatusCreated, gin.H{"user": a.mePayload(&user)})
}
