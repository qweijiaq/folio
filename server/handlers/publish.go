package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type publishReviewInput struct {
	Content        string   `json:"content"`
	Title          string   `json:"title"`
	Abstract       string   `json:"abstract"`
	Tags           []string `json:"tags"`
	Perspective    string   `json:"perspective"`
	Mode           string   `json:"mode"`
	Kind           string   `json:"kind"`
	Reference      string   `json:"reference"`
	ReferenceTitle string   `json:"referenceTitle"`
}

type publishEdit struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
	Reason  string `json:"reason"`
}

type publishReviewOutput struct {
	Issues   []string      `json:"issues"`
	Edits    []publishEdit `json:"edits"`
	Title    string        `json:"title"`
	Abstract string        `json:"abstract"`
	Tags     []string      `json:"tags"`
}

func publishPerspectiveText(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "first":
		return "第一人称：以“我”的视角叙述，保持真诚、具体。"
	case "third":
		return "第三人称：用“他/她/他们”或客观叙述，避免直接对读者说话。"
	default:
		return "上帝视角：全文不要出现“我”“我们”“你”，用“人”“一个人”“人们”或客观陈述。"
	}
}

func publishModeText(value string) string {
	if strings.ToLower(strings.TrimSpace(value)) == "proofread" {
		return "只做必要的事实、错别字、标点、语法和结构修正，尽量保留原句。"
	}
	return "在修正错误的基础上，去掉 AI 味、模板化过渡和空泛总结，统一视角，让文字更自然、具体。"
}

// PublishReview 让已配置的 AI 摘要服务审校待发布内容，并返回问题、逐条修改建议和标题/摘要/标签。
func (a *API) PublishReview(c *gin.Context) {
	var in publishReviewInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if runeLen(in.Content) < 20 {
		bad(c, "正文太短，至少 20 个字后再审校")
		return
	}
	if runeLen(in.Content) > 60000 {
		bad(c, "正文超过 6 万字，建议分章审校")
		return
	}
	in.Reference = strings.TrimSpace(in.Reference)
	if runeLen(in.Reference) > 6000 {
		in.Reference = truncateRunes(in.Reference, 6000)
	}
	kind := "文章"
	if in.Kind == "doc" {
		kind = "文档"
	}
	cfg, err := a.aiSummaryConfig()
	if err != nil {
		fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	systemPrompt := `你是一名中文编辑，负责审校准备发布到「知微」个人知识站的文章或文档。

要求：
1. 找出错别字、标点错误、重复词、病句、逻辑跳跃、标题层级或编号错误、图注编号不连续。
2. ` + publishModeText(in.Mode) + `
3. ` + publishPerspectiveText(in.Perspective) + `
4. 如果提供了「参考文风」，先体会它的叙述视角、句子长短、段落节奏、标题层级、用词习惯和举例方式，再把待审内容改写到接近这种写法。只学习写法，不要照抄参考文中的句子、事实或观点；不要改变待审内容本身的事实和信息。
5. 不改变事实、观点和原文结构；不要新增原文没有的信息。
6. 只输出一个 JSON 对象，不要 Markdown 代码块，不要解释。

JSON 格式：
{
  "issues": ["问题1", "问题2"],
  "edits": [
    {"find": "原文中连续出现的一段文字", "replace": "替换后的文字", "reason": "修改原因"}
  ],
  "title": "建议标题，可为空",
  "abstract": "建议摘要，可为空",
  "tags": ["标签1", "标签2"]
}

edits 的 find 必须与原文逐字匹配；可以是短句，也可以是一个完整段落。优先用整段替换来统一文风，单条 find 不超过 500 字，不要返回整篇全文。最多 30 条 edits。`

	userPrompt := "标题：" + strings.TrimSpace(in.Title) +
		"\n摘要：" + strings.TrimSpace(in.Abstract) +
		"\n标签：" + strings.Join(in.Tags, "、") +
		"\n类型：" + kind
	if in.Reference != "" {
		name := strings.TrimSpace(in.ReferenceTitle)
		if name == "" {
			name = "未命名参考文档"
		}
		userPrompt += "\n\n参考文风（《" + name + "》，只学习写法，不要照抄）：\n" + in.Reference
	}
	userPrompt += "\n\n待审正文：\n" + in.Content

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	raw, err := a.callSummaryAI(ctx, cfg, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("发布助审校失败: %v", err)
		fail(c, http.StatusBadGateway, "AI 审校失败，请稍后重试")
		return
	}
	rawJSON := extractJSONObject(raw)
	if rawJSON == "" {
		log.Printf("发布助审校返回格式异常: %s", truncateRunes(raw, 500))
		fail(c, http.StatusBadGateway, "AI 返回格式异常，请重试")
		return
	}
	var out publishReviewOutput
	if err := json.Unmarshal([]byte(rawJSON), &out); err != nil {
		log.Printf("发布助审校 JSON 解析失败: %v", err)
		fail(c, http.StatusBadGateway, "AI 返回格式异常，请重试")
		return
	}
	if out.Issues == nil {
		out.Issues = []string{}
	}
	if out.Edits == nil {
		out.Edits = []publishEdit{}
	}
	if len(out.Edits) > 30 {
		out.Edits = out.Edits[:30]
	}
	out.Tags = normalizeTags(out.Tags)
	c.JSON(http.StatusOK, out)
}
