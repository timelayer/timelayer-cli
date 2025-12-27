package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

/*
PromptBlock 表示一段被注入到 LLM Prompt 中的上下文块。
它描述的是「Prompt 组成」，而不是「记忆层级」。
*/
type PromptBlock struct {
	Role    string // system | user | assistant
	Source  string // daily_summary | search_hit | recent_raw
	Content string
}

// 构建 chat 上下文（被 Chat / DebugChat 行为调用）
// 注意：这里只负责“Prompt 组装”，不注入当前 user input
func BuildChatContext(
	cfg Config,
	db *sql.DB,
	date string,
	userQuestion string, // 保留参数，仅用于 search
) []PromptBlock {

	var ctx []PromptBlock

	// 1️⃣ 今日 daily summary（长期抽象，只注入一次）
	if daily := loadDailySummary(cfg, date); daily != "" {
		ctx = append(ctx, PromptBlock{
			Role:    "assistant",
			Source:  "daily_summary",
			Content: "这是今天的对话摘要：\n" + daily,
		})
	}

	// 2️⃣ 相似历史（长期记忆：embedding 命中，排除今天）
	hits, err := SearchWithScore(db, cfg, userQuestion)
	if err == nil && len(hits) > 0 {
		var b strings.Builder
		b.WriteString("这是你过去相关的问题和记录：\n")

		max := min(cfg.SearchTopK, len(hits))
		for i := 0; i < max; i++ {
			h := hits[i]

			// ❗关键：跳过今天的 daily（防止当天自反馈）
			if h.Type == "daily" && h.Date == date {
				continue
			}

			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(h.Text))
			b.WriteString("\n")
		}

		if b.Len() > 0 {
			ctx = append(ctx, PromptBlock{
				Role:    "assistant",
				Source:  "search_hit",
				Content: b.String(),
			})
		}
	}

	// 3️⃣ 最近 raw 对话（短期工作上下文）
	// ⚠️ 只保留 user，彻底阻断 assistant 风格回流
	if recent := loadRecentRaw(cfg, date, 20); recent != "" {
		ctx = append(ctx, PromptBlock{
			Role:    "assistant",
			Source:  "recent_raw",
			Content: "以下是最近的原始对话记录：\n" + recent,
		})
	}

	// ❌ 重要：不再注入当前 userQuestion
	// user input 只允许通过真正的 user message 进入模型

	return ctx
}

// ---------- helpers ----------

// 读取当日 daily summary（抽象层）
func loadDailySummary(cfg Config, date string) string {
	path := filepath.Join(cfg.LogDir, date+".daily.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// 读取最近 raw 对话（干净版）
func loadRecentRaw(cfg Config, date string, maxLines int) string {
	path := filepath.Join(cfg.LogDir, date+".jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(b), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	var out []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var m struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}

		// ✅ 只保留 user
		if m.Role == "user" {
			out = append(out, "用户："+strings.TrimSpace(m.Content))
		}
	}

	if len(out) == 0 {
		return ""
	}

	return strings.Join(out, "\n")
}
