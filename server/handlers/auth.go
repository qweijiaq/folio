package handlers

import (
	"net/http"
	"regexp"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,20}$`)

type userBrief struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Bio       string    `json:"bio"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func toBrief(u models.User) userBrief {
	return userBrief{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Bio:       u.Bio,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}

func (a *API) countsFor(u *models.User) gin.H {
	var articles, docs, favorites, followers, following int64
	a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ? AND status = ?", u.ID, models.KindArticle, models.StatusPublished).Count(&articles)
	a.DB.Model(&models.Document{}).Where("author_id = ? AND kind = ? AND status = ?", u.ID, models.KindDoc, models.StatusPublished).Count(&docs)
	a.DB.Model(&models.Favorite{}).Where("user_id = ?", u.ID).Count(&favorites)
	a.DB.Model(&models.Follow{}).Where("followee_id = ?", u.ID).Count(&followers)
	a.DB.Model(&models.Follow{}).Where("follower_id = ?", u.ID).Count(&following)
	return gin.H{
		"articles":  articles,
		"docs":      docs,
		"favorites": favorites,
		"followers": followers,
		"following": following,
	}
}

func (a *API) mePayload(u *models.User) gin.H {
	return gin.H{
		"id":        u.ID,
		"username":  u.Username,
		"nickname":  u.Nickname,
		"email":     u.Email,
		"bio":       u.Bio,
		"role":      u.Role,
		"createdAt": u.CreatedAt,
		"counts":    a.countsFor(u),
	}
}

func (a *API) Register(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Username = trim(in.Username)
	in.Nickname = trim(in.Nickname)
	in.Email = trim(in.Email)
	if !usernamePattern.MatchString(in.Username) {
		bad(c, "用户名需为 2-20 位字母、数字或下划线")
		return
	}
	if runeLen(in.Password) < 6 || runeLen(in.Password) > 64 {
		bad(c, "密码长度需在 6-64 位之间")
		return
	}
	if !validEmail(in.Email) {
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
	var exists int64
	a.DB.Model(&models.User{}).Where("lower(username) = lower(?)", in.Username).Count(&exists)
	if exists > 0 {
		bad(c, "用户名已被占用")
		return
	}
	var total int64
	a.DB.Model(&models.User{}).Count(&total)
	role := models.RoleUser
	if total == 0 {
		role = models.RoleSuperAdmin
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "注册失败")
		return
	}
	u := models.User{
		Username: in.Username,
		Nickname: in.Nickname,
		Email:    in.Email,
		Password: hash,
		Role:     role,
	}
	if err := a.DB.Create(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, "注册失败")
		return
	}
	token, err := a.issueToken(&u)
	if err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": a.mePayload(&u)})
}

