package app

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DebugChat：打印与 Chat() 完全一致的 system prompt（不调用模型）
func DebugChat(cfg Config, db *sql.DB, input string) {
	now := time.Now().In(cfg.Location)
	date := now.Format("2006-01-02")

	// 与 Chat 使用完全相同的上下文构建
	blocks := BuildChatContext(cfg, db, date, input)

	var system strings.Builder

	// ===== 1️⃣ 系统事实（与 Chat 对齐）=====
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

	// ===== 2️⃣ 历史上下文（Prompt Blocks）=====
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

	// ===== 3️⃣ 打印 Debug 结果 =====
	fmt.Println("========== DEBUG CHAT ==========")
	fmt.Println("【User Input】")
	fmt.Println(input)
	fmt.Println()

	fmt.Println("【System Prompt（将以 system role 发送给模型）】")
	fmt.Println(system.String())

	fmt.Println("================================")
}
