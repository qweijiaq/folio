package handlers

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"folio/server/models"
)

var qaStopWords = map[string]bool{
	"什么": true, "怎么": true, "如何": true, "可以": true, "一个": true,
	"这个": true, "那个": true, "哪些": true, "为什么": true, "请问": true,
	"介绍": true, "一下": true, "是否": true, "有没有": true, "以及": true,
	"现在": true, "这里": true, "就是": true, "还是": true, "关于": true,
}

var qaSynonyms = map[string][]string{
	"登录": {"登陆", "账号", "密码"},
	"注册": {"账号", "邀请"},
	"邀请": {"邀请码", "注册"},
	"私信": {"消息", "聊天"},
	"消息": {"私信", "通知"},
	"收藏": {"收藏夹", "书签"},
	"封面": {"cover", "图片"},
	"存储": {"七牛", "对象存储", "图片"},
	"七牛": {"对象存储", "qiniu", "存储"},
	"部署": {"docker", "容器", "运行"},
	"导出": {"备份", "markdown", "zip"},
	"版本": {"历史", "恢复", "回滚"},
	"通知": {"消息", "提醒"},
	"评论": {"回复", "留言"},
	"主题": {"暗色", "明暗", "theme"},
	"问答": {"ai", "知识库", "提问"},
}

