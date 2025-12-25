package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS summaries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,                 -- daily|weekly|monthly
  period_key TEXT NOT NULL,           -- daily: YYYY-MM-DD, weekly: YYYY-MM-DD..YYYY-MM-DD, monthly: YYYY-MM
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  json TEXT NOT NULL,
  text TEXT NOT NULL,                 -- 索引用文本
  source_path TEXT,
  created_at TEXT NOT NULL,
  UNIQUE(type, period_key)
);

CREATE TABLE IF NOT EXISTS embeddings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  summary_id INTEGER NOT NULL,
  model TEXT NOT NULL,
  dim INTEGER NOT NULL,
  vec BLOB NOT NULL,                  -- float32 little-endian
  l2 REAL NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(summary_id, model),
  FOREIGN KEY(summary_id) REFERENCES summaries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_summaries_type_period ON summaries(type, period_key);
CREATE INDEX IF NOT EXISTS idx_embeddings_model ON embeddings(model);
`

func mustOpenDB(cfg Config) *sql.DB {
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		panic(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		panic(err)
	}
	return db
}

func summaryExists(db *sql.DB, typ, key string) (bool, error) {
	row := db.QueryRow(`SELECT 1 FROM summaries WHERE type=? AND period_key=? LIMIT 1`, typ, key)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func upsertSummary(db *sql.DB, cfg Config, typ, key, startDate, endDate, js, text, srcPath string) (int64, error) {
	now := time.Now().In(cfg.Location).Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO summaries(type, period_key, start_date, end_date, json, text, source_path, created_at)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(type, period_key) DO UPDATE SET
		  json=excluded.json,
		  text=excluded.text,
		  source_path=excluded.source_path
	`, typ, key, startDate, endDate, js, text, srcPath, now)
	if err != nil {
		return 0, err
	}

	row := db.QueryRow(`SELECT id FROM summaries WHERE type=? AND period_key=?`, typ, key)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func loadSummaryJSON(db *sql.DB, typ, key string) (string, bool) {
	row := db.QueryRow(`SELECT json FROM summaries WHERE type=? AND period_key=?`, typ, key)
	var js string
	if err := row.Scan(&js); err != nil {
		return "", false
	}
	return js, true
}

func loadWeekliesInRange(db *sql.DB, startDate, endDate string) []string {
	rows, err := db.Query(`
		SELECT json FROM summaries
		WHERE type='weekly' AND start_date>=? AND end_date<=?
		ORDER BY start_date ASC
	`, startDate, endDate)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var js string
		if err := rows.Scan(&js); err == nil {
			out = append(out, js)
		}
	}
	return out
}

func hasEmbedding(db *sql.DB, summaryID int64, model string) bool {
	row := db.QueryRow(`SELECT 1 FROM embeddings WHERE summary_id=? AND model=? LIMIT 1`, summaryID, model)
	var one int
	return row.Scan(&one) == nil
}
