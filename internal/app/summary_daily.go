package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

/*
========================
Daily Summary (with --force)
+ User Fact Extraction
========================
*/

func ensureDaily(cfg Config, db *sql.DB, date string, force bool) error {
	// ---------- FORCE MODE ----------
	if force {
		_, _ = db.Exec(`
			DELETE FROM embeddings
			WHERE summary_id IN (
				SELECT id FROM summaries
				WHERE type='daily' AND period_key=?
			)
		`, date)

		_, _ = db.Exec(`
			DELETE FROM summaries
			WHERE type='daily' AND period_key=?
		`, date)

		_ = os.Remove(filepath.Join(cfg.LogDir, date+".daily.json"))
	}

	// ---------- IDEMPOTENT CHECK ----------
	if !force {
		if ok, _ := summaryExists(db, "daily", date); ok {
			return nil
		}
	}

	logPath := filepath.Join(cfg.LogDir, date+".jsonl")
	info, err := os.Stat(logPath)
	if err != nil || info.Size() == 0 {
		return nil
	}

	// ---------- READ TRANSCRIPT ----------
	var transcript []byte
	if info.Size() > cfg.MaxDailyJSONLBytes {
		transcript, _ = readTailBytes(logPath, cfg.MaxDailyJSONLBytes)
	} else {
		transcript, err = os.ReadFile(logPath)
		if err != nil {
			return err
		}
	}

	// ---------- DAILY PROMPT ----------
	prompt := mustReadPrompt(cfg, "daily.txt")
	prompt = strings.ReplaceAll(prompt, "{{DATE}}", date)
	prompt = strings.ReplaceAll(prompt, "{{TRANSCRIPT}}", string(transcript))

	llmOut, err := callLLMNonStream(prompt)
	if err != nil {
		return err
	}

	// ---------- USER FACT EXTRACTION（关键新增） ----------
	rawLines, _ := loadRawLinesForDate(cfg, date)
	userFacts := ExtractUserFactsFromRaw(rawLines)

	out := buildDailyFinal(llmOut, userFacts)

	if len(userFacts) > 0 {
		var b strings.Builder
		b.WriteString("\n\n【用户事实】\n")
		for _, f := range userFacts {
			b.WriteString("- ")
			b.WriteString(f)
			b.WriteString("\n")
		}
		out += b.String()
	}

	// ---------- WRITE DAILY FILE ----------
	outPath := filepath.Join(cfg.LogDir, date+".daily.json")
	_ = os.WriteFile(outPath, []byte(out), 0644)

	// ---------- INDEX + DB ----------
	indexText := extractIndexText(out)

	_, err = upsertSummary(
		db,
		cfg,
		"daily",
		date,
		date,
		date,
		out,
		indexText,
		logPath,
	)
	if err != nil {
		return err
	}

	// ---------- EMBEDDING ----------
	_ = ensureEmbedding(db, cfg, indexText, "daily", date)

	return nil
}

/*
========================
Helpers
========================
*/

// 读取某一天的 raw 对话
func loadRawLinesForDate(cfg Config, date string) ([]RawLine, error) {
	path := filepath.Join(cfg.LogDir, date+".jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lines []RawLine
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var r RawLine
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			lines = append(lines, r)
		}
	}

	return lines, nil
}

func buildDailyFinal(llmJSON string, userFacts []string) string {
	if len(userFacts) == 0 {
		return llmJSON
	}

	var b strings.Builder
	b.WriteString(llmJSON)
	b.WriteString("\n\n【用户事实（来自用户明确表述）】\n")

	for _, f := range userFacts {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}

	return b.String()
}
