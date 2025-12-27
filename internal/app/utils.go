package app

import (
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

func mustEnsureDirs(cfg Config) {
	_ = os.MkdirAll(cfg.LogDir, 0755)
	_ = os.MkdirAll(cfg.ArchiveDir, 0755)
	_ = os.MkdirAll(cfg.PromptDir, 0755)
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)
}

// Week range: Monday..Sunday
func weekRange(d time.Time, loc *time.Location) (start, end time.Time) {
	dd := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	wd := int(dd.Weekday()) // Sunday=0
	offset := (wd + 6) % 7  // Monday->0 ... Sunday->6
	start = dd.AddDate(0, 0, -offset)
	end = start.AddDate(0, 0, 6)
	return
}

func monthRange(d time.Time, loc *time.Location) (start, end time.Time) {
	dd := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	start = time.Date(dd.Year(), dd.Month(), 1, 0, 0, 0, 0, loc)
	end = start.AddDate(0, 1, 0).AddDate(0, 0, -1)
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sanitizeUTF8 确保字符串是合法 UTF-8。
// 如果包含非法字节，会通过 rune 重构，
// 将非法部分替换为 �，避免污染日志与后续 JSON / Prompt。
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return string([]rune(s))
}
