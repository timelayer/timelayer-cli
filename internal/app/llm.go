package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var llmHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
}

func callLLMNonStream(prompt string) (string, error) {
	payload := map[string]any{
		"model": chatModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", chatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// ✅ 1. 先检查 HTTP 状态码
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return "", fmt.Errorf("llm http error %d: %s", resp.StatusCode, msg)
	}

	// ✅ 2. 解析完整结构（choices + error）
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"` // 兼容部分实现
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &r); err != nil {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return "", fmt.Errorf("llm decode failed: %v; body=%s", err, msg)
	}

	// ✅ 3. 显式处理 error 字段
	if r.Error != nil && strings.TrimSpace(r.Error.Message) != "" {
		return "", fmt.Errorf("llm error: %s", strings.TrimSpace(r.Error.Message))
	}

	// ✅ 4. 正常 choices
	if len(r.Choices) == 0 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		return "", fmt.Errorf("no choices; body=%s", msg)
	}

	// 标准 OpenAI 格式
	if c := strings.TrimSpace(r.Choices[0].Message.Content); c != "" {
		return c, nil
	}

	// 兼容 text 格式
	if t := strings.TrimSpace(r.Choices[0].Text); t != "" {
		return t, nil
	}

	return "", fmt.Errorf("empty content in choices")
}
