package app

import (
	"database/sql"
	"strings"
	"time"
)

// Chat = 对话行为入口（唯一！）
func Chat(lw *LogWriter, cfg Config, db *sql.DB, input string) error {
	// === 0️⃣ 系统时间（权威事实来源） ===
	now := time.Now().In(cfg.Location)

	// === 1️⃣ 写 user raw ===
	_ = lw.WriteRecord(map[string]string{
		"role":    "user",
		"content": input,
	})

	// === 2️⃣ 构建上下文（历史 / 事实） ===
	// 注意：这里的 BuildChatContext 里不要再注入 user input（否则会重复一次）
	date := now.Format("2006-01-02")
	blocks := BuildChatContext(cfg, db, date, input)

	// === 3️⃣ 构建 system prompt ===
	var system strings.Builder

	// --- 系统事实（时间）---
	system.WriteString("【系统事实（权威）】\n")
	system.WriteString("当前日期：")
	system.WriteString(now.Format("2006-01-02"))
	system.WriteString("\n")

	system.WriteString("当前时间：")
	system.WriteString(now.Format("15:04:05"))
	system.WriteString("\n")

	system.WriteString("星期：")
	system.WriteString(now.Weekday().String())
	system.WriteString("\n")

	system.WriteString("时区：")
	system.WriteString(now.Location().String())
	system.WriteString("\n\n")

	system.WriteString(
		"以上时间信息来自系统，是准确且可信的事实。\n" +
			"涉及日期、时间、星期的问题，请直接基于这些事实回答，不允许猜测或自行推断。\n\n",
	)

	// --- 原有 system 说明 ---
	system.WriteString("以下是用户的对话历史与已知事实，请严格基于这些信息回答。\n\n")

	for _, b := range blocks {
		system.WriteString(b.Content)
		system.WriteString("\n\n")
	}

	// === 4️⃣ 调用流式 chat ===
	answer := streamChatWithContext(
		system.String(),
		nil,
		input,
	)

	// === 5️⃣ 写 assistant raw ===
	_ = lw.WriteRecord(map[string]string{
		"role":    "assistant",
		"content": answer,
	})

	return nil
}
