package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rivo/uniseg"
)

type SSEChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// 本地默认超时：避免 CLI “卡死”
// 不依赖 cfg 字段，确保你现有项目可直接编译运行
var chatHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
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

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("marshal error:", err)
		return ""
	}

	req, err := http.NewRequest("POST", chatURL, bytes.NewReader(body))
	if err != nil {
		fmt.Println("new request error:", err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := chatHTTPClient.Do(req)
	if err != nil {
		fmt.Println("request error:", err)
		return ""
	}
	defer resp.Body.Close()

	// ✅ 必须检查 HTTP 状态，否则 401/500 会表现为“空输出 / 卡住”
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("http error: %d\n%s\n", resp.StatusCode, strings.TrimSpace(string(b)))
		return ""
	}

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
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		var chunk SSEChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			// SSE 中偶尔有非 JSON 行，忽略即可
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

		// ✅ 修复：代码块状态要随 ``` 切换
		updateCodeBlockState(text, &inCodeBlock)

		render(text, &inCodeBlock)
	}

	// ✅ scanner.Err() 不处理会导致“中途断流”你完全不知道
	if err := scanner.Err(); err != nil {
		fmt.Println("\nstream error:", err)
	}

	return full.String()
}

// 根据文本里的 ``` 出现次数切换 code block 状态（出现奇数次就 toggle）
func updateCodeBlockState(text string, inCodeBlock *bool) {
	n := strings.Count(text, "```")
	if n%2 == 1 {
		*inCodeBlock = !*inCodeBlock
	}
}

func render(text string, inCodeBlock *bool) {
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		ch := gr.Str()
		fmt.Print(ch)

		// 非代码块时，中文句号/问号/叹号自动换行
		if !*inCodeBlock {
			switch ch {
			case "。", "！", "？":
				fmt.Print("\n")
			}
		}
	}
}
