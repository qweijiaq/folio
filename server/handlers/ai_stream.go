package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gin-gonic/gin"
)

func sseEvent(c *gin.Context, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
	c.Writer.Flush()
}

func qaErrorMessage(prepared qaPrepared, err error) string {
	message := prepared.Provider.Name + " 请求失败，请检查你的 API Key、额度和网络"
	var statusErr qaStatusError
	if errors.As(err, &statusErr) {
		message = prepared.Provider.Name + "：" + qaUpstreamError(statusErr.status)
	}
	return message
}

// AIAskStream 用 SSE 先返回检索到的来源，再流式输出回答。
func (a *API) AIAskStream(c *gin.Context) {
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

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	sseEvent(c, "meta", gin.H{
		"sources":   prepared.Sources,
		"provider":  prepared.Provider.ID,
		"model":     prepared.Model,
		"mode":      prepared.Mode,
		"retrieved": len(prepared.Sources),
	})

	if len(prepared.Sources) == 0 && prepared.Mode == "strict" {
		answer := qaStrictNoSourceAnswer()
		sseEvent(c, "delta", gin.H{"text": answer})
		sseEvent(c, "done", gin.H{"answer": answer, "retrieved": 0})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), qaTimeout)
	defer cancel()
	var answer string
	var err error
	if prepared.Provider.APIStyle == "anthropic" {
		answer, err = a.streamAnthropicQA(ctx, c, prepared)
	} else {
		answer, err = a.streamOpenAICompatQA(ctx, c, prepared)
	}
	if err != nil {
		log.Printf("AI 流式问答失败 provider=%s model=%s: %v", prepared.Provider.ID, prepared.Model, err)
		sseEvent(c, "error", gin.H{"error": qaErrorMessage(prepared, err)})
		return
	}
	if strings.TrimSpace(answer) == "" {
		sseEvent(c, "error", gin.H{"error": prepared.Provider.Name + " 没有返回有效回答"})
		return
	}
	sseEvent(c, "done", gin.H{"answer": strings.TrimSpace(answer), "retrieved": len(prepared.Sources)})
}

func (a *API) streamAnthropicQA(ctx context.Context, c *gin.Context, prepared qaPrepared) (string, error) {
	options := []option.RequestOption{option.WithAPIKey(prepared.APIKey)}
	if prepared.Provider.BaseURL != "" {
		options = append(options, option.WithBaseURL(prepared.Provider.BaseURL))
	}
	client := anthropic.NewClient(options...)
	messages := make([]anthropic.MessageParam, 0, len(prepared.History)+1)
	for _, item := range prepared.History {
		if item.Role == "assistant" {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(item.Content)))
		} else {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(item.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(prepared.UserPrompt)))
	stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(prepared.Model),
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: prepared.SystemPrompt}},
		Messages:  messages,
	})
	defer stream.Close()
	var answer strings.Builder
	for stream.Next() {
		event := stream.Current()
		if deltaEvent, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if textDelta, ok := deltaEvent.Delta.AsAny().(anthropic.TextDelta); ok && textDelta.Text != "" {
				answer.WriteString(textDelta.Text)
				sseEvent(c, "delta", gin.H{"text": textDelta.Text})
			}
		}
	}
	if err := stream.Err(); err != nil {
		return answer.String(), err
	}
	return answer.String(), nil
}

func (a *API) streamOpenAICompatQA(ctx context.Context, c *gin.Context, prepared qaPrepared) (string, error) {
	messages := make([]map[string]string, 0, len(prepared.History)+2)
	messages = append(messages, map[string]string{"role": "system", "content": prepared.SystemPrompt})
	for _, item := range prepared.History {
		messages = append(messages, map[string]string{"role": item.Role, "content": item.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prepared.UserPrompt})
	payload := map[string]any{
		"model":       prepared.Model,
		"messages":    messages,
		"temperature": 0.3,
		"max_tokens":  4096,
		"stream":      true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(prepared.Provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+prepared.APIKey)
	req.Header.Set("User-Agent", "folio-ai-qa/1.0")
	resp, err := qaHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", qaStatusError{status: resp.StatusCode}
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var answer strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			if data == "[DONE]" {
				break
			}
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return answer.String(), fmt.Errorf("%s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		answer.WriteString(delta)
		sseEvent(c, "delta", gin.H{"text": delta})
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), err
	}
	return answer.String(), nil
}
