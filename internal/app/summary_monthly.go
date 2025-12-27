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
Monthly Summary (FINAL)
- Weekly JSON slimming
- Chunk + merge
========================
periodKey = YYYY-MM
*/

func ensureMonthly(cfg Config, db *sql.DB, monthKey string, force bool) error {
	// ---------- FORCE MODE ----------
	if force {
		_, _ = db.Exec(`
			DELETE FROM embeddings
			WHERE summary_id IN (
				SELECT id FROM summaries
				WHERE type='monthly' AND period_key=?
			)
		`, monthKey)

		_, _ = db.Exec(`
			DELETE FROM summaries
			WHERE type='monthly' AND period_key=?
		`, monthKey)

		_ = os.Remove(filepath.Join(cfg.LogDir, monthKey+".monthly.json"))
	}

	// ---------- IDEMPOTENT CHECK ----------
	if !force {
		if ok, _ := summaryExists(db, "monthly", monthKey); ok {
			return nil
		}
	}

	// ---------- COLLECT WEEKLY ----------
	weeklies := collectWeeklySummariesForMonth(cfg, monthKey)
	if len(weeklies) == 0 {
		return nil
	}

	// ---------- MONTH RANGE ----------
	t, err := time.ParseInLocation("2006-01", monthKey, cfg.Location)
	if err != nil {
		return err
	}
	startT, endT := monthRange(t, cfg.Location)
	monthStart := startT.Format("2006-01-02")
	monthEnd := endT.Format("2006-01-02")

	// ---------- SLIM WEEKLY JSON ----------
	// monthly 只需要 trajectory / themes / wins / losses / improvements
	slimmed := make([]map[string]any, 0, len(weeklies))
	for _, s := range weeklies {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !json.Valid([]byte(s)) {
			return fmt.Errorf("monthly refused: weekly invalid JSON")
		}

		var obj map[string]any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return fmt.Errorf("monthly weekly unmarshal failed: %w", err)
		}

		slim := map[string]any{
			"week_start":         obj["week_start"],
			"week_end":           obj["week_end"],
			"themes":             obj["themes"],
			"progress":           obj["progress"],
			"recurring_blockers": obj["recurring_blockers"],
			"notable_decisions":  obj["notable_decisions"],
			"next_week_focus":    obj["next_week_focus"],
		}
		slimmed = append(slimmed, slim)
	}

	rawBytes, err := json.Marshal(slimmed)
	if err != nil {
		return fmt.Errorf("monthly marshal slimmed weeklies failed: %w", err)
	}

	// ---------- SPLIT IF NEEDED ----------
	chunks := splitJSONBytes(rawBytes, cfg.MaxDailyJSONLBytes)

	var monthlyJSON string

	if len(chunks) == 1 {
		prompt := mustReadPrompt(cfg, "monthly.txt")
		prompt = strings.ReplaceAll(prompt, "{{MONTH}}", monthKey)
		prompt = strings.ReplaceAll(prompt, "{{MONTH_START}}", monthStart)
		prompt = strings.ReplaceAll(prompt, "{{MONTH_END}}", monthEnd)
		prompt = strings.ReplaceAll(prompt, "{{WEEKLY_JSON_ARRAY}}", string(chunks[0]))

		out, err := callLLMNonStream(prompt)
		if err != nil {
			return err
		}
		out = strings.TrimSpace(out)
		if out == "" {
			return fmt.Errorf("monthly llm output is empty")
		}
		if !json.Valid([]byte(out)) {
			return fmt.Errorf("monthly llm output invalid JSON\nraw:\n%s", out)
		}
		monthlyJSON = out
	} else {
		partials := make([]string, 0, len(chunks))

		for i, c := range chunks {
			prompt := mustReadPrompt(cfg, "monthly.txt")
			prompt = strings.ReplaceAll(prompt, "{{MONTH}}", monthKey)
			prompt = strings.ReplaceAll(prompt, "{{MONTH_START}}", monthStart)
			prompt = strings.ReplaceAll(prompt, "{{MONTH_END}}", monthEnd)
			prompt = strings.ReplaceAll(
				prompt,
				"{{WEEKLY_JSON_ARRAY}}",
				fmt.Sprintf("/* PART %d/%d */\n%s", i+1, len(chunks), string(c)),
			)

			out, err := callLLMNonStream(prompt)
			if err != nil {
				return err
			}
			out = strings.TrimSpace(out)
			if out == "" {
				return fmt.Errorf("monthly chunk %d empty", i+1)
			}
			if !json.Valid([]byte(out)) {
				return fmt.Errorf("monthly chunk %d invalid JSON\nraw:\n%s", i+1, out)
			}
			partials = append(partials, out)
		}

		mergePrompt := buildMonthlyMergePrompt(monthKey, monthStart, monthEnd, partials)
		merged, err := callLLMNonStream(mergePrompt)
		if err != nil {
			return err
		}
		merged = strings.TrimSpace(merged)
		if merged == "" {
			return fmt.Errorf("monthly merged output empty")
		}
		if !json.Valid([]byte(merged)) {
			return fmt.Errorf("monthly merged output invalid JSON\nraw:\n%s", merged)
		}
		monthlyJSON = merged
	}

	// ---------- WRITE FILE ----------
	outPath := filepath.Join(cfg.LogDir, monthKey+".monthly.json")
	if err := os.WriteFile(outPath, []byte(monthlyJSON), 0644); err != nil {
		return err
	}

	// ---------- INDEX + DB ----------
	indexText := extractIndexText(monthlyJSON)

	_, err = upsertSummary(
		db,
		cfg,
		"monthly",
		monthKey,
		monthStart,
		monthEnd,
		monthlyJSON,
		indexText,
		outPath,
	)
	if err != nil {
		return err
	}

	// ---------- EMBEDDING ----------
	_ = ensureEmbedding(db, cfg, indexText, "monthly", monthKey)

	return nil
}

