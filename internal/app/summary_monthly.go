package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
========================
Monthly Summary (with --force)
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

	// ---------- COLLECT WEEKLY SUMMARIES ----------
	weeklies := collectWeeklySummariesForMonth(cfg, monthKey)
	if len(weeklies) == 0 {
		return nil
	}

	// ---------- BUILD PROMPT ----------
	prompt := mustReadPrompt(cfg, "monthly.txt")
	prompt = strings.ReplaceAll(prompt, "{{MONTH}}", monthKey)
	prompt = strings.ReplaceAll(prompt, "{{WEEKLIES}}", strings.Join(weeklies, "\n\n"))

	// ---------- CALL LLM ----------
	out, err := callLLMNonStream(prompt)
	if err != nil {
		return err
	}

	// ---------- WRITE FILE ----------
	outPath := filepath.Join(cfg.LogDir, monthKey+".monthly.json")
	_ = os.WriteFile(outPath, []byte(out), 0644)

	// ---------- INDEX + DB ----------
	indexText := extractIndexText(out)

	_, err = upsertSummary(
		db,
		cfg,
		"monthly",
		monthKey,
		monthKey,
		monthKey,
		out,
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
		weekKey := formatWeekKey(y, w)

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

func formatWeekKey(year, week int) string {
	return fmt.Sprintf("%04d-W%02d", year, week)
}
