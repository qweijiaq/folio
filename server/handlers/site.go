package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type tagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (a *API) tagStats(limit int, kind string) []tagCount {
	var raw []string
	q := a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished)
	if kind == models.KindArticle || kind == models.KindDoc {
		q = q.Where("kind = ?", kind)
	}
	q.Pluck("tags", &raw)
	counts := map[string]int{}
	for _, s := range raw {
		var tags []string
		if err := json.Unmarshal([]byte(s), &tags); err != nil {
			continue
		}
		for _, t := range tags {
			counts[t]++
		}
	}
	items := make([]tagCount, 0, len(counts))
	for k, v := range counts {
		items = append(items, tagCount{Name: k, Count: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (a *API) Tags(c *gin.Context) {
	kind := strings.ToLower(strings.TrimSpace(c.Query("kind")))
	if kind != models.KindArticle && kind != models.KindDoc {
		kind = ""
	}
	c.JSON(http.StatusOK, gin.H{"items": a.tagStats(60, kind)})
}

func (a *API) SiteHome(c *gin.Context) {
	var articles, docs, users int64
	a.DB.Model(&models.Document{}).Where("status = ? AND kind = ?", models.StatusPublished, models.KindArticle).Count(&articles)
	a.DB.Model(&models.Document{}).Where("status = ? AND kind = ?", models.StatusPublished, models.KindDoc).Count(&docs)
	a.DB.Model(&models.User{}).Count(&users)
	var views int64
	a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished).
		Select("COALESCE(SUM(view_count), 0)").Scan(&views)

	authorSelect := func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}

	var featured models.Document
	a.DB.Preload("Category").Preload("Author", authorSelect).
		Where("status = ? AND kind = ?", models.StatusPublished, models.KindArticle).
		Order("pinned DESC, COALESCE(published_at, created_at) DESC, id DESC").First(&featured)

	var recent []models.Document
	a.DB.Preload("Category").Preload("Author", authorSelect).
		Where("status = ? AND kind = ?", models.StatusPublished, models.KindArticle).
		Order("COALESCE(published_at, created_at) DESC, id DESC").Limit(6).Find(&recent)

	var recentDocs []models.Document
	a.DB.Preload("Category").Preload("Author", authorSelect).
		Where("status = ? AND kind = ?", models.StatusPublished, models.KindDoc).
		Order("view_count DESC, id DESC").Limit(6).Find(&recentDocs)
	blankContent(recent)
	blankContent(recentDocs)

	var latestUsers []models.User
	a.DB.Select("id", "username", "nickname", "bio", "role", "created_at").
		Order("id DESC").Limit(5).Find(&latestUsers)
	people := make([]userBrief, 0, len(latestUsers))
	for _, u := range latestUsers {
		people = append(people, toBrief(u))
	}

	payload := gin.H{
		"site":   a.siteInfo(),
		"stats":  gin.H{"articles": articles, "docs": docs, "users": users, "views": views},
		"recent": recent,
		"docs":   recentDocs,
		"tags":   a.tagStats(16, ""),
		"people": people,
	}
	if featured.ID != 0 {
		featured.Content = ""
		payload["featured"] = featured
	} else {
		payload["featured"] = nil
	}
	c.JSON(http.StatusOK, payload)
}
