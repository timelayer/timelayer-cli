package app

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

func mustEnsureDirs(cfg Config) {
	_ = os.MkdirAll(cfg.LogDir, 0755)
	_ = os.MkdirAll(cfg.ArchiveDir, 0755)
	_ = os.MkdirAll(cfg.PromptDir, 0755)
	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0755)
}

func readTailBytes(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()

	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	_, _ = f.Seek(start, io.SeekStart)
	return io.ReadAll(f)
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
