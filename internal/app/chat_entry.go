// chat_entry.go
package app

import (
	"database/sql"
	"strings"
	"time"
)

// Chat = 对话行为入口（唯一！）
func Chat(lw *LogWriter, cfg Config, db *sql.DB, input string) error {
	date := time.Now().In(cfg.Location).Format("2006-01-02")

	// 1️⃣ 写 user raw
	_ = lw.WriteRecord(map[string]string{
		"role":    "user",
		"content": input,
	})

	// 2️⃣ 构建上下文（现在返回 []PromptBlock）
	blocks := BuildChatContext(cfg, db, date, input)

	// 3️⃣ 拼 system prompt
	var system strings.Builder
	system.WriteString("以下是用户的对话历史与已知事实，请严格基于这些信息回答。\n\n")

	for _, b := range blocks {
		system.WriteString(b.Content)
		system.WriteString("\n\n")
	}

	// 4️⃣ 调用“流式 chat 实现”
	answer := streamChatWithContext(
		system.String(),
		nil,
		input,
	)

	// 5️⃣ 写 assistant raw
	_ = lw.WriteRecord(map[string]string{
		"role":    "assistant",
		"content": answer,
	})

	return nil
}