/*
========================
Helpers
========================
*/

func collectWeeklySummariesForMonth(cfg Config, monthKey string) []string {
	t, err := time.ParseInLocation("2006-01", monthKey, cfg.Location)
	if err != nil {
		return nil
	}

	start, end := monthRange(t, cfg.Location)

	seen := make(map[string]bool)
	var out []string

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		y, w := d.ISOWeek()
		weekKey := fmt.Sprintf("%04d-W%02d", y, w)

		if seen[weekKey] {
			continue
		}
		seen[weekKey] = true

		path := filepath.Join(cfg.LogDir, weekKey+".weekly.json")
		if b, err := os.ReadFile(path); err == nil {
			out = append(out, strings.TrimSpace(string(b)))
		}
	}
	return out
}

func buildMonthlyMergePrompt(monthKey, monthStart, monthEnd string, partials []string) string {
	var b strings.Builder

	b.WriteString("You are a strict monthly summary reducer.\n")
	b.WriteString("Merge multiple partial monthly summaries into ONE final monthly summary.\n\n")

	b.WriteString("CRITICAL RULES:\n")
	b.WriteString("- Output JSON only.\n")
	b.WriteString("- Do NOT add new facts.\n")
	b.WriteString("- Do NOT infer user identity.\n")
	b.WriteString("- Deduplicate and merge semantically.\n\n")

	b.WriteString("OUTPUT FORMAT (JSON only):\n")
	b.WriteString("{\n")
	b.WriteString(`  "type": "monthly",` + "\n")
	b.WriteString(fmt.Sprintf(`  "month": "%s",`+"\n", monthKey))
	b.WriteString(fmt.Sprintf(`  "month_start": "%s",`+"\n", monthStart))
	b.WriteString(fmt.Sprintf(`  "month_end": "%s",`+"\n", monthEnd))
	b.WriteString(`  "trajectory": [],` + "\n")
	b.WriteString(`  "top_themes": [],` + "\n")
	b.WriteString(`  "wins": [],` + "\n")
	b.WriteString(`  "losses": [],` + "\n")
	b.WriteString(`  "systems_improvements": [],` + "\n")
	b.WriteString(`  "next_month_bets": []` + "\n")
	b.WriteString("}\n\n")

	b.WriteString("PARTIAL MONTHLY SUMMARIES:\n")
	for i, p := range partials {
		b.WriteString(fmt.Sprintf("\n--- PART %d/%d ---\n", i+1, len(partials)))
		b.WriteString(strings.TrimSpace(p))
		b.WriteString("\n")
	}

	return b.String()
}
