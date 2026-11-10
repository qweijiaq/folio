package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type markdownFormatInput struct {
	Content string `json:"content"`
}

const markdownFormatChunkRunes = 5000

func markdownFormatPrompt() string {
	fence := strings.Repeat(string(rune(96)), 3)
	return "你是一名严谨的 Markdown 排版编辑器。请只调整 Markdown 语法和排版，不改变任何文字内容。\n\n" +
		"必须遵守：\n" +
		"1. 不改写句子，不增删段落，不改变事实、数字、链接、图片路径和代码内容。\n" +
		"2. 标题层级连续：正文主标题用 #，小节用 ##，更小的节用 ###；不要强行添加原文没有的标题。\n" +
		"3. 标题前后各空一行，段落之间空一行，不要留下连续两个以上空行。\n" +
		"4. 列表统一使用 - 或 1.，同一层级缩进一致。\n" +
		"5. 代码围栏使用 " + fence + " 开头结尾，尽量保留原有语言标记。\n" +
		"6. 图片 ![alt](url)、链接 [text](url) 保持有效，修复多余空格、缺失括号和错误转义。\n" +
		"7. 表格前后空行，分隔行使用 | --- |。\n" +
		"8. 引用块 > 后加一个空格。\n" +
		"9. 去掉没有意义的重复星号、井号或分隔线，但保留真正用于分隔章节的 ---。\n" +
		"10. 只返回排版后的 Markdown 正文，不要解释，不要用代码块包裹整篇内容。"
}

// splitMarkdownChunks 按行切分长文，避免把代码围栏从中间切断。
func splitMarkdownChunks(text string, maxRunes int) []string {
	lines := strings.Split(text, "\n")
	chunks := make([]string, 0, 4)
	current := make([]string, 0, 40)
	currentRunes := 0
	inFence := false

	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, strings.Join(current, "\n"))
		current = current[:0]
		currentRunes = 0
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isFence := strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
		lineRunes := utf8.RuneCountInString(line) + 1
		if isFence {
			if !inFence && currentRunes > 0 && currentRunes+lineRunes > maxRunes {
				flush()
			}
			current = append(current, line)
			currentRunes += lineRunes
			inFence = !inFence
			continue
		}
		if !inFence && currentRunes > 0 && currentRunes+lineRunes > maxRunes {
			flush()
		}
		current = append(current, line)
		currentRunes += lineRunes
	}
	flush()
	return chunks
}

func stripMarkdownFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") && !strings.HasPrefix(text, "~~~") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return text
	}
	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "```") && !strings.HasPrefix(first, "~~~") {
		return text
	}
	lines = lines[1:]
	if len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "```" || last == "~~~" {
			lines = lines[:len(lines)-1]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (a *API) formatMarkdownChunk(ctx context.Context, cfg aiSummaryConfig, chunk string) (string, error) {
	userPrompt := "请排版下面这段 Markdown：\n\n" + chunk
	raw, err := a.callSummaryAIWithOptions(ctx, cfg, markdownFormatPrompt(), userPrompt, 8192, false)
	if err != nil {
		return "", err
	}
	out := stripMarkdownFence(raw)
	if out == "" {
		return "", fmt.Errorf("AI 没有返回排版结果")
	}
	inLen := utf8.RuneCountInString(chunk)
	outLen := utf8.RuneCountInString(out)
	if inLen >= 80 && (outLen < inLen*55/100 || outLen > inLen*175/100) {
		return "", fmt.Errorf("AI 排版前后长度差异过大")
	}
	return out, nil
}

// MarkdownFormat 用已配置的 AI 服务整理 Markdown 语法，不修改正文内容。
func (a *API) MarkdownFormat(c *gin.Context) {
	var in markdownFormatInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if runeLen(in.Content) < 20 {
		bad(c, "正文太短，至少写 20 个字后再排版")
		return
	}
	if runeLen(in.Content) > 25000 {
		bad(c, "正文超过 2.5 万字，建议分段排版")
		return
	}
	cfg, err := a.aiSummaryConfig()
	if err != nil {
		fail(c, http.StatusServiceUnavailable, err.Error())
		return
	}

	chunks := splitMarkdownChunks(in.Content, markdownFormatChunkRunes)
	outputs := make([]string, 0, len(chunks))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()
	for index, chunk := range chunks {
		out, err := a.formatMarkdownChunk(ctx, cfg, chunk)
		if err != nil {
			if errors.Is(err, errAIOutputTruncated) {
				fail(c, http.StatusBadGateway, "AI 输出被截断，请把正文分短一些再排版")
				return
			}
			log.Printf("Markdown 排版第 %d 段失败: %v", index+1, err)
			fail(c, http.StatusBadGateway, "AI 排版失败，请稍后重试")
			return
		}
		outputs = append(outputs, out)
	}
	result := strings.TrimSpace(strings.Join(outputs, "\n\n"))
	if result == "" {
		fail(c, http.StatusBadGateway, "AI 没有返回排版结果")
		return
	}
	c.JSON(http.StatusOK, gin.H{"markdown": result})
}
