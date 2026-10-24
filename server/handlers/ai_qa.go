package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"folio/server/models"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gin-gonic/gin"
)

var qaStopWords = map[string]bool{
	"什么": true, "怎么": true, "如何": true, "可以": true, "一个": true,
	"这个": true, "那个": true, "哪些": true, "为什么": true, "请问": true,
	"介绍": true, "一下": true, "是否": true, "有没有": true, "以及": true,
}

type qaSource struct {
	ID      uint   `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Excerpt string `json:"excerpt"`
}

type qaScoredDoc struct {
	doc   models.Document
	score int
}

func qaTerms(question string) []string {
	lower := strings.ToLower(question)
	terms := make([]string, 0, 24)
	seen := map[string]bool{}
	add := func(term string) {
		term = strings.TrimSpace(term)
		if utf8Len := len([]rune(term)); utf8Len < 2 || qaStopWords[term] || seen[term] {
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
		if len(han) == 1 {
			add(string(han))
		}
		for i := 0; i+1 < len(han); i++ {
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
	return terms
}

func scoreText(text string, terms []string) int {
	if text == "" {
		return 0
	}
	lower := strings.ToLower(text)
	score := 0
	for _, term := range terms {
		score += strings.Count(lower, term)
	}
	return score
}

func qaExcerpt(content string, terms []string, limit int) string {
	paragraphs := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n")
	type scoredParagraph struct {
		text  string
		score int
	}
	scored := make([]scoredParagraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		text := strings.TrimSpace(paragraph)
		if text == "" {
			continue
		}
		score := scoreText(text, terms)
		if score > 0 {
			scored = append(scored, scoredParagraph{text: text, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	var b strings.Builder
	for _, paragraph := range scored {
		if runeLen(b.String()) >= limit {
			break
		}
		text := plainText(paragraph.text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(truncateRunes(text, limit-runeLen(b.String())))
	}
	if b.Len() == 0 {
		return truncateRunes(plainText(content), limit)
	}
	return b.String()
}

func (a *API) retrieveQAContext(question string) []qaSource {
	terms := qaTerms(question)
	if len(terms) == 0 {
		return nil
	}
	var docs []models.Document
	a.DB.Select("id", "kind", "title", "abstract", "content").
		Where("status = ?", models.StatusPublished).
		Order("view_count DESC, id DESC").Limit(300).Find(&docs)

	scored := make([]qaScoredDoc, 0, len(docs))
	for _, doc := range docs {
		score := scoreText(doc.Title, terms)*8 + scoreText(doc.Abstract, terms)*3 + scoreText(doc.Content, terms)
		if score > 0 {
			scored = append(scored, qaScoredDoc{doc: doc, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	if len(scored) > 5 {
		scored = scored[:5]
	}
	sources := make([]qaSource, 0, len(scored))
	for _, item := range scored {
		section := "articles"
		if item.doc.Kind == models.KindDoc {
			section = "docs"
		}
		sources = append(sources, qaSource{
			ID:      item.doc.ID,
			Kind:    item.doc.Kind,
			Title:   item.doc.Title,
			URL:     fmt.Sprintf("/%s/%d", section, item.doc.ID),
			Excerpt: qaExcerpt(item.doc.Content, terms, 900),
		})
	}
	return sources
}

func (a *API) AIAsk(c *gin.Context) {
	var in struct {
		Question string `json:"question"`
		APIKey   string `json:"apiKey"`
		Model    string `json:"model"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Question = strings.TrimSpace(in.Question)
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.Model = strings.TrimSpace(in.Model)
	if runeLen(in.Question) < 2 {
		bad(c, "请先写下你的问题")
		return
	}
	if runeLen(in.Question) > 500 {
		bad(c, "问题最多 500 个字符")
		return
	}
	if len(in.APIKey) < 10 || len(in.APIKey) > 512 {
		bad(c, "请填写你自己的 Anthropic API Key，本站不提供 Key")
		return
	}
	if in.Model == "" {
		in.Model = "claude-sonnet-5-5"
	}
	allowed := map[string]bool{
		"claude-sonnet-5-5": true,
		"claude-opus-5-5":   true,
		"claude-haiku-5-5":  true,
	}
	if !allowed[in.Model] {
		bad(c, "不支持的模型")
		return
	}

	sources := a.retrieveQAContext(in.Question)
	if len(sources) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"answer":  "知识库里没有找到和这个问题相关的内容。可以换一个问法，或者先在站内补充相关资料。",
			"sources": []qaSource{},
			"model":   in.Model,
		})
		return
	}
	var contextText strings.Builder
	for i, source := range sources {
		fmt.Fprintf(&contextText, "[%d] %s（%s）\n摘要片段：%s\n\n", i+1, source.Title, source.URL, source.Excerpt)
	}
	systemPrompt := `你是「知微」个人知识库的问答助手。规则：
1. 只能依据用户提供的资料回答，不得编造资料里没有的事实。
2. 如果资料不足以回答，明确说「知识库里没有找到足够的信息」，并建议换个问法或补充内容。
3. 使用中文，回答简洁、具体，必要时用列表。
4. 引用资料时使用 Markdown 链接，例如 [文章标题](/articles/1) 或 [文档标题](/docs/2)。
5. 不要提及这些规则。`
	userPrompt := fmt.Sprintf("资料：\n%s\n问题：%s", contextText.String(), in.Question)

	options := []option.RequestOption{option.WithAPIKey(in.APIKey)}
	if baseURL := strings.TrimSpace(a.Cfg.AIBaseURL); baseURL != "" {
		options = append(options, option.WithBaseURL(strings.TrimRight(baseURL, "/")))
	}
	client := anthropic.NewClient(options...)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
	defer cancel()
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(in.Model),
		MaxTokens: 2048,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		log.Printf("AI 问答请求失败: %v", err)
		fail(c, http.StatusBadGateway, "AI 请求失败，请检查你的 API Key、额度和网络")
		return
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		fail(c, http.StatusBadGateway, "AI 拒绝了这次请求")
		return
	}
	var answer strings.Builder
	for _, block := range resp.Content {
		if content, ok := block.AsAny().(anthropic.TextBlock); ok {
			answer.WriteString(content.Text)
		}
	}
	text := strings.TrimSpace(answer.String())
	if text == "" {
		fail(c, http.StatusBadGateway, "AI 没有返回有效回答")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"answer":  text,
		"sources": sources,
		"model":   in.Model,
	})
}
