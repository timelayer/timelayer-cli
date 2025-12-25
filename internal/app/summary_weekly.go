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
Weekly Summary (with --force)
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

	// ---------- COLLECT DAILY SUMMARIES ----------
	dailies := collectDailySummariesForWeek(cfg, weekKey)
	if len(dailies) == 0 {
		return nil
	}

	// ---------- BUILD PROMPT ----------
	prompt := mustReadPrompt(cfg, "weekly.txt")
	prompt = strings.ReplaceAll(prompt, "{{WEEK}}", weekKey)
	prompt = strings.ReplaceAll(prompt, "{{DAILIES}}", strings.Join(dailies, "\n\n"))

	// ---------- CALL LLM ----------
	out, err := callLLMNonStream(prompt)
	if err != nil {
		return err
	}

	// ---------- WRITE FILE ----------
	outPath := filepath.Join(cfg.LogDir, weekKey+".weekly.json")
	_ = os.WriteFile(outPath, []byte(out), 0644)

	// ---------- INDEX + DB ----------
	indexText := extractIndexText(out)

	_, err = upsertSummary(
		db,
		cfg,
		"weekly",
		weekKey,
		weekKey,
		weekKey,
		out,
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
Helpers
========================
*/

func parseWeekKey(weekKey string) (year int, week int) {
	fmt.Sscanf(weekKey, "%d-W%d", &year, &week)
	return
}

func collectDailySummariesForWeek(cfg Config, weekKey string) []string {
	year, week := parseWeekKey(weekKey)

	// ISO 官方推荐：用 1 月 4 日作为基准，找到落在该 ISO 周的日期
	ref := time.Date(year, 1, 4, 0, 0, 0, 0, cfg.Location)
	for {
		y, w := ref.ISOWeek()
		if y == year && w == week {
			break
		}
		ref = ref.AddDate(0, 0, 1)
	}

	// 使用你已有的 weekRange（不要重复造）
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
