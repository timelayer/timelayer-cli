package app

import (
	"os"
	"path/filepath"
	"time"
)

const (
	chatURL  = "http://localhost:8080/v1/chat/completions"
	embedURL = "http://localhost:11434/v1/embeddings"

	chatModel  = "qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf"
	embedModel = "nomic-embed-text"
)

type Config struct {
	BaseDir            string
	LogDir             string
	ArchiveDir         string
	PromptDir          string
	DBPath             string
	Location           *time.Location
	KeepRawDays        int
	MaxDailyJSONLBytes int64
	HTTPTimeout        time.Duration
	SearchTopK         int
	SearchMinScore     float64
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, "local-ai")
	loc := time.Local // ✅ 使用系统时区

	return Config{
		BaseDir:            base,
		LogDir:             filepath.Join(base, "logs"),
		ArchiveDir:         filepath.Join(base, "logs", "archive"),
		PromptDir:          filepath.Join(base, "prompts"),
		DBPath:             filepath.Join(base, "memory", "memory.sqlite"),
		Location:           loc,
		KeepRawDays:        45,
		MaxDailyJSONLBytes: 25 * 1024 * 1024, // 25MB
		HTTPTimeout:        60 * time.Second,
		SearchTopK:         5,
		SearchMinScore:     0.00,
	}
}
