package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

var allowedAttachmentExts = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true, ".md": true, ".csv": true,
	".json": true, ".zip": true, ".rar": true, ".7z": true, ".tar": true,
	".gz": true, ".epub": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".mp3": true, ".m4a": true, ".wav": true,
	".mp4": true, ".webm": true,
}

func isHTTPURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func normalizeAttachments(items []models.Attachment) ([]models.Attachment, error) {
	if len(items) > 12 {
		return nil, fmt.Errorf("最多添加 12 个附件")
	}
	out := make([]models.Attachment, 0, len(items))
	for _, item := range items {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		if kind == "" {
			kind = "file"
		}
		if kind != "file" && kind != "link" {
			return nil, fmt.Errorf("附件类型不正确")
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			return nil, fmt.Errorf("附件名称不能为空")
		}
		if runeLen(title) > 120 {
			return nil, fmt.Errorf("附件名称最多 120 个字符")
		}
		url := strings.TrimSpace(item.URL)
		if kind == "link" {
			if !isHTTPURL(url) {
				return nil, fmt.Errorf("附件链接需要以 http:// 或 https:// 开头")
			}
			out = append(out, models.Attachment{Kind: kind, Title: title, URL: url})
			continue
		}
		if !strings.HasPrefix(url, "/uploads/") && !isHTTPURL(url) {
			return nil, fmt.Errorf("附件地址不正确")
		}
		size := item.Size
		if size < 0 {
			size = 0
		}
		out = append(out, models.Attachment{
			Kind:  kind,
			Title: title,
			URL:   url,
			Size:  size,
			MIME:  strings.TrimSpace(item.MIME),
		})
	}
	return out, nil
}

func sameAttachments(a, b []models.Attachment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AttachmentUpload 上传一个通用附件。文件和图片共用同一套存储，
// 但附件只把 URL 和展示信息随文档保存，不单独落一张附件表。
func (a *API) AttachmentUpload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		bad(c, "请选择附件文件")
		return
	}
	maxBytes := int64(a.Cfg.MaxUploadMB) * 1024 * 1024
	if fileHeader.Size <= 0 {
		bad(c, "附件文件为空")
		return
	}
	if fileHeader.Size > maxBytes {
		bad(c, fmt.Sprintf("附件不能超过 %dMB", a.Cfg.MaxUploadMB))
		return
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedAttachmentExts[ext] {
		bad(c, "不支持这种附件格式")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取附件失败")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "读取附件失败")
		return
	}
	if int64(len(data)) > maxBytes {
		bad(c, fmt.Sprintf("附件不能超过 %dMB", a.Cfg.MaxUploadMB))
		return
	}

	user := currentUser(c)
	if ok, retry := a.Limiter.Allow(userRateKey("attachment", user.ID), 20, 10*time.Minute); !ok {
		tooMany(c, retry)
		return
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	relPath := fmt.Sprintf("attachments/%s/%s%s", time.Now().Format("2006/01"), hash[:24], ext)
	mimeType := http.DetectContentType(data)
	if mimeType == "application/octet-stream" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			mimeType = byExt
		}
	}
	object, err := a.Storage.Put(c.Request.Context(), relPath, data, mimeType)
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	title := safeImageName(fileHeader.Filename, ext)
	a.logAction(c, "上传附件", "attachment", 0, title)
	c.JSON(http.StatusCreated, gin.H{
		"attachment": models.Attachment{
			Kind:  "file",
			Title: title,
			URL:   object.URL,
			Size:  int64(len(data)),
			MIME:  mimeType,
		},
	})
}
