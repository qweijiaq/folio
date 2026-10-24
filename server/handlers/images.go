package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func imageURL(path string) string {
	return "/uploads/" + strings.TrimLeft(filepath.ToSlash(path), "/")
}

func (a *API) imagePayload(img models.Image) gin.H {
	return gin.H{
		"id":        img.ID,
		"url":       imageURL(img.Path),
		"filename":  img.Filename,
		"mime":      img.MIME,
		"size":      img.Size,
		"createdAt": img.CreatedAt,
		"user":      toBrief(img.User),
	}
}

func safeImageName(name, ext string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		name = "image" + ext
	}
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	return name
}

func (a *API) ImageUpload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		fileHeader, err = c.FormFile("image")
	}
	if err != nil {
		bad(c, "请选择图片文件")
		return
	}
	maxBytes := int64(a.Cfg.MaxUploadMB) * 1024 * 1024
	if fileHeader.Size <= 0 {
		bad(c, "图片文件为空")
		return
	}
	if fileHeader.Size > maxBytes {
		bad(c, fmt.Sprintf("图片不能超过 %dMB", a.Cfg.MaxUploadMB))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取图片失败")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "读取图片失败")
		return
	}
	if int64(len(data)) > maxBytes {
		bad(c, fmt.Sprintf("图片不能超过 %dMB", a.Cfg.MaxUploadMB))
		return
	}
	mimeType := http.DetectContentType(data)
	ext, ok := allowedImageTypes[mimeType]
	if !ok {
		bad(c, "只支持 JPG、PNG、GIF、WebP 图片")
		return
	}

	user := currentUser(c)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	var existing models.Image
	if err := a.DB.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Where("user_id = ? AND hash = ?", user.ID, hash).First(&existing).Error; err == nil {
		if existing.User.ID == 0 {
			existing.User = *user
		}
		c.JSON(http.StatusOK, gin.H{"image": a.imagePayload(existing), "url": imageURL(existing.Path)})
		return
	}

	relPath := filepath.Join(time.Now().Format("2006/01"), hash[:24]+ext)
	fullPath := filepath.Join(a.Cfg.UploadDir, relPath)
	if _, err := os.Stat(fullPath); err != nil {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			fail(c, http.StatusInternalServerError, "创建图片目录失败")
			return
		}
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			fail(c, http.StatusInternalServerError, "保存图片失败")
			return
		}
	}

	img := models.Image{
		UserID:   user.ID,
		Filename: safeImageName(fileHeader.Filename, ext),
		Path:     filepath.ToSlash(relPath),
		MIME:     mimeType,
		Size:     int64(len(data)),
		Hash:     hash,
	}
	if err := a.DB.Create(&img).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存图片记录失败")
		return
	}
	img.User = *user
	a.logAction(c, "上传图片", "image", img.ID, img.Filename)
	c.JSON(http.StatusCreated, gin.H{"image": a.imagePayload(img), "url": imageURL(img.Path)})
}

func (a *API) ImageList(c *gin.Context) {
	viewer := currentUser(c)
	page, size := pageParams(c)
	q := a.DB.Model(&models.Image{})
	if !isSuperAdmin(viewer) {
		q = q.Where("user_id = ?", viewer.ID)
	}
	if keyword := trim(c.Query("q")); keyword != "" {
		q = q.Where(`filename LIKE ? ESCAPE '!'`, likePattern(keyword))
	}
	var total int64
	q.Count(&total)
	var images []models.Image
	q.Preload("User", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&images)
	items := make([]gin.H, 0, len(images))
	for _, img := range images {
		items = append(items, a.imagePayload(img))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": size})
}

func (a *API) removeImageFileIfUnused(path string) {
	var count int64
	a.DB.Model(&models.Image{}).Where("path = ?", path).Count(&count)
	if count > 0 {
		return
	}
	dir, err := filepath.Abs(a.Cfg.UploadDir)
	if err != nil {
		return
	}
	full, err := filepath.Abs(filepath.Join(a.Cfg.UploadDir, filepath.FromSlash(path)))
	if err != nil {
		return
	}
	rel, err := filepath.Rel(dir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	_ = os.Remove(full)
}

func (a *API) ImageDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var img models.Image
	if err := a.DB.First(&img, id).Error; err != nil {
		fail(c, http.StatusNotFound, "图片不存在")
		return
	}
	u := currentUser(c)
	if img.UserID != u.ID && !isSuperAdmin(u) {
		fail(c, http.StatusForbidden, "没有删除权限")
		return
	}
	path := img.Path
	if err := a.DB.Delete(&img).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	a.removeImageFileIfUnused(path)
	a.logAction(c, "删除图片", "image", img.ID, img.Filename)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
