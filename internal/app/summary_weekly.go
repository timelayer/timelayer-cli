package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
========================
Weekly Summary (FINAL)
- Daily JSON slimming
- Chunked weekly if needed
========================
periodKey = YYYY-Www
*/

func ensureWeekly(cfg Config, db *sql.DB, weekKey string, force bool) error {
	// ---------- FORCE MODE ----------
	if force {
		_, _ = db.Exec(`
			DELETE FROM embeddings
			WHERE summary_id IN (
				SELECT id FROM summaries
				WHERE type='weekly' AND period_key=?
			)
		`, weekKey)

		_, _ = db.Exec(`
			DELETE FROM summaries
			WHERE type='weekly' AND period_key=?
		`, weekKey)

		_ = os.Remove(filepath.Join(cfg.LogDir, weekKey+".weekly.json"))
	}

	// ---------- IDEMPOTENT CHECK ----------
	if !force {
		if ok, _ := summaryExists(db, "weekly", weekKey); ok {
			return nil
		}
	}

	// ---------- COLLECT DAILY ----------
	dailies := collectDailySummariesForWeek(cfg, weekKey)
	if len(dailies) == 0 {
		return nil
	}

	// ---------- WEEK RANGE ----------
	year, week := parseWeekKey(weekKey)

	ref := time.Date(year, 1, 4, 0, 0, 0, 0, cfg.Location)
	for {
		y, w := ref.ISOWeek()
		if y == year && w == week {
			break
		}
		ref = ref.AddDate(0, 0, 1)
	}
	startT, endT := weekRange(ref, cfg.Location)
	weekStart := startT.Format("2006-01-02")
	weekEnd := endT.Format("2006-01-02")

	// ---------- SLIM DAILY JSON ----------
	// weekly 不需要吃 full daily json（那样会炸 token）
	// 只保留 weekly 真正需要的字段，语义不损失（因为 weekly 目标就是趋势/模式）
	slimmed := make([]map[string]any, 0, len(dailies))
	for _, s := range dailies {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !json.Valid([]byte(s)) {
			return fmt.Errorf("weekly refused: daily summary invalid JSON")
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return fmt.Errorf("weekly refused: daily json unmarshal failed: %w", err)
		}

		slim := map[string]any{
			"date":           obj["date"],
			"topics":         obj["topics"],
			"patterns":       obj["patterns"],
			"open_questions": obj["open_questions"],
			"highlights":     obj["highlights"],
			"lowlights":      obj["lowlights"],
		}
		slimmed = append(slimmed, slim)
	}

	rawBytes, err := json.Marshal(slimmed)
	if err != nil {
		return fmt.Errorf("weekly marshal slimmed dailies failed: %w", err)
	}

	// ---------- CHUNK IF NEEDED ----------
	// 这里的 maxBytes 用 cfg.MaxDailyJSONLBytes 复用即可：它本质是“给 LLM 的安全上限”
	chunks := splitJSONBytes(rawBytes, cfg.MaxDailyJSONLBytes)

	var weeklyJSON string

	if len(chunks) == 1 {
		prompt := mustReadPrompt(cfg, "weekly.txt")
		prompt = strings.ReplaceAll(prompt, "{{WEEK_START}}", weekStart)
		prompt = strings.ReplaceAll(prompt, "{{WEEK_END}}", weekEnd)
		prompt = strings.ReplaceAll(prompt, "{{DAILY_JSON_ARRAY}}", string(chunks[0]))

		out, err := callLLMNonStream(prompt)
		if err != nil {
			return err
		}
		out = strings.TrimSpace(out)
		if out == "" {
			return fmt.Errorf("weekly llm output is empty")
		}
		if !json.Valid([]byte(out)) {
			return fmt.Errorf("weekly llm output is not valid JSON\nraw:\n%s", out)
		}
		weeklyJSON = out
	} else {
		partials := make([]string, 0, len(chunks))

		for i, c := range chunks {
			prompt := mustReadPrompt(cfg, "weekly.txt")
			prompt = strings.ReplaceAll(prompt, "{{WEEK_START}}", weekStart)
			prompt = strings.ReplaceAll(prompt, "{{WEEK_END}}", weekEnd)

			// 每次只给一部分 daily-array
			prompt = strings.ReplaceAll(
				prompt,
				"{{DAILY_JSON_ARRAY}}",
				fmt.Sprintf("/* PART %d/%d */\n%s", i+1, len(chunks), string(c)),
			)

			out, err := callLLMNonStream(prompt)
			if err != nil {
				return err
			}
			out = strings.TrimSpace(out)
			if out == "" {
				return fmt.Errorf("weekly chunk %d output is empty", i+1)
			}
			if !json.Valid([]byte(out)) {
				return fmt.Errorf("weekly chunk %d output invalid JSON\nraw:\n%s", i+1, out)
			}
			partials = append(partials, out)
		}

		mergePrompt := buildWeeklyMergePrompt(weekKey, weekStart, weekEnd, partials)
		merged, err := callLLMNonStream(mergePrompt)
		if err != nil {
			return err
		}
		merged = strings.TrimSpace(merged)
		if merged == "" {
			return fmt.Errorf("weekly merged output is empty")
		}
		if !json.Valid([]byte(merged)) {
			return fmt.Errorf("weekly merged output invalid JSON\nraw:\n%s", merged)
		}
		weeklyJSON = merged
	}

	// ---------- WRITE FILE ----------
	outPath := filepath.Join(cfg.LogDir, weekKey+".weekly.json")
	if err := os.WriteFile(outPath, []byte(weeklyJSON), 0644); err != nil {
		return err
	}

	// ---------- INDEX + DB ----------
	indexText := extractIndexText(weeklyJSON)

	_, err = upsertSummary(
		db,
		cfg,
		"weekly",
		weekKey,
		weekStart,
		weekEnd,
		weeklyJSON,
		indexText,
		outPath,
	)
	if err != nil {
		return err
	}

	// ---------- EMBEDDING ----------
	_ = ensureEmbedding(db, cfg, indexText, "weekly", weekKey)

	return nil
}

