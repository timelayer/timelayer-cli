package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rivo/uniseg"
)

type SSEChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// 保留原接口（无上下文）
func streamChat(question string) string {
	return streamChatWithContext("", nil, question)
}

// 带上下文的流式 chat
func streamChatWithContext(
	systemPrompt string,
	contextMessages []map[string]string,
	userQuestion string,
) string {

	messages := []map[string]string{}

	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	for _, m := range contextMessages {
		if m["role"] != "" && m["content"] != "" {
			messages = append(messages, m)
		}
	}

	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userQuestion,
	})

	payload := map[string]any{
		"model":    chatModel,
		"stream":   true,
		"messages": messages,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", chatURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("request error:", err)
		return ""
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var full strings.Builder
	inCodeBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "data: [DONE]" {
			break
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		raw := strings.TrimPrefix(line, "data: ")
		var chunk SSEChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		text := chunk.Choices[0].Delta.Content
		if text == "" {
			continue
		}

		full.WriteString(text)
		render(text, &inCodeBlock)
	}

	return full.String()
}

func render(text string, inCodeBlock *bool) {
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		ch := gr.Str()
		fmt.Print(ch)

		if !*inCodeBlock {
			switch ch {
			case "。", "！", "？":
				fmt.Print("\n")
			}
		}
	}
}
