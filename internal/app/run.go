package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

/*
================================================
RUN MODE SWITCH（唯一需要改的地方）
------------------------------------------------
true  = 默认聊天即“长期记忆自我”（推荐）
false = 默认聊天仅即时回答（streamChat）
================================================
*/
const DefaultUseLongTermChat = true

// ==============================
// Run（最终 UX 版）
// ==============================
func Run() {
	// ------------------------------
	// 0️⃣ 初始化
	// ------------------------------
	cfg := defaultConfig()
	mustEnsureDirs(cfg)
	mustEnsurePromptFiles(cfg)

	db := mustOpenDB(cfg)
	defer db.Close()

	lw := NewLogWriter(cfg, db)
	defer lw.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🧠 Local AI Chat")
	fmt.Println("Type exit to quit, /help for commands")
	fmt.Println()

	// ==============================
	// 1️⃣ 主循环
	// ==============================
	for {
		fmt.Print("You> ")

		line, err := readLine(reader)
		if err != nil {
			fmt.Println("\nbye")
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 统一退出
		if line == "exit" {
			return
		}

		// ------------------------------
		// 2️⃣ 命令模式（/xxx）
		// ------------------------------
		if strings.HasPrefix(line, "/") {
			handleCommand(cfg, db, lw, reader, line)
			fmt.Println("\n------------------\n")
			continue
		}

		// ------------------------------
		// 3️⃣ Markdown fence 多行
		// ------------------------------
		var input string
		if line == "```" {
			input, err = readUntilFence(reader)
			if err != nil {
				fmt.Println("input error:", err)
				fmt.Println("\n------------------\n")
				continue
			}
		} else {
			// 默认单行
			input = line
		}

		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("\n------------------\n")
			continue
		}

		// ------------------------------
		// 4️⃣ 默认聊天入口
		// ------------------------------
		fmt.Println("\nAssistant>")

		if DefaultUseLongTermChat {
			if err := Chat(lw, cfg, db, input); err != nil {
				fmt.Println("chat error:", err)
			}
		} else {
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

// ==============================
// 输入工具函数
// ==============================

// readLine：读取单行（canonical stdin）
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// readMultiline：空行提交（用于 /paste）
func readMultiline(r *bufio.Reader) (string, error) {
	var lines []string

	for {
		line, err := readLine(r)
		if err != nil {
			if len(lines) > 0 {
				break
			}
			return "", err
		}

		if strings.TrimSpace(line) == "" {
			break
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// readUntilFence：``` 结束
func readUntilFence(r *bufio.Reader) (string, error) {
	var lines []string

	for {
		line, err := readLine(r)
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(line) == "```" {
			break
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}
