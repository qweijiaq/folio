package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", " ")
	return `"` + value + `"`
}

func exportFilename(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "untitled"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-", "\n", " ")
	title = replacer.Replace(title)
	runes := []rune(title)
	if len(runes) > 60 {
		title = string(runes[:60])
	}
	return strings.Trim(title, " .")
}

func (a *API) ExportContent(c *gin.Context) {
	var docs []models.Document
	query := a.DB.Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	})
	if hidden := a.hiddenDocIDs(currentUser(c)); len(hidden) > 0 {
		query = query.Where("id NOT IN ?", hidden)
	}
	query.Order("kind ASC, id ASC").Find(&docs)

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	manifest, err := writer.Create("README.txt")
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	_, _ = fmt.Fprintf(manifest, "知微内容导出\n生成时间：%s\n共 %d 篇内容。\n\n图片仍使用站内 /uploads 或对象存储地址，请一并备份。\n", time.Now().Format(time.RFC3339), len(docs))

	for _, doc := range docs {
		section := "articles"
		if doc.Kind == models.KindDoc {
			section = "docs"
		}
		name := fmt.Sprintf("%s/%d-%s.md", section, doc.ID, exportFilename(doc.Title))
		entry, err := writer.Create(name)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		tags, _ := json.Marshal(doc.Tags)
		category := ""
		if doc.Category != nil {
			category = doc.Category.Name
		}
		parent := "null"
		if doc.ParentID != nil {
			parent = fmt.Sprintf("%d", *doc.ParentID)
		}
		fmt.Fprintf(entry, "---\n")
		fmt.Fprintf(entry, "title: %s\n", yamlQuote(doc.Title))
		fmt.Fprintf(entry, "kind: %s\n", doc.Kind)
		fmt.Fprintf(entry, "status: %s\n", doc.Status)
		fmt.Fprintf(entry, "author: %s\n", yamlQuote(doc.Author.Nickname))
		if category != "" {
			fmt.Fprintf(entry, "category: %s\n", yamlQuote(category))
		}
		fmt.Fprintf(entry, "tags: %s\n", tags)
		fmt.Fprintf(entry, "created: %s\n", doc.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(entry, "updated: %s\n", doc.UpdatedAt.Format(time.RFC3339))
		fmt.Fprintf(entry, "parentID: %s\n", parent)
		fmt.Fprintf(entry, "---\n\n# %s\n\n%s\n", doc.Title, doc.Content)
	}
	if err := writer.Close(); err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	filename := fmt.Sprintf("folio-export-%s.zip", time.Now().Format("20060102-1504"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
