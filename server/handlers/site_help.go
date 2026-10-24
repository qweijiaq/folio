package handlers

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed site_help.json
var siteHelpRaw []byte

type siteHelpCatalog struct {
	Version    string             `json:"version"`
	UpdatedAt  string             `json:"updatedAt"`
	Intro      string             `json:"intro"`
	Categories []siteHelpCategory `json:"categories"`
}

type siteHelpCategory struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Summary  string            `json:"summary"`
	Icon     string            `json:"icon"`
	Features []siteHelpFeature `json:"features"`
}

type siteHelpFeature struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Audience    string   `json:"audience"`
	Route       string   `json:"route"`
	Summary     string   `json:"summary"`
	Details     []string `json:"details"`
	Keywords    []string `json:"keywords"`
	Screenshots []string `json:"screenshots,omitempty"`
	Video       string   `json:"video,omitempty"`
}

type siteHelpEntry struct {
	Category siteHelpCategory
	Feature  siteHelpFeature
}

var siteHelpData siteHelpCatalog

func init() {
	if err := json.Unmarshal(siteHelpRaw, &siteHelpData); err != nil {
		log.Printf("站点帮助目录解析失败: %v", err)
	}
}

func (a *API) SiteFeatures(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"catalog": siteHelpData})
}

func siteHelpEntries(staff bool) []siteHelpEntry {
	entries := make([]siteHelpEntry, 0, 48)
	for _, category := range siteHelpData.Categories {
		for _, feature := range category.Features {
			if feature.Audience == "staff" && !staff {
				continue
			}
			entries = append(entries, siteHelpEntry{Category: category, Feature: feature})
		}
	}
	return entries
}
