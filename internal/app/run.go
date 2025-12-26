package app

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

/*
================================================
RUN MODE SWITCH（唯一需要改的地方）
------------------------------------------------
true  = 默认聊天即“长期记忆自我”（推荐 / 当前模式）
false = 默认聊天仅即时回答（streamChat，不进长期上下文）
================================================
*/
const DefaultUseLongTermChat = true

func Run() {
	// ------------------------------
	// 0️⃣ 基础初始化（配置 / 目录 / prompt）
	// ------------------------------
	cfg := defaultConfig()
	mustEnsureDirs(cfg)
	mustEnsurePromptFiles(cfg)

	// ------------------------------
	// 1️⃣ 数据库 & 日志系统
	// ------------------------------
	db := mustOpenDB(cfg)
	defer db.Close()

	lw := NewLogWriter(cfg, db)
	defer lw.Close()

	// ------------------------------
	// 2️⃣ CLI 输入
	// ------------------------------
	rl, _ := readline.New("You> ")
	defer rl.Close()

	fmt.Println("🧠 Local AI Chat")
	fmt.Println("Type exit to quit, /help for commands")

	// ==============================
	// 3️⃣ 主循环
	// ==============================
	for {
		line, err := rl.Readline()
		if err != nil {
			return
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// 统一退出
		if input == "exit" {
			return
		}

		// ------------------------------
		// 4️⃣ 命令模式（/ask /chat /weekly /monthly …）
		// ------------------------------
		// 命令永远走 handleCommand，不受 DefaultUseLongTermChat 影响
		if strings.HasPrefix(input, "/") {
			handleCommand(cfg, db, lw, line)
			continue
		}

		// ------------------------------
		// 5️⃣ 默认聊天入口（关键语义分流点）
		// ------------------------------
		fmt.Println("\nAssistant>")

		if DefaultUseLongTermChat {
			/*
				长期记忆自我（推荐模式）

				语义：
				- 每次输入都会：
				  1) 写 user raw
				  2) 构建长期上下文（历史 / summary / embedding）
				  3) 使用 context 流式回答
				  4) 写 assistant raw

				注意：
				- Chat() 内部已经调用 streamChatWithContext()
				- 这里【绝对不要】再调用 streamChat()
			*/
			if err := Chat(lw, cfg, db, input); err != nil {
				fmt.Println("chat error:", err)
			}

		} else {
			/*
				即时自我（旧模式 / 轻量模式）

				语义：
				- 不使用长期上下文
				- 只做即时回答
				- 但仍然写 raw log（供 daily / weekly / monthly 使用）
			*/

			answer := streamChat(input)

			_ = lw.WriteRecord(map[string]string{
				"role":    "user",
				"content": input,
			})

			_ = lw.WriteRecord(map[string]string{
				"role":    "assistant",
				"content": answer,
			})
		}

		fmt.Println("\n------------------\n")
	}
}
