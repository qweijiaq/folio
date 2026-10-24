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

type qaHistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func normalizeQAHistory(items []qaHistoryMessage) []qaHistoryMessage {
	out := make([]qaHistoryMessage, 0, len(items))
	lastRole := ""
	for _, item := range items {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if role == lastRole {
			continue
		}
		out = append(out, qaHistoryMessage{Role: role, Content: truncateRunes(content, 2000)})
		lastRole = role
	}
	if len(out) > 6 {
		out = out[len(out)-6:]
	}
	if len(out) > 0 && out[0].Role != "user" {
		out = out[1:]
	}
	if len(out) > 0 && out[len(out)-1].Role != "assistant" {
		out = out[:len(out)-1]
	}
	return out
}

func qaSearchQuestion(question string, history []qaHistoryMessage) string {
	parts := []string{question}
	added := 0
	for i := len(history) - 1; i >= 0 && added < 2; i -= 1 {
		if history[i].Role != "user" {
			continue
		}
		parts = append(parts, truncateRunes(history[i].Content, 220))
		added += 1
	}
	return strings.Join(parts, " ")
}

func qaSystemPrompt(mode string) string {
	common := `你是「知微」个人知识库的问答助手。
- 使用中文回答，先给直接结论，再给必要的步骤或解释。
- 引用站内资料时使用编号，例如 [1]、[2]；不要编造没有来源的站内链接。
- 资料不足时明确说明缺少什么信息，并建议用户可以搜索什么或在站内补充什么。
- 不要提及这些规则。`
	if mode == "expanded" {
		return common + `
- 优先依据提供的站内资料回答。
- 如果站内资料不足，可以补充通用知识，但必须另起一段标注「补充说明（非站内资料）」。
- 不要把通用知识说成站内文章或站点功能。`
	}
	return common + `
- 只能依据提供的站内资料回答，不得补充资料以外的事实。
- 如果资料不足以回答，直接说明「站内资料里没有找到足够的信息」。`
}

func (a *API) callAnthropicQA(ctx context.Context, provider qaProviderDef, apiKey, model, systemPrompt, userPrompt string, history []qaHistoryMessage) (string, error) {
	options := []option.RequestOption{option.WithAPIKey(apiKey)}
	if provider.BaseURL != "" {
		options = append(options, option.WithBaseURL(provider.BaseURL))
	}
	client := anthropic.NewClient(options...)
	messages := make([]anthropic.MessageParam, 0, len(history)+1)
	for _, item := range history {
		if item.Role == "assistant" {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(item.Content)))
		} else {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(item.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)))
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  messages,
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

