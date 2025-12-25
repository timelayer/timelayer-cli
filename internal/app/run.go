package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

func Run() {
	cfg := defaultConfig()
	mustEnsureDirs(cfg)
	mustEnsurePromptFiles(cfg)

	db := mustOpenDB(cfg)
	defer db.Close()

	lw := NewLogWriter(cfg, db)
	defer lw.Close()

	rl, _ := readline.New("You> ")
	defer rl.Close()

	fmt.Println("🧠 Local AI Chat")
	fmt.Println("Type exit to quit, /help for commands")

	for {
		line, err := rl.Readline()
		if err != nil {
			return
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		if input == "exit" {
			return
		}

		if strings.HasPrefix(input, "/") {
			handleCommand(cfg, db, lw, line)
			continue
		}

		fmt.Println("\nAssistant>")
		ans := streamChat(input)
		fmt.Println("\n------------------\n")

		_ = lw.WriteRecord(map[string]string{
			"time":     time.Now().Format(time.RFC3339),
			"question": input,
			"answer":   ans,
		})
	}
}
