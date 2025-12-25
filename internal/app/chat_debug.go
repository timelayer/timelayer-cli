package app

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func DebugChat(cfg Config, db *sql.DB, input string) {
	date := time.Now().In(cfg.Location).Format("2006-01-02")

	// 构建上下文（和 Chat 使用完全相同的逻辑）
	blocks := BuildChatContext(cfg, db, date, input)

	var system strings.Builder
	system.WriteString("以下是用户的对话历史与已知事实：\n\n")

	for _, b := range blocks {
		system.WriteString(
			fmt.Sprintf(
				"---- Prompt Block (%s | %s) ----\n",
				b.Role,
				b.Source,
			),
		)
		system.WriteString(b.Content)
		system.WriteString("\n\n")
	}

	// 打印 system prompt
	fmt.Println("========== DEBUG CHAT ==========")
	fmt.Println("User Input:")
	fmt.Println(input)
	fmt.Println()

	fmt.Println("System Prompt:")
	fmt.Println(system.String())

	fmt.Println("================================")
}