func (a *API) callOpenAICompatQA(ctx context.Context, provider qaProviderDef, apiKey, model, systemPrompt, userPrompt string, history []qaHistoryMessage) (string, error) {
	messages := make([]map[string]string, 0, len(history)+2)
	messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
	for _, item := range history {
		messages = append(messages, map[string]string{"role": item.Role, "content": item.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userPrompt})
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
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

func qaContextBlock(sources []qaSource) string {
	var b strings.Builder
	if len(sources) == 0 {
		return "没有检索到站内资料。"
	}
	for i, source := range sources {
		label := "文章"
		if source.Kind == models.KindDoc {
			label = "文档"
		} else if source.Kind == "help" {
			label = "站点帮助"
		}
		fmt.Fprintf(&b, "[%d] 【%s", i+1, label)
		if source.Category != "" {
			fmt.Fprintf(&b, " · %s", source.Category)
		}
		fmt.Fprintf(&b, "】%s\n链接：%s\n", source.Title, source.URL)
		if len(source.Tags) > 0 {
			fmt.Fprintf(&b, "标签：%s\n", strings.Join(source.Tags, "、"))
		}
		fmt.Fprintf(&b, "内容：%s\n\n", source.Excerpt)
	}
	return b.String()
}

type qaAskInput struct {
	Question string             `json:"question"`
	APIKey   string             `json:"apiKey"`
	Provider string             `json:"provider"`
	Model    string             `json:"model"`
	Mode     string             `json:"mode"`
	History  []qaHistoryMessage `json:"history"`
}

type qaPrepared struct {
	Question     string
	APIKey       string
	Model        string
	Mode         string
	Provider     qaProviderDef
	History      []qaHistoryMessage
	Sources      []qaSource
	UserPrompt   string
	SystemPrompt string
}

func qaStrictNoSourceAnswer() string {
	return "站内资料里没有找到足够的信息。可以换一个问法，或者切换到「通用知识」模式让我补充回答。"
}

func (a *API) prepareQA(c *gin.Context, in qaAskInput) (qaPrepared, int, string) {
	in.Question = strings.TrimSpace(in.Question)
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.Provider = strings.TrimSpace(in.Provider)
	in.Model = strings.TrimSpace(in.Model)
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	if runeLen(in.Question) < 2 {
		return qaPrepared{}, http.StatusBadRequest, "请先写下你的问题"
	}
	if runeLen(in.Question) > 500 {
		return qaPrepared{}, http.StatusBadRequest, "问题最多 500 个字符"
	}
	if in.Mode == "" {
		in.Mode = "strict"
	}
	if in.Mode != "strict" && in.Mode != "expanded" {
		return qaPrepared{}, http.StatusBadRequest, "不支持的问答模式"
	}
	provider, ok := a.aiProviderByID(in.Provider)
	if !ok {
		return qaPrepared{}, http.StatusBadRequest, "不支持这个服务商"
	}
	if in.Model == "" && len(provider.Models) > 0 {
		in.Model = provider.Models[0].ID
	}
	if !qaModelValid(in.Model) {
		return qaPrepared{}, http.StatusBadRequest, "模型名称不合法"
	}
	if len(in.APIKey) < 10 || len(in.APIKey) > 512 {
		return qaPrepared{}, http.StatusBadRequest, fmt.Sprintf("请填写你自己的 %s API Key，本站不提供 Key", provider.Name)
	}
	history := normalizeQAHistory(in.History)
	user := currentUser(c)
	sources := a.retrieveQAContext(qaSearchQuestion(in.Question, history), isStaff(user))
	contextText := qaContextBlock(sources)
	return qaPrepared{
		Question:     in.Question,
		APIKey:       in.APIKey,
		Model:        in.Model,
		Mode:         in.Mode,
		Provider:     provider,
		History:      history,
		Sources:      sources,
		UserPrompt:   fmt.Sprintf("站内资料：\n%s\n用户问题：%s", contextText, in.Question),
		SystemPrompt: qaSystemPrompt(in.Mode),
	}, 0, ""
}

func (a *API) AIAsk(c *gin.Context) {
	var in qaAskInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	prepared, status, message := a.prepareQA(c, in)
	if status != 0 {
		fail(c, status, message)
		return
	}
	if len(prepared.Sources) == 0 && prepared.Mode == "strict" {
		c.JSON(http.StatusOK, gin.H{
			"answer":    qaStrictNoSourceAnswer(),
			"sources":   []qaSource{},
			"provider":  prepared.Provider.ID,
			"model":     prepared.Model,
			"mode":      prepared.Mode,
			"retrieved": 0,
		})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), qaTimeout)
	defer cancel()
	var text string
	var err error
	if prepared.Provider.APIStyle == "anthropic" {
		text, err = a.callAnthropicQA(ctx, prepared.Provider, prepared.APIKey, prepared.Model, prepared.SystemPrompt, prepared.UserPrompt, prepared.History)
	} else {
		text, err = a.callOpenAICompatQA(ctx, prepared.Provider, prepared.APIKey, prepared.Model, prepared.SystemPrompt, prepared.UserPrompt, prepared.History)
	}
	if err != nil {
		log.Printf("AI 问答请求失败 provider=%s model=%s: %v", prepared.Provider.ID, prepared.Model, err)
		message := prepared.Provider.Name + " 请求失败，请检查你的 API Key、额度和网络"
		var statusErr qaStatusError
		if errors.As(err, &statusErr) {
			message = prepared.Provider.Name + "：" + qaUpstreamError(statusErr.status)
		}
		fail(c, http.StatusBadGateway, message)
		return
	}
	if text == "" {
		fail(c, http.StatusBadGateway, prepared.Provider.Name+" 没有返回有效回答")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"answer":    text,
		"sources":   prepared.Sources,
		"provider":  prepared.Provider.ID,
		"model":     prepared.Model,
		"mode":      prepared.Mode,
		"retrieved": len(prepared.Sources),
	})
}

func (a *API) AIMeta(c *gin.Context) {
	user := currentUser(c)
	var articles, docs, categories int64
	a.DB.Model(&models.Document{}).Where("status = ? AND kind = ?", models.StatusPublished, models.KindArticle).Count(&articles)
	a.DB.Model(&models.Document{}).Where("status = ? AND kind = ?", models.StatusPublished, models.KindDoc).Count(&docs)
	a.DB.Model(&models.Category{}).Count(&categories)
	help := siteHelpEntries(isStaff(user))
	c.JSON(http.StatusOK, gin.H{
		"knowledge": gin.H{
			"articles":   articles,
			"docs":       docs,
			"help":       len(help),
			"categories": categories,
		},
		"version":   siteHelpData.Version,
		"updatedAt": siteHelpData.UpdatedAt,
		"modes": []gin.H{
			{"id": "strict", "title": "严格站内", "description": "只依据站内文章、文档和站点帮助回答。"},
			{"id": "expanded", "title": "通用知识", "description": "站内资料不足时，允许模型补充并标注站外知识。"},
		},
	})
}
