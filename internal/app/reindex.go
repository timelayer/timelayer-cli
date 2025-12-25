package app

import (
	"database/sql"
	"fmt"
)

/*
========================
Reindex (Embedding Backfill)
========================
*/

func Reindex(db *sql.DB, cfg Config, typ string) error {
	var rows *sql.Rows
	var err error

	switch typ {
	case "daily", "weekly", "monthly":
		rows, err = db.Query(`
			SELECT id, type, period_key, json
			FROM summaries
			WHERE type = ?
			ORDER BY period_key
		`, typ)

	case "all":
		rows, err = db.Query(`
			SELECT id, type, period_key, json
			FROM summaries
			ORDER BY type, period_key
		`)

	default:
		return fmt.Errorf("unknown reindex type: %s", typ)
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		total   int
		created int
		skipped int
	)

	for rows.Next() {
		var (
			id  int64
			sty string
			key string
			js  string
		)
		if err := rows.Scan(&id, &sty, &key, &js); err != nil {
			continue
		}
		total++

		// 如果已有 embedding，跳过
		if hasEmbedding(db, id, embedModel) {
			skipped++
			continue
		}

		// 从 JSON 动态提取 indexText
		indexText := extractIndexText(js)
		if indexText == "" {
			skipped++
			continue
		}

		err := ensureEmbedding(db, cfg, indexText, sty, key)
		if err != nil {
			fmt.Printf("[warn] failed to embed %s %s: %v\n", sty, key, err)
			continue
		}

		fmt.Printf("[ok] embedded %s %s\n", sty, key)
		created++
	}

	fmt.Printf(
		"[reindex done] total=%d created=%d skipped=%d\n",
		total, created, skipped,
	)

	return nil
}