type qaSource struct {
	Ref      string   `json:"ref"`
	ID       uint     `json:"id,omitempty"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Excerpt  string   `json:"excerpt"`
	Context  string   `json:"-"`
}

type qaChunk struct {
	Ref      string
	ID       uint
	Kind     string
	Title    string
	URL      string
	Category string
	Tags     []string
	Abstract string
	Text     string
	Keywords string
	Score    float64
}

func qaTerms(question string) []string {
	lower := strings.ToLower(question)
	terms := make([]string, 0, 32)
	seen := map[string]bool{}
	add := func(term string) {
		term = strings.TrimSpace(term)
		if runeLen(term) < 2 || qaStopWords[term] || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}
	var ascii strings.Builder
	var han []rune
	flushASCII := func() {
		if ascii.Len() >= 2 {
			add(ascii.String())
		}
		ascii.Reset()
	}
	flushHan := func() {
		for i := 0; i+1 < len(han); i += 1 {
			add(string(han[i : i+2]))
		}
		if len(han) >= 3 && len(han) <= 8 {
			add(string(han))
		}
		han = han[:0]
	}
	for _, r := range lower {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			flushHan()
			ascii.WriteRune(r)
		case unicode.Is(unicode.Han, r):
			flushASCII()
			han = append(han, r)
		default:
			flushASCII()
			flushHan()
		}
	}
	flushASCII()
	flushHan()

	expanded := append([]string(nil), terms...)
	for _, term := range terms {
		for _, synonym := range qaSynonyms[term] {
			if !seen[synonym] {
				seen[synonym] = true
				expanded = append(expanded, synonym)
			}
		}
	}
	return expanded
}

func qaNormalize(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func qaASCIIWord(term string) bool {
	if term == "" {
		return false
	}
	for _, r := range term {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func qaASCIIAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func qaContainsTerm(text, term string) bool {
	if !qaASCIIWord(term) {
		return strings.Contains(text, term)
	}
	for offset := 0; offset < len(text); {
		idx := strings.Index(text[offset:], term)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(term)
		beforeOK := start == 0 || !qaASCIIAlnum(text[start-1])
		afterOK := end >= len(text) || !qaASCIIAlnum(text[end])
		if beforeOK && afterOK {
			return true
		}
		offset = start + len(term)
	}
	return false
}

func qaCountTerm(text, term string) int {
	if !qaASCIIWord(term) {
		return strings.Count(text, term)
	}
	count := 0
	for offset := 0; offset < len(text); {
		idx := strings.Index(text[offset:], term)
		if idx < 0 {
			break
		}
		start := offset + idx
		end := start + len(term)
		beforeOK := start == 0 || !qaASCIIAlnum(text[start-1])
		afterOK := end >= len(text) || !qaASCIIAlnum(text[end])
		if beforeOK && afterOK {
			count += 1
		}
		offset = start + len(term)
	}
	return count
}

func qaFieldScore(text string, terms []string, idf map[string]float64) float64 {
	if text == "" {
		return 0
	}
	lower := strings.ToLower(text)
	score := 0.0
	for _, term := range terms {
		count := qaCountTerm(lower, term)
		if count > 0 {
			score += idf[term] * (1 + math.Log(float64(count)))
		}
	}
	return score
}

func qaChunkText(content string, target int) []string {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	paragraphs := strings.Split(content, "\n\n")
	chunks := make([]string, 0, 8)
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			chunks = append(chunks, text)
		}
		current.Reset()
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		runes := []rune(paragraph)
		if len(runes) > target {
			for len(runes) > 0 {
				take := target
				if take > len(runes) {
					take = len(runes)
				}
				part := strings.TrimSpace(string(runes[:take]))
				if current.Len() > 0 {
					if runeLen(current.String())+runeLen(part) <= target {
						current.WriteString("\n\n")
						current.WriteString(part)
						flush()
					} else {
						flush()
						current.WriteString(part)
					}
				} else {
					current.WriteString(part)
				}
				runes = runes[take:]
			}
			continue
		}
		if current.Len() > 0 && runeLen(current.String())+runeLen(paragraph) > target {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	flush()
	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

// retrieveQAContext 把已发布内容和站点帮助切块后按相关度排序，返回合并后的引用来源。
func (a *API) retrieveQAContext(question string, staff bool) []qaSource {
	terms := qaTerms(question)
	if len(terms) == 0 {
		return nil
	}
	phrase := qaNormalize(question)
	chunks := make([]qaChunk, 0, 160)

	var docs []models.Document
	docQuery := a.DB.Preload("Category").Where("status = ?", models.StatusPublished)
	if hidden := a.hiddenDocIDs(nil); len(hidden) > 0 {
		docQuery = docQuery.Where("documents.id NOT IN ?", hidden)
	}
	docQuery.Order("view_count DESC, id DESC").Limit(500).Find(&docs)
	for _, doc := range docs {
		category := ""
		if doc.Category != nil {
			category = doc.Category.Name
		}
		section := "articles"
		if doc.Kind == models.KindDoc {
			section = "docs"
		}
		ref := fmt.Sprintf("%s-%d", doc.Kind, doc.ID)
		parts := qaChunkText(doc.Content, 620)
		if len(parts) == 0 {
			parts = []string{doc.Abstract}
		}
		for _, part := range parts {
			chunks = append(chunks, qaChunk{
				Ref:      ref,
				ID:       doc.ID,
				Kind:     doc.Kind,
				Title:    doc.Title,
				URL:      fmt.Sprintf("/%s/%d", section, doc.ID),
				Category: category,
				Tags:     doc.Tags,
				Abstract: doc.Abstract,
				Text:     part,
			})
		}
	}

	for _, entry := range siteHelpEntries(staff) {
		feature := entry.Feature
		route := feature.Route
		if route == "" {
			route = "/about#" + feature.ID
		}
		details := strings.Join(feature.Details, "\n")
		chunks = append(chunks, qaChunk{
			Ref:      "help-" + feature.ID,
			Kind:     "help",
			Title:    feature.Title,
			URL:      route,
			Category: entry.Category.Title,
			Abstract: feature.Summary,
			Text:     details,
			Keywords: strings.Join(feature.Keywords, " "),
		})
	}

	if len(chunks) == 0 {
		return nil
	}
	df := make(map[string]int, len(terms))
	for _, term := range terms {
		for _, chunk := range chunks {
			haystack := chunk.Title + "\n" + chunk.Abstract + "\n" + chunk.Text + "\n" + chunk.Keywords + "\n" + strings.Join(chunk.Tags, " ")
			if qaContainsTerm(strings.ToLower(haystack), term) {
				df[term] += 1
			}
		}
	}
	total := float64(len(chunks))
	idf := make(map[string]float64, len(terms))
	for _, term := range terms {
		idf[term] = math.Log(1 + total/float64(df[term]+1))
	}

	scored := make([]qaChunk, 0, len(chunks))
	for _, chunk := range chunks {
		tags := strings.Join(chunk.Tags, " ")
		titleLike := strings.ToLower(chunk.Title + "\n" + chunk.Keywords + "\n" + tags + "\n" + chunk.Category)
		bodyLike := strings.ToLower(chunk.Abstract + "\n" + chunk.Text)
		titleHits := 0
		matched := 0
		for _, term := range terms {
			inTitle := qaContainsTerm(titleLike, term)
			inBody := qaContainsTerm(bodyLike, term)
			if inTitle {
				titleHits += 1
			}
			if inTitle || inBody {
				matched += 1
			}
		}
		phraseMatch := phrase != "" && (strings.Contains(qaNormalize(chunk.Title), phrase) ||
			strings.Contains(qaNormalize(tags), phrase) ||
			strings.Contains(qaNormalize(chunk.Text), phrase))
		// 中文双字词很容易在无关段落里偶然出现；没有标题/关键词命中时，至少要有两个词命中才保留。
		if titleHits == 0 && matched < 2 && !phraseMatch {
			continue
		}
		score := qaFieldScore(chunk.Title, terms, idf)*8 +
			qaFieldScore(tags, terms, idf)*6 +
			qaFieldScore(chunk.Category, terms, idf)*4 +
			qaFieldScore(chunk.Abstract, terms, idf)*3 +
			qaFieldScore(chunk.Text, terms, idf)
		if chunk.Keywords != "" {
			score += qaFieldScore(chunk.Keywords, terms, idf) * 7
		}
		if phrase != "" {
			if strings.Contains(qaNormalize(chunk.Title), phrase) {
				score += 16
			}
			if strings.Contains(qaNormalize(tags), phrase) {
				score += 10
			}
			if strings.Contains(qaNormalize(chunk.Text), phrase) {
				score += 5
			}
		}
		if score <= 0 {
			continue
		}
		chunk.Score = score
		scored = append(scored, chunk)
	}
	if len(scored) == 0 {
		return nil
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	selected := make([]qaChunk, 0, 10)
	perRef := map[string]int{}
	helpCount := 0
	for _, chunk := range scored {
		if len(selected) >= 10 {
			break
		}
		if perRef[chunk.Ref] >= 2 {
			continue
		}
		// 站点帮助最多占 3 段，避免内容类问题被功能说明淹没。
		if chunk.Kind == "help" {
			if helpCount >= 3 {
				continue
			}
			helpCount += 1
		}
		selected = append(selected, chunk)
		perRef[chunk.Ref] += 1
	}

	sources := make([]qaSource, 0, len(selected))
	index := map[string]int{}
	totalChars := 0
	for _, chunk := range selected {
		excerpt := truncateRunes(plainText(chunk.Text), 900)
		if excerpt == "" {
			excerpt = truncateRunes(plainText(chunk.Abstract), 900)
		}
		if idx, ok := index[chunk.Ref]; ok {
			remaining := 7200 - totalChars
			if remaining <= 0 {
				break
			}
			add := truncateRunes(excerpt, remaining)
			if add != "" {
				sources[idx].Excerpt += "\n…\n" + add
				totalChars += runeLen(add)
			}
			continue
		}
		if totalChars+runeLen(excerpt) > 7200 {
			continue
		}
		sources = append(sources, qaSource{
			Ref:      chunk.Ref,
			ID:       chunk.ID,
			Kind:     chunk.Kind,
			Title:    chunk.Title,
			URL:      chunk.URL,
			Category: chunk.Category,
			Tags:     chunk.Tags,
			Excerpt:  excerpt,
		})
		index[chunk.Ref] = len(sources) - 1
		totalChars += runeLen(excerpt)
		if len(sources) >= 8 {
			break
		}
	}
	return sources
}
