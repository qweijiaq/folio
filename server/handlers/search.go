package handlers

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type searchScoredDoc struct {
	doc   models.Document
	score float64
}

func searchPlainText(md string) string {
	s := reFence.ReplaceAllString(md, " ")
	s = reImage.ReplaceAllString(s, " ")
	s = reLink.ReplaceAllString(s, "$1")
	s = reHTML.ReplaceAllString(s, " ")
	s = reMarks.ReplaceAllString(s, " ")
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

func searchSnippet(content string, terms []string, limit int) string {
	text := searchPlainText(content)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	index := -1
	for _, candidate := range terms {
		if pos := strings.Index(lower, candidate); pos >= 0 && (index < 0 || pos < index) {
			index = pos
		}
	}
	if index < 0 {
		return truncateRunes(text, limit)
	}
	runeIndex := utf8.RuneCountInString(text[:index])
	runes := []rune(text)
	start := runeIndex - 36
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(runes) {
		end = len(runes)
	}
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet
}

func searchTermScore(text string, terms []string, weight float64) float64 {
	if text == "" {
		return 0
	}
	lower := strings.ToLower(text)
	score := 0.0
	for _, term := range terms {
		count := 0
		for offset := 0; offset < len(lower); {
			idx := strings.Index(lower[offset:], term)
			if idx < 0 {
				break
			}
			// ASCII 词做边界判断，避免 go 命中 good 这类情况。
			ascii := true
			for _, r := range term {
				if r < 'a' || r > 'z' {
					ascii = false
					break
				}
			}
			start := offset + idx
			end := start + len(term)
			if !ascii || (start == 0 || lower[start-1] < 'a' || lower[start-1] > 'z') &&
				(end == len(lower) || lower[end] < 'a' || lower[end] > 'z') {
				count++
			}
			offset = start + len(term)
		}
		if count > 0 {
			score += weight * (1 + math.Log(float64(count)))
		}
	}
	return score
}

func searchTagsFrom(docs []models.Document, limit int) []tagCount {
	counts := map[string]int{}
	for _, doc := range docs {
		for _, tag := range doc.Tags {
			counts[tag]++
		}
	}
	items := make([]tagCount, 0, len(counts))
	for name, count := range counts {
		items = append(items, tagCount{Name: name, Count: count})
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

// Search 对文章和文档做加权检索，返回相关度排序、摘要片段和标签建议。
func (a *API) Search(c *gin.Context) {
	keyword := trim(c.Query("q"))
	tag := trim(c.Query("tag"))
	kind := strings.ToLower(trim(c.Query("kind")))
	if kind != models.KindArticle && kind != models.KindDoc {
		kind = ""
	}
	page, size := pageParams(c)
	if size > 30 {
		size = 30
	}
	if keyword == "" && tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"items":    []models.Document{},
			"total":    0,
			"counts":   gin.H{"all": 0, "article": 0, "doc": 0},
			"page":     page,
			"pageSize": size,
			"tags":     []tagCount{},
		})
		return
	}
	terms := qaTerms(keyword)
	if len(terms) == 0 && keyword != "" {
		terms = []string{strings.ToLower(keyword)}
	}
	if len(terms) > 12 {
		terms = terms[:12]
	}
	viewer := currentUser(c)
	hidden := a.hiddenDocIDs(viewer)

	build := func(withKind string) *gorm.DB {
		query := a.DB.Model(&models.Document{}).Where("status = ?", models.StatusPublished)
		if len(hidden) > 0 {
			query = query.Where("id NOT IN ?", hidden)
		}
		if tag != "" {
			query = query.Where(`tags LIKE ? ESCAPE '!'`, tagPattern(tag))
		}
		if len(terms) > 0 {
			parts := make([]string, 0, len(terms))
			args := make([]any, 0, len(terms)*4)
			for _, term := range terms {
				like := likePattern(term)
				parts = append(parts, `(title LIKE ? ESCAPE '!' OR abstract LIKE ? ESCAPE '!' OR content LIKE ? ESCAPE '!' OR tags LIKE ? ESCAPE '!')`)
				args = append(args, like, like, like, like)
			}
			query = query.Where(strings.Join(parts, " OR "), args...)
		}
		if withKind != "" {
			query = query.Where("kind = ?", withKind)
		}
		return query
	}

	var articleCount, docCount int64
	build(models.KindArticle).Count(&articleCount)
	build(models.KindDoc).Count(&docCount)

	var candidates []models.Document
	build(kind).Preload("Category").Preload("Author", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "username", "nickname", "bio", "role", "created_at")
	}).Order("COALESCE(published_at, created_at) DESC, id DESC").Limit(1200).Find(&candidates)

	suggestions := searchTagsFrom(candidates, 10)
	phrase := qaNormalize(keyword)
	scored := make([]searchScoredDoc, 0, len(candidates))
	for _, doc := range candidates {
		score := 0.0
		if keyword != "" {
			score += searchTermScore(doc.Title, terms, 10)
			score += searchTermScore(strings.Join(doc.Tags, " "), terms, 7)
			score += searchTermScore(doc.Abstract, terms, 4)
			score += searchTermScore(doc.Content, terms, 1)
			if phrase != "" {
				if strings.Contains(qaNormalize(doc.Title), phrase) {
					score += 32
				}
				if strings.Contains(qaNormalize(strings.Join(doc.Tags, " ")), phrase) {
					score += 16
				}
				if strings.Contains(qaNormalize(doc.Abstract), phrase) {
					score += 8
				}
			}
		}
		if keyword != "" {
			if snippet := searchSnippet(doc.Content+"\n\n"+doc.Abstract, terms, 150); snippet != "" {
				doc.Abstract = snippet
			}
		}
		doc.Content = ""
		scored = append(scored, searchScoredDoc{doc: doc, score: score})
	}
	if keyword != "" {
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score == scored[j].score {
				return scored[i].doc.ID > scored[j].doc.ID
			}
			return scored[i].score > scored[j].score
		})
	}
	total := articleCount + docCount
	if kind != "" {
		if kind == models.KindArticle {
			total = articleCount
		} else {
			total = docCount
		}
	}
	start := (page - 1) * size
	if start > len(scored) {
		start = len(scored)
	}
	end := start + size
	if end > len(scored) {
		end = len(scored)
	}
	items := make([]models.Document, 0, end-start)
	for _, item := range scored[start:end] {
		items = append(items, item.doc)
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"total":    total,
		"counts":   gin.H{"all": articleCount + docCount, "article": articleCount, "doc": docCount},
		"page":     page,
		"pageSize": size,
		"tags":     suggestions,
	})
}
