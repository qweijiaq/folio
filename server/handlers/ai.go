package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type aiAnalysis struct {
	Title    string   `json:"title"`
	Abstract string   `json:"abstract"`
	Tags     []string `json:"tags"`
}

func (a *API) AIStatus(c *gin.Context) {
	status := a.aiSummaryStatus()
	c.JSON(http.StatusOK, gin.H{
		"enabled":  status["enabled"],
		"provider": status["provider"],
		"model":    status["model"],
		"source":   status["source"],
		"hasKey":   status["hasKey"],
	})
}

func (a *API) callSummaryAI(ctx context.Context, cfg aiSummaryConfig, systemPrompt, userPrompt string) (string, error) {
	if cfg.Provider.APIStyle == "anthropic" {
		return a.callAnthropicQA(ctx, cfg.Provider, cfg.APIKey, cfg.Model, systemPrompt, userPrompt, nil)
	}
	return a.callOpenAICompatQA(ctx, cfg.Provider, cfg.APIKey, cfg.Model, systemPrompt, userPrompt, nil)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

func normalizeAITags(tags []string) []string {
	out := make([]string, 0, 6)
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		tag = truncateRunes(tag, 16)
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func (a *API) AIAnalyze(c *gin.Context) {
	aiConfig, err := a.aiSummaryConfig()
	if err != nil {
		fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	var in struct {
		Content string `json:"content"`
		Title   string `json:"title"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if runeLen(in.Content) < 20 {
		bad(c, "正文太短，至少写 20 个字后再使用 AI 摘要")
		return
	}
	if runeLen(in.Content) > 60000 {
		bad(c, "正文超过 6 万字，AI 摘要暂不支持这么长的内容")
		return
	}

	systemPrompt := `你是一名中文写作编辑。请阅读用户给出的文章正文，提炼标题、摘要和标签。要求：
1. 只输出一个 JSON 对象，不要 Markdown 代码块，不要解释。
2. 格式严格为 {"title":"...","abstract":"...","tags":["..."]}。
3. 标题不超过 24 个汉字；摘要不超过 90 个汉字；标签 3-6 个，每个不超过 12 个汉字。
4. 摘要要具体，不要使用“本文介绍了”之类的空话。`
	userPrompt := fmt.Sprintf("原标题（可能为空，可参考或改写）：%s\n\n正文：\n%s", strings.TrimSpace(in.Title), in.Content)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	text, err := a.callSummaryAI(ctx, aiConfig, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("AI 摘要请求失败: %v", err)
		fail(c, http.StatusBadGateway, "AI 请求失败，请稍后再试")
		return
	}
	raw := extractJSONObject(text)
	if raw == "" {
		fail(c, http.StatusBadGateway, "AI 返回格式异常")
		return
	}
	var result aiAnalysis
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Printf("AI 返回 JSON 解析失败: %v raw=%s", err, raw)
		fail(c, http.StatusBadGateway, "AI 返回格式异常")
		return
	}
	result.Title = truncateRunes(strings.TrimSpace(result.Title), 24)
	result.Abstract = truncateRunes(strings.TrimSpace(result.Abstract), 90)
	result.Tags = normalizeAITags(result.Tags)
	if result.Title == "" && result.Abstract == "" && len(result.Tags) == 0 {
		fail(c, http.StatusBadGateway, "AI 没有返回有效内容")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"title":    result.Title,
		"abstract": result.Abstract,
		"tags":     result.Tags,
	})
}
