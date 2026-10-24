package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"folio/server/art"
	"folio/server/models"

	"github.com/gin-gonic/gin"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Generator     string    `xml:"generator"`
	Image         *rssImage `xml:"image,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssImage struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

func (a *API) publicBaseURL(c *gin.Context) string {
	if a.Cfg.SiteURL != "" {
		return a.Cfg.SiteURL
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	host := c.Request.Host
	if forwarded := c.GetHeader("X-Forwarded-Host"); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return scheme + "://" + host
}

func (a *API) siteMeta() (name, description string) {
	setting := a.siteSetting()
	return settingText(setting.SiteName, a.Cfg.SiteName), settingText(setting.SiteDesc, a.Cfg.SiteDesc)
}

func (a *API) RSSFeed(c *gin.Context) {
	base := a.publicBaseURL(c)
	name, description := a.siteMeta()

	var docs []models.Document
	a.DB.Preload("Category").Where("kind = ? AND status = ?", models.KindArticle, models.StatusPublished).
		Order("COALESCE(published_at, created_at) DESC, id DESC").Limit(30).Find(&docs)

	items := make([]rssItem, 0, len(docs))
	for _, doc := range docs {
		published := doc.CreatedAt
		if doc.PublishedAt != nil {
			published = *doc.PublishedAt
		}
		link := fmt.Sprintf("%s/articles/%d", base, doc.ID)
		categories := make([]string, 0, len(doc.Tags)+1)
		if doc.Category != nil {
			categories = append(categories, doc.Category.Name)
		}
		categories = append(categories, doc.Tags...)
		items = append(items, rssItem{
			Title:       doc.Title,
			Link:        link,
			GUID:        link,
			Description: doc.Abstract,
			PubDate:     published.Format(time.RFC1123Z),
			Categories:  categories,
		})
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:         name,
			Link:          base + "/",
			Description:   description,
			Language:      "zh-CN",
			LastBuildDate: time.Now().Format(time.RFC1123Z),
			Generator:     "知微 Folio",
			Image: &rssImage{
				URL:   base + "/api/art/cover.png?seed=folio-site&kind=site",
				Title: name,
				Link:  base + "/",
			},
			Items: items,
		},
	}
	payload, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成 RSS 失败")
		return
	}
	c.Header("Cache-Control", "public, max-age=600")
	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), payload...))
}

func (a *API) Sitemap(c *gin.Context) {
	base := a.publicBaseURL(c)
	urls := []sitemapURL{
		{Loc: base + "/", ChangeFreq: "daily", Priority: "1.0"},
		{Loc: base + "/articles", ChangeFreq: "daily", Priority: "0.9"},
		{Loc: base + "/docs", ChangeFreq: "weekly", Priority: "0.8"},
		{Loc: base + "/about", ChangeFreq: "monthly", Priority: "0.6"},
	}

	var docs []models.Document
	docQuery := a.DB.Select("id", "kind", "updated_at").
		Where("status = ?", models.StatusPublished)
	if hidden := a.hiddenDocIDs(nil); len(hidden) > 0 {
		docQuery = docQuery.Where("id NOT IN ?", hidden)
	}
	docQuery.Order("updated_at DESC").Limit(50000).Find(&docs)
	for _, doc := range docs {
		section := "articles"
		priority := "0.8"
		if doc.Kind == models.KindDoc {
			section = "docs"
			priority = "0.7"
		}
		urls = append(urls, sitemapURL{
			Loc:        fmt.Sprintf("%s/%s/%d", base, section, doc.ID),
			LastMod:    doc.UpdatedAt.Format("2006-01-02"),
			ChangeFreq: "monthly",
			Priority:   priority,
		})
	}

	var users []models.User
	a.DB.Select("username", "updated_at").Find(&users)
	for _, user := range users {
		urls = append(urls, sitemapURL{
			Loc:        base + "/u/" + url.PathEscape(user.Username),
			LastMod:    user.UpdatedAt.Format("2006-01-02"),
			ChangeFreq: "weekly",
			Priority:   "0.4",
		})
	}

	set := sitemapURLSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}
	payload, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成 sitemap 失败")
		return
	}
	c.Header("Cache-Control", "public, max-age=600")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), payload...))
}

func (a *API) Robots(c *gin.Context) {
	base := a.publicBaseURL(c)
	body := strings.Join([]string{
		"User-agent: *",
		"Allow: /",
		"Allow: /api/art/",
		"Disallow: /api/",
		"Disallow: /admin",
		"Disallow: /me",
		"Disallow: /messages",
		"Disallow: /write",
		"Disallow: /login",
		"Disallow: /register",
		"Disallow: /forgot",
		"Disallow: /search",
		"Disallow: /ask",
		"",
		"Sitemap: " + base + "/sitemap.xml",
		"",
	}, "\n")
	c.Header("Cache-Control", "public, max-age=600")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body))
}

func (a *API) CoverPNG(c *gin.Context) {
	seed := c.Query("seed")
	if seed == "" {
		seed = c.Query("title")
	}
	if seed == "" {
		seed = "folio"
	}
	if runeLen(seed) > 120 {
		seed = string([]rune(seed)[:120])
	}
	data, err := art.CoverPNG(seed, c.Query("kind"))
	if err != nil {
		fail(c, http.StatusInternalServerError, "生成分享图失败")
		return
	}
	c.Header("Cache-Control", "public, max-age=604800")
	c.Data(http.StatusOK, "image/png", data)
}

type pageMeta struct {
	Title       string
	Description string
	Image       string
	Type        string
	URL         string
	Published   string
	Modified    string
	Tags        []string
	JSONLD      any
	NoIndex     bool
}

func (a *API) pageMetaFor(c *gin.Context, path string) pageMeta {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	base := a.publicBaseURL(c)
	name, description := a.siteMeta()
	image := base + "/api/art/cover.png?seed=folio-site&kind=site"
	meta := pageMeta{
		Title:       "",
		Description: description,
		Image:       image,
		Type:        "website",
		URL:         base + path,
	}

	switch {
	case path == "/" || path == "":
		meta.Title = name
	case path == "/articles" || path == "/articles/":
		meta.Title = "文章"
		meta.Description = "较长的思考、读书笔记，以及一些还没想清楚的问题。"
		meta.Image = base + "/api/art/cover.png?seed=articles&kind=site"
	case path == "/docs" || path == "/docs/":
		meta.Title = "文档"
		meta.Description = "按主题整理的长期笔记：手册、速查、清单。"
		meta.Image = base + "/api/art/cover.png?seed=docs&kind=site"
	case path == "/about":
		meta.Title = "关于"
		meta.Description = "系统了解知微的功能：文章、知识库、写作、评论、私信、AI 问答、管理与存储。"
		meta.Image = base + "/api/art/cover.png?seed=about&kind=site"
		meta.JSONLD = map[string]any{
			"@context":    "https://schema.org",
			"@type":       "AboutPage",
			"name":        "关于" + name,
			"description": meta.Description,
			"url":         base + "/about",
		}
	case path == "/search" || strings.HasPrefix(path, "/search/"):
		meta.Title = "搜索"
		meta.NoIndex = true
	case strings.HasSuffix(path, "/edit"):
		meta.Title = "编辑"
		meta.NoIndex = true
	case strings.HasPrefix(path, "/articles/"):
		id, err := strconv.ParseUint(strings.TrimPrefix(path, "/articles/"), 10, 64)
		if err == nil {
			var doc models.Document
			if err := a.DB.Preload("Category").Preload("Author").Where("id = ? AND kind = ? AND status = ?", id, models.KindArticle, models.StatusPublished).First(&doc).Error; err == nil {
				a.fillDocumentMeta(&meta, base, &doc)
			}
		}
	case strings.HasPrefix(path, "/docs/"):
		id, err := strconv.ParseUint(strings.TrimPrefix(path, "/docs/"), 10, 64)
		if err == nil {
			var doc models.Document
			if err := a.DB.Preload("Category").Preload("Author").Where("id = ? AND kind = ? AND status = ?", id, models.KindDoc, models.StatusPublished).First(&doc).Error; err == nil {
				if a.docIsPrivate(doc.ID) {
					meta.Title = "文档"
					meta.NoIndex = true
				} else {
					a.fillDocumentMeta(&meta, base, &doc)
				}
			}
		}
	case strings.HasPrefix(path, "/u/"):
		username := strings.TrimPrefix(path, "/u/")
		var user models.User
		if err := a.DB.Where("lower(username) = lower(?)", username).First(&user).Error; err == nil {
			meta.Title = user.Nickname
			meta.Description = user.Bio
			if meta.Description == "" {
				meta.Description = name + " 的读者主页"
			}
			meta.Type = "profile"
			meta.Image = base + "/api/art/cover.png?seed=" + url.QueryEscape(user.Username) + "&kind=user"
			meta.URL = base + "/u/" + url.PathEscape(user.Username)
			meta.JSONLD = map[string]any{
				"@context": "https://schema.org",
				"@type":    "ProfilePage",
				"mainEntity": map[string]any{
					"@type":         "Person",
					"name":          user.Nickname,
					"alternateName": user.Username,
					"description":   user.Bio,
					"url":           meta.URL,
				},
			}
		}
	case strings.HasPrefix(path, "/ask"):
		meta.Title = "问 AI"
		meta.NoIndex = true
	case strings.HasPrefix(path, "/login"), strings.HasPrefix(path, "/register"), strings.HasPrefix(path, "/forgot"),
		strings.HasPrefix(path, "/me"), strings.HasPrefix(path, "/write"), strings.HasPrefix(path, "/messages"), strings.HasPrefix(path, "/admin"):
		meta.NoIndex = true
	}
	return meta
}

func (a *API) fillDocumentMeta(meta *pageMeta, base string, doc *models.Document) {
	meta.Title = doc.Title
	meta.Description = doc.Abstract
	if strings.TrimSpace(meta.Description) == "" {
		meta.Description = plainText(doc.Content)
	}
	meta.Description = clipText(meta.Description, 160)
	section := "articles"
	articleType := "BlogPosting"
	if doc.Kind == models.KindDoc {
		section = "docs"
		articleType = "TechArticle"
	}
	meta.Type = "article"
	meta.URL = fmt.Sprintf("%s/%s/%d", base, section, doc.ID)
	meta.Image = fmt.Sprintf("%s/api/art/cover.png?seed=%d&kind=%s", base, doc.ID, doc.Kind)
	published := doc.CreatedAt
	if doc.PublishedAt != nil {
		published = *doc.PublishedAt
	}
	meta.Published = published.Format(time.RFC3339)
	meta.Modified = doc.UpdatedAt.Format(time.RFC3339)
	meta.Tags = doc.Tags
	meta.JSONLD = map[string]any{
		"@context":         "https://schema.org",
		"@type":            articleType,
		"headline":         doc.Title,
		"description":      meta.Description,
		"datePublished":    meta.Published,
		"dateModified":     meta.Modified,
		"mainEntityOfPage": meta.URL,
		"image":            meta.Image,
		"author": map[string]any{
			"@type": "Person",
			"name":  doc.Author.Nickname,
			"url":   base + "/u/" + url.PathEscape(doc.Author.Username),
		},
	}
	if doc.Category != nil {
		meta.JSONLD.(map[string]any)["articleSection"] = doc.Category.Name
	}
	if len(doc.Tags) > 0 {
		meta.JSONLD.(map[string]any)["keywords"] = strings.Join(doc.Tags, ",")
	}
}

func (m pageMeta) tagsHTML(name string) string {
	title := m.Title
	if title == "" {
		title = name
	} else if title != name {
		title = title + " · " + name
	}
	var b strings.Builder
	writeMeta := func(attr, key, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, `<meta %s="%s" content="%s">`+"\n", attr, key, html.EscapeString(value))
	}
	writeMeta("name", "description", m.Description)
	if m.NoIndex {
		writeMeta("name", "robots", "noindex, nofollow")
	} else {
		writeMeta("name", "robots", "index, follow")
	}
	writeMeta("property", "og:site_name", name)
	writeMeta("property", "og:title", title)
	writeMeta("property", "og:description", m.Description)
	writeMeta("property", "og:type", m.Type)
	writeMeta("property", "og:url", m.URL)
	writeMeta("property", "og:image", m.Image)
	writeMeta("property", "og:image:width", "1200")
	writeMeta("property", "og:image:height", "630")
	writeMeta("property", "og:locale", "zh_CN")
	writeMeta("name", "twitter:card", "summary_large_image")
	writeMeta("name", "twitter:title", title)
	writeMeta("name", "twitter:description", m.Description)
	writeMeta("name", "twitter:image", m.Image)
	if m.Published != "" {
		writeMeta("property", "article:published_time", m.Published)
	}
	if m.Modified != "" {
		writeMeta("property", "article:modified_time", m.Modified)
	}
	for _, tag := range m.Tags {
		writeMeta("property", "article:tag", tag)
	}
	if m.URL != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`+"\n", html.EscapeString(m.URL))
	}
	if m.JSONLD != nil {
		if data, err := json.Marshal(m.JSONLD); err == nil {
			safe := strings.ReplaceAll(string(data), "</", "<\\/")
			fmt.Fprintf(&b, `<script id="folio-page-jsonld" type="application/ld+json">%s</script>`+"\n", safe)
		}
	}
	return b.String()
}

func injectPageMeta(document, tags, title string) string {
	if start := strings.Index(document, "<title>"); start >= 0 {
		if end := strings.Index(document[start:], "</title>"); end >= 0 {
			document = document[:start] + "<title>" + html.EscapeString(title) + "</title>" + document[start+end+len("</title>"):]
		}
	}
	const marker = "<!--folio:meta-->"
	if strings.Contains(document, marker) {
		return strings.Replace(document, marker, tags, 1)
	}
	if idx := strings.Index(document, "</head>"); idx >= 0 {
		return document[:idx] + tags + document[idx:]
	}
	return document
}

// ServeIndex 返回带服务端 SEO 元信息的 index.html，爬虫不执行 JS 也能读到标题、
// 摘要、分享图和 canonical。
func (a *API) ServeIndex(c *gin.Context, dist string) {
	raw, err := os.ReadFile(filepath.Join(dist, "index.html"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	path := c.Request.URL.Path
	meta := a.pageMetaFor(c, path)
	name, _ := a.siteMeta()
	title := meta.Title
	if title == "" {
		title = name
	} else if title != name {
		title = title + " · " + name
	}
	page := injectPageMeta(string(raw), meta.tagsHTML(name), title)
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}
