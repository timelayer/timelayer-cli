package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LogWriter struct {
	cfg        Config
	db         *sql.DB
	file       *os.File
	currentDay string
}

func NewLogWriter(cfg Config, db *sql.DB) *LogWriter {
	return &LogWriter{
		cfg: cfg,
		db:  db,
	}
}

func (lw *LogWriter) Close() {
	if lw.file != nil {
		_ = lw.file.Close()
		lw.file = nil
	}
}

func (lw *LogWriter) WriteRecord(rec map[string]string) error {
	now := time.Now().In(lw.cfg.Location)
	today := now.Format("2006-01-02")

	// 跨天：先处理昨天
	if lw.currentDay != "" && lw.currentDay != today {
		yesterday := lw.currentDay

		// ---------- DAILY ----------
		_ = ensureDaily(lw.cfg, lw.db, yesterday, false)

		// ---------- WEEKLY ----------
		yDate, _ := time.ParseInLocation("2006-01-02", yesterday, lw.cfg.Location)
		tDate, _ := time.ParseInLocation("2006-01-02", today, lw.cfg.Location)

		yYear, yWeek := yDate.ISOWeek()
		tYear, tWeek := tDate.ISOWeek()

		if yYear != tYear || yWeek != tWeek {
			weekKey := fmt.Sprintf("%04d-W%02d", yYear, yWeek)
			_ = ensureWeekly(lw.cfg, lw.db, weekKey, false)
		}

		// ---------- MONTHLY ----------
		yMonth := yDate.Format("2006-01")
		tMonth := tDate.Format("2006-01")

		if yMonth != tMonth {
			_ = ensureMonthly(lw.cfg, lw.db, yMonth, false)
		}

		// ---------- ARCHIVE ----------
		_ = forgetAndArchive(lw.cfg, lw.db)

		if lw.file != nil {
			_ = lw.file.Close()
			lw.file = nil
		}
	}

	if lw.file == nil {
		_ = os.MkdirAll(lw.cfg.LogDir, 0755)
		f, err := os.OpenFile(
			filepath.Join(lw.cfg.LogDir, today+".jsonl"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return err
		}
		lw.file = f
		lw.currentDay = today
	}

	b, _ := json.Marshal(rec)
	_, err := lw.file.Write(append(b, '\n'))
	return err
}