func (a *API) Login(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	username := trim(in.Username)
	var u models.User
	if err := a.DB.Where("lower(username) = lower(?)", username).First(&u).Error; err != nil {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if !checkPassword(u.Password, in.Password) {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	now := time.Now()
	a.DB.Model(&u).UpdateColumn("last_login_at", now)
	token, err := a.issueToken(&u)
	if err != nil {
		fail(c, http.StatusInternalServerError, "登录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": a.mePayload(&u)})
}

func (a *API) Logout(c *gin.Context) {
	if claims := currentClaims(c); claims != nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		_ = a.Cache.Blacklist(c.Request.Context(), claims.ID, ttl)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": a.mePayload(currentUser(c))})
}

func (a *API) MeUpdate(c *gin.Context) {
	u := currentUser(c)
	var in struct {
		Nickname string `json:"nickname"`
		Bio      string `json:"bio"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Nickname = trim(in.Nickname)
	in.Bio = trim(in.Bio)
	in.Email = trim(in.Email)
	if runeLen(in.Nickname) < 1 || runeLen(in.Nickname) > 24 {
		bad(c, "昵称长度需在 1-24 个字符之间")
		return
	}
	if runeLen(in.Bio) > 120 {
		bad(c, "简介最多 120 个字符")
		return
	}
	if !validEmail(in.Email) {
		bad(c, "邮箱格式不正确")
		return
	}
	if err := a.DB.Model(u).Updates(map[string]any{
		"nickname": in.Nickname,
		"bio":      in.Bio,
		"email":    in.Email,
	}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	a.DB.First(u, u.ID)
	c.JSON(http.StatusOK, gin.H{"user": a.mePayload(u)})
}

func (a *API) MePassword(c *gin.Context) {
	u := currentUser(c)
	var in struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if !checkPassword(u.Password, in.OldPassword) {
		bad(c, "原密码不正确")
		return
	}
	if runeLen(in.NewPassword) < 6 || runeLen(in.NewPassword) > 64 {
		bad(c, "新密码长度需在 6-64 位之间")
		return
	}
	hash, err := hashPassword(in.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "修改失败")
		return
	}
	a.DB.Model(u).UpdateColumn("password", hash)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) UserSearch(c *gin.Context) {
	q := trim(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, gin.H{"items": []userBrief{}})
		return
	}
	like := likePattern(q)
	var users []models.User
	a.DB.Select("id", "username", "nickname", "bio", "role", "created_at").
		Where(`username LIKE ? ESCAPE '!' OR nickname LIKE ? ESCAPE '!'`, like, like).
		Order("id ASC").Limit(10).Find(&users)
	items := make([]userBrief, 0, len(users))
	for _, u := range users {
		items = append(items, toBrief(u))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) UserProfile(c *gin.Context) {
	username := c.Param("username")
	var u models.User
	if err := a.DB.Where("lower(username) = lower(?)", username).First(&u).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	viewer := currentUser(c)
	isSelf := viewer != nil && viewer.ID == u.ID
	isFollowing := false
	if viewer != nil && !isSelf {
		var n int64
		a.DB.Model(&models.Follow{}).Where("follower_id = ? AND followee_id = ?", viewer.ID, u.ID).Count(&n)
		isFollowing = n > 0
	}
	payload := gin.H{
		"user":        toBrief(u),
		"counts":      a.countsFor(&u),
		"isSelf":      isSelf,
		"isFollowing": isFollowing,
	}
	if isSelf || isSuperAdmin(viewer) {
		payload["email"] = u.Email
	}
	c.JSON(http.StatusOK, payload)
}

func (a *API) peopleList(c *gin.Context, ids []uint, total int64) {
	page, size := pageParams(c)
	start := (page - 1) * size
	if start > len(ids) {
		start = len(ids)
	}
	end := start + size
	if end > len(ids) {
		end = len(ids)
	}
	items := make([]gin.H, 0, end-start)
	viewer := currentUser(c)
	for _, id := range ids[start:end] {
		var u models.User
		if err := a.DB.Select("id", "username", "nickname", "bio", "role", "created_at").First(&u, id).Error; err != nil {
			continue
		}
		var docs int64
		a.DB.Model(&models.Document{}).Where("author_id = ? AND status = ?", u.ID, models.StatusPublished).Count(&docs)
		following := false
		if viewer != nil && viewer.ID != u.ID {
			var n int64
			a.DB.Model(&models.Follow{}).Where("follower_id = ? AND followee_id = ?", viewer.ID, u.ID).Count(&n)
			following = n > 0
		}
		items = append(items, gin.H{
			"user":        toBrief(u),
			"docCount":    docs,
			"isFollowing": following,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

func (a *API) Followers(c *gin.Context) {
	var u models.User
	if err := a.DB.Where("lower(username) = lower(?)", c.Param("username")).First(&u).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	var rows []models.Follow
	a.DB.Where("followee_id = ?", u.ID).Order("id DESC").Find(&rows)
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.FollowerID)
	}
	a.peopleList(c, ids, int64(len(ids)))
}

func (a *API) Following(c *gin.Context) {
	var u models.User
	if err := a.DB.Where("lower(username) = lower(?)", c.Param("username")).First(&u).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	var rows []models.Follow
	a.DB.Where("follower_id = ?", u.ID).Order("id DESC").Find(&rows)
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.FolloweeID)
	}
	a.peopleList(c, ids, int64(len(ids)))
}

func (a *API) FollowUser(c *gin.Context)   { a.follow(c, true) }
func (a *API) UnfollowUser(c *gin.Context) { a.follow(c, false) }

func (a *API) follow(c *gin.Context, on bool) {
	me := currentUser(c)
	var target models.User
	if err := a.DB.Where("lower(username) = lower(?)", c.Param("username")).First(&target).Error; err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if target.ID == me.ID {
		bad(c, "不能关注自己")
		return
	}
	if on {
		f := models.Follow{FollowerID: me.ID, FolloweeID: target.ID}
		a.DB.Where("follower_id = ? AND followee_id = ?", me.ID, target.ID).FirstOrCreate(&f)
		a.notify(target.ID, me.ID, models.NotifyFollow, nil, "")
	} else {
		a.DB.Where("follower_id = ? AND followee_id = ?", me.ID, target.ID).Delete(&models.Follow{})
		a.DB.Where("user_id = ? AND actor_id = ? AND type = ?", target.ID, me.ID, models.NotifyFollow).Delete(&models.Notification{})
	}
	var followers int64
	a.DB.Model(&models.Follow{}).Where("followee_id = ?", target.ID).Count(&followers)
	c.JSON(http.StatusOK, gin.H{"following": on, "followers": followers})
}
