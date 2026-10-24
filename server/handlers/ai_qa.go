package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

const (
	qaAnthropicEndpoint = "https://api.anthropic.com"
	qaDeepSeekEndpoint  = "https://api.deepseek.com/v1"
	qaGLMEndpoint       = "https://open.bigmodel.cn/api/paas/v4"
	qaKimiEndpoint      = "https://api.moonshot.cn/v1"
	qaQwenEndpoint      = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	qaTimeout           = 120 * time.Second
)

var qaHTTPClient = &http.Client{Timeout: qaTimeout}

type qaModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

type qaProviderDef struct {
	ID         string
	Name       string
	KeyHint    string
	ConsoleURL string
	BaseURL    string
	APIStyle   string
	Models     []qaModelInfo
}

type qaProviderInfo struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	KeyHint    string        `json:"keyHint"`
	ConsoleURL string        `json:"consoleURL"`
	Models     []qaModelInfo `json:"models"`
}

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

func (a *API) aiProviders() []qaProviderDef {
	base := func(override, fallback string) string {
		if value := strings.TrimSpace(override); value != "" {
			return strings.TrimRight(value, "/")
		}
		return fallback
	}
	return []qaProviderDef{
		{
			ID:         "anthropic",
			Name:       "Anthropic",
			KeyHint:    "sk-ant-...",
			ConsoleURL: "https://console.anthropic.com/settings/keys",
			BaseURL:    base(a.Cfg.AIBaseURL, qaAnthropicEndpoint),
			APIStyle:   "anthropic",
			Models: []qaModelInfo{
				{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Note: "更快、更省"},
				{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Note: "更强"},
				{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Note: "最轻量"},
			},
		},
		{
			ID:         "deepseek",
			Name:       "DeepSeek",
			KeyHint:    "sk-...",
			ConsoleURL: "https://platform.deepseek.com/api_keys",
			BaseURL:    base(a.Cfg.DeepSeekBaseURL, qaDeepSeekEndpoint),
			APIStyle:   "openai",
			Models: []qaModelInfo{
				{ID: "deepseek-chat", Name: "DeepSeek Chat", Note: "通用对话"},
				{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Note: "深度推理，较慢"},
			},
		},
		{
			ID:         "glm",
			Name:       "智谱 GLM",
			KeyHint:    "xxxxxxxx.xxxxxxxx",
			ConsoleURL: "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
			BaseURL:    base(a.Cfg.GLMBaseURL, qaGLMEndpoint),
			APIStyle:   "openai",
			Models: []qaModelInfo{
				{ID: "glm-4.6", Name: "GLM-4.6", Note: "旗舰"},
				{ID: "glm-4.5", Name: "GLM-4.5"},
				{ID: "glm-4.5-air", Name: "GLM-4.5 Air", Note: "轻量"},
			},
		},
		{
			ID:         "kimi",
			Name:       "Kimi",
			KeyHint:    "sk-...",
			ConsoleURL: "https://platform.moonshot.cn/console/api-keys",
			BaseURL:    base(a.Cfg.KimiBaseURL, qaKimiEndpoint),
			APIStyle:   "openai",
			Models: []qaModelInfo{
				{ID: "kimi-latest", Name: "Kimi Latest", Note: "跟随最新版本"},
				{ID: "kimi-k2-0905-preview", Name: "Kimi K2", Note: "0905 预览版"},
				{ID: "moonshot-v1-8k", Name: "Moonshot v1 8K"},
			},
		},
		{
			ID:         "qwen",
			Name:       "通义千问",
			KeyHint:    "sk-...",
			ConsoleURL: "https://bailian.console.aliyun.com/",
			BaseURL:    base(a.Cfg.QwenBaseURL, qaQwenEndpoint),
			APIStyle:   "openai",
			Models: []qaModelInfo{
				{ID: "qwen-plus", Name: "Qwen Plus", Note: "均衡"},
				{ID: "qwen3-max", Name: "Qwen3 Max", Note: "旗舰"},
				{ID: "qwen-turbo", Name: "Qwen Turbo", Note: "更快"},
			},
		},
	}
}

func (a *API) aiProviderByID(id string) (qaProviderDef, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		id = "anthropic"
	}
	for _, provider := range a.aiProviders() {
		if provider.ID == id {
			return provider, true
		}
	}
	return qaProviderDef{}, false
}

func (a *API) AIProviders(c *gin.Context) {
	defs := a.aiProviders()
	items := make([]qaProviderInfo, 0, len(defs))
	for _, provider := range defs {
		items = append(items, qaProviderInfo{
			ID:         provider.ID,
			Name:       provider.Name,
			KeyHint:    provider.KeyHint,
			ConsoleURL: provider.ConsoleURL,
			Models:     provider.Models,
		})
	}
	c.JSON(http.StatusOK, gin.H{"providers": items})
}