/*
========================
Helpers (ALL IN THIS FILE)
========================
*/

func parseWeekKey(weekKey string) (year int, week int) {
	// 支持 YYYY-Www
	fmt.Sscanf(weekKey, "%d-W%d", &year, &week)
	return
}

func collectDailySummariesForWeek(cfg Config, weekKey string) []string {
	year, week := parseWeekKey(weekKey)

	ref := time.Date(year, 1, 4, 0, 0, 0, 0, cfg.Location)
	for {
		y, w := ref.ISOWeek()
		if y == year && w == week {
			break
		}
		ref = ref.AddDate(0, 0, 1)
	}

	start, end := weekRange(ref, cfg.Location)

	var out []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		path := filepath.Join(cfg.LogDir, dateKey+".daily.json")
		if b, err := os.ReadFile(path); err == nil {
			out = append(out, strings.TrimSpace(string(b)))
		}
	}
	return out
}

// splitJSONBytes：把一个大的 JSON 字节串切成多个 chunk，每个 <= maxBytes。
// 注意：为了稳定性，这里按“对象级”切分（外层必须是 JSON array）。
func splitJSONBytes(arrJSON []byte, maxBytes int64) [][]byte {
	if maxBytes <= 0 {
		return [][]byte{arrJSON}
	}
	if int64(len(arrJSON)) <= maxBytes {
		return [][]byte{arrJSON}
	}

	// 外层必须是 array
	var items []json.RawMessage
	if err := json.Unmarshal(arrJSON, &items); err != nil || len(items) == 0 {
		// 兜底：无法解析时，直接硬切（仍然保证不会 OOM，只是不保证语义）
		return hardSplitBytes(arrJSON, maxBytes)
	}

	var chunks [][]byte
	var cur []json.RawMessage
	var curSize int64 = 2 // for "[]"

	flush := func() {
		if len(cur) == 0 {
			return
		}
		b, _ := json.Marshal(cur)
		chunks = append(chunks, b)
		cur = nil
		curSize = 2
	}

	for _, it := range items {
		itSize := int64(len(it))
		// 逗号 + 空间
		add := itSize
		if len(cur) > 0 {
			add += 1
		}

		if len(cur) > 0 && curSize+add > maxBytes {
			flush()
		}

		cur = append(cur, it)
		curSize += add

		// 极端：单个 item 就超过 maxBytes
		if int64(len(it)) > maxBytes {
			flush()
			chunks = append(chunks, []byte("["+string(it)+"]"))
		}
	}

	flush()

	if len(chunks) == 0 {
		return [][]byte{arrJSON}
	}
	return chunks
}

func hardSplitBytes(b []byte, maxBytes int64) [][]byte {
	if maxBytes <= 0 {
		return [][]byte{b}
	}
	var out [][]byte
	for i := int64(0); i < int64(len(b)); i += maxBytes {
		end := i + maxBytes
		if end > int64(len(b)) {
			end = int64(len(b))
		}
		out = append(out, b[i:end])
	}
	return out
}

func buildWeeklyMergePrompt(weekKey, weekStart, weekEnd string, partials []string) string {
	var b strings.Builder

	b.WriteString("You are a strict weekly summary reducer.\n")
	b.WriteString("Merge multiple partial weekly summaries into ONE final weekly summary.\n\n")

	b.WriteString("CRITICAL RULES:\n")
	b.WriteString("- Output JSON only.\n")
	b.WriteString("- Do NOT add new facts.\n")
	b.WriteString("- Do NOT infer user identity.\n")
	b.WriteString("- Deduplicate and merge semantically.\n\n")

	b.WriteString("OUTPUT FORMAT (JSON only):\n")
	b.WriteString("{\n")
	b.WriteString(`  "type": "weekly",` + "\n")
	b.WriteString(fmt.Sprintf(`  "week_key": "%s",`+"\n", weekKey))
	b.WriteString(fmt.Sprintf(`  "week_start": "%s",`+"\n", weekStart))
	b.WriteString(fmt.Sprintf(`  "week_end": "%s",`+"\n", weekEnd))
	b.WriteString(`  "themes": [],` + "\n")
	b.WriteString(`  "progress": [],` + "\n")
	b.WriteString(`  "recurring_blockers": [],` + "\n")
	b.WriteString(`  "notable_decisions": [],` + "\n")
	b.WriteString(`  "next_week_focus": []` + "\n")
	b.WriteString("}\n\n")

	b.WriteString("PARTIAL WEEKLY SUMMARIES:\n")
	for i, p := range partials {
		b.WriteString(fmt.Sprintf("\n--- PART %d/%d ---\n", i+1, len(partials)))
		b.WriteString(strings.TrimSpace(p))
		b.WriteString("\n")
	}

	return b.String()
}