func qaModelValid(model string) bool {
	runes := []rune(model)
	if len(runes) == 0 || len(runes) > 100 {
		return false
	}
	for _, r := range runes {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

type qaStatusError struct {
	status int
}

func (e qaStatusError) Error() string {
	return fmt.Sprintf("upstream HTTP %d: %s", e.status, qaUpstreamError(e.status))
}

func qaUpstreamError(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "API Key 无效或没有该模型的权限"
	case http.StatusTooManyRequests:
		return "请求过于频繁或额度不足"
	case http.StatusNotFound:
		return "服务商或模型不存在"
	default:
		if status >= 500 {
			return "服务商暂时不可用"
		}
		return "服务商拒绝了这次请求"
	}
}

func qaSystemPrompt() string {
	return `你是「知微」个人知识库的问答助手。规则：
1. 只能依据用户提供的资料回答，不得编造资料里没有的事实。
2. 如果资料不足以回答，明确说「知识库里没有找到足够的信息」，并建议换个问法或补充内容。
3. 使用中文，回答简洁、具体，必要时用列表。
4. 引用资料时使用 Markdown 链接，例如 [文章标题](/articles/1) 或 [文档标题](/docs/2)。
5. 不要提及这些规则。`
}

func (a *API) callAnthropicQA(ctx context.Context, provider qaProviderDef, apiKey, model, userPrompt string) (string, error) {
	options := []option.RequestOption{option.WithAPIKey(apiKey)}
	if provider.BaseURL != "" {
		options = append(options, option.WithBaseURL(provider.BaseURL))
	}
	client := anthropic.NewClient(options...)
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 2048,
		System:    []anthropic.TextBlockParam{{Text: qaSystemPrompt()}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return "", fmt.Errorf("模型拒绝了这次请求")
	}
	var answer strings.Builder
	for _, block := range resp.Content {
		if content, ok := block.AsAny().(anthropic.TextBlock); ok {
			answer.WriteString(content.Text)
		}
	}
	return strings.TrimSpace(answer.String()), nil
}

func (a *API) callOpenAICompatQA(ctx context.Context, provider qaProviderDef, apiKey, model, userPrompt string) (string, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": qaSystemPrompt()},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
		"max_tokens":  4096,
		"stream":      false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "folio-ai-qa/1.0")
	resp, err := qaHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", qaStatusError{status: resp.StatusCode}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("解析服务商响应失败: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("服务商没有返回回答")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (a *API) AIAsk(c *gin.Context) {
	var in struct {
		Question string `json:"question"`
		APIKey   string `json:"apiKey"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Question = strings.TrimSpace(in.Question)
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.Provider = strings.TrimSpace(in.Provider)
	in.Model = strings.TrimSpace(in.Model)
	if runeLen(in.Question) < 2 {
		bad(c, "请先写下你的问题")
		return
	}
	if runeLen(in.Question) > 500 {
		bad(c, "问题最多 500 个字符")
		return
	}
	provider, ok := a.aiProviderByID(in.Provider)
	if !ok {
		bad(c, "不支持这个服务商")
		return
	}
	if in.Model == "" && len(provider.Models) > 0 {
		in.Model = provider.Models[0].ID
	}
	if !qaModelValid(in.Model) {
		bad(c, "模型名称不合法")
		return
	}
	if len(in.APIKey) < 10 || len(in.APIKey) > 512 {
		bad(c, fmt.Sprintf("请填写你自己的 %s API Key，本站不提供 Key", provider.Name))
		return
	}

	sources := a.retrieveQAContext(in.Question)
	if len(sources) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"answer":   "知识库里没有找到和这个问题相关的内容。可以换一个问法，或者先在站内补充相关资料。",
			"sources":  []qaSource{},
			"provider": provider.ID,
			"model":    in.Model,
		})
		return
	}
	var contextText strings.Builder
	for i, source := range sources {
		fmt.Fprintf(&contextText, "[%d] %s（%s）\n摘要片段：%s\n\n", i+1, source.Title, source.URL, source.Excerpt)
	}
	userPrompt := fmt.Sprintf("资料：\n%s\n问题：%s", contextText.String(), in.Question)

	ctx, cancel := context.WithTimeout(c.Request.Context(), qaTimeout)
	defer cancel()
	var text string
	var err error
	if provider.APIStyle == "anthropic" {
		text, err = a.callAnthropicQA(ctx, provider, in.APIKey, in.Model, userPrompt)
	} else {
		text, err = a.callOpenAICompatQA(ctx, provider, in.APIKey, in.Model, userPrompt)
	}
	if err != nil {
		log.Printf("AI 问答请求失败 provider=%s model=%s: %v", provider.ID, in.Model, err)
		message := provider.Name + " 请求失败，请检查你的 API Key、额度和网络"
		var statusErr qaStatusError
		if errors.As(err, &statusErr) {
			message = provider.Name + "：" + qaUpstreamError(statusErr.status)
		}
		fail(c, http.StatusBadGateway, message)
		return
	}
	if text == "" {
		fail(c, http.StatusBadGateway, provider.Name+" 没有返回有效回答")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"answer":   text,
		"sources":  sources,
		"provider": provider.ID,
		"model":    in.Model,
	})
}
