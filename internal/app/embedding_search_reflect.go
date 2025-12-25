package app

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

/*
========================
Embedding Response
========================
*/

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

/*
========================
Embedding Writer
（保持原样）
========================
*/

func ensureEmbedding(db *sql.DB, cfg Config, text, typ, key string) error {
	row := db.QueryRow(`SELECT id FROM summaries WHERE type=? AND period_key=?`, typ, key)
	var sid int64
	if err := row.Scan(&sid); err != nil {
		return err
	}

	if hasEmbedding(db, sid, embedModel) {
		return nil
	}

	payload := map[string]any{
		"model": embedModel,
		"input": text,
	}
	b, _ := json.Marshal(payload)

	resp, err := http.Post(embedURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var er embedResp
	_ = json.NewDecoder(resp.Body).Decode(&er)
	if len(er.Data) == 0 {
		return fmt.Errorf("empty embedding")
	}

	vec := er.Data[0].Embedding
	buf := new(bytes.Buffer)
	var l2 float64
	for _, v := range vec {
		_ = binary.Write(buf, binary.LittleEndian, v)
		l2 += float64(v * v)
	}
	l2 = math.Sqrt(l2)

	_, err = db.Exec(`
		INSERT INTO embeddings(summary_id, model, dim, vec, l2, created_at)
		VALUES(?,?,?,?,?,?)
	`, sid, embedModel, len(vec), buf.Bytes(), l2, time.Now().Format(time.RFC3339))
	return err
}

/*
========================
CLI Command Router（修复版）
========================
*/

func handleCommand(cfg Config, db *sql.DB, lw *LogWriter, input string) {
	switch {

	case input == "/help":
		fmt.Println(`
/help                         show help

/chat <msg>                   chat with memory context
/ask <question>               ask with memory context
/ask <question> --refs        include top-N memory references

/search <query>               semantic search summaries

/daily                        generate today's daily summary (idempotent)
/daily --force                regenerate today's daily summary

/weekly                       generate current week's summary
/weekly --force               regenerate current week's summary

/monthly                      generate current month's summary
/monthly --force              regenerate current month's summary

/reindex daily|weekly|monthly|all   backfill embeddings

/remember <fact>              explicitly teach the system a confirmed fact
/forget <fact>                explicitly retract a previously remembered fact

`)

	// ---------- SEARCH ----------
	case strings.HasPrefix(input, "/search "):
		q := strings.TrimPrefix(input, "/search ")
		hits, err := SearchWithScore(db, cfg, q)
		if err != nil {
			fmt.Println("search error:", err)
			return
		}
		if len(hits) == 0 {
			fmt.Println("no related memory")
			return
		}
		for _, h := range hits {
			fmt.Printf("[%.2f] %s %s\n", h.Score, h.Date, h.Type)
			fmt.Println(h.Text)
			fmt.Println("----------------------")
		}

	// ---------- ASK ----------
	case strings.HasPrefix(input, "/ask "):
		raw := strings.TrimPrefix(input, "/ask ")
		ans, err := Ask(db, cfg, raw)
		if err != nil {
			fmt.Println("ask error:", err)
			return
		}
		fmt.Println(ans)

		// ---------- REMEMBER ----------
	case strings.HasPrefix(input, "/remember "):
		content := strings.TrimSpace(strings.TrimPrefix(input, "/remember "))
		if content == "" {
			fmt.Println("usage: /remember <fact>")
			return
		}
		if err := RememberFact(lw, cfg, db, content); err != nil {
			fmt.Println("remember error:", err)
		} else {
			fmt.Println("[ok] fact recorded, will be persisted in daily summary")
		}

	case strings.HasPrefix(input, "/forget "):
		content := strings.TrimSpace(strings.TrimPrefix(input, "/forget "))
		if content == "" {
			fmt.Println("usage: /forget <fact>")
			return
		}
		if err := ForgetFact(lw, cfg, db, content); err != nil {
			fmt.Println("forget error:", err)
		} else {
			fmt.Println("[ok] fact retracted, will be reflected in daily summary")
		}

	// ---------- CHAT ----------
	case strings.HasPrefix(input, "/chat "):
		raw := strings.TrimPrefix(input, "/chat ")
		if err := Chat(lw, cfg, db, raw); err != nil {
			fmt.Println("chat error:", err)
		}
	// ---------- DAILY ----------
	case strings.HasPrefix(input, "/daily"):
		parts := strings.Fields(input)
		force := len(parts) > 1 && parts[1] == "--force"

		today := time.Now().In(cfg.Location).Format("2006-01-02")

		if err := ensureDaily(cfg, db, today, force); err != nil {
			fmt.Println("daily error:", err)
			return
		}

		if force {
			fmt.Println("[ok] daily summary regenerated:", today)
		} else {
			fmt.Println("[ok] daily summary ensured:", today)
		}

		// ---------- WEEKLY ----------
	case strings.HasPrefix(input, "/weekly"):
		parts := strings.Fields(input)
		force := len(parts) > 1 && parts[1] == "--force"

		now := time.Now().In(cfg.Location)
		year, week := now.ISOWeek()
		weekKey := fmt.Sprintf("%04d-W%02d", year, week)

		if err := ensureWeekly(cfg, db, weekKey, force); err != nil {
			fmt.Println("weekly error:", err)
			return
		}

		if force {
			fmt.Println("[ok] weekly summary regenerated:", weekKey)
		} else {
			fmt.Println("[ok] weekly summary ensured:", weekKey)
		}

	// ---------- MONTHLY ----------
	case strings.HasPrefix(input, "/monthly"):
		parts := strings.Fields(input)
		force := len(parts) > 1 && parts[1] == "--force"

		monthKey := time.Now().In(cfg.Location).Format("2006-01")

		if err := ensureMonthly(cfg, db, monthKey, force); err != nil {
			fmt.Println("monthly error:", err)
			return
		}

		if force {
			fmt.Println("[ok] monthly summary regenerated:", monthKey)
		} else {
			fmt.Println("[ok] monthly summary ensured:", monthKey)
		}

	case strings.HasPrefix(input, "/reindex"):
		parts := strings.Fields(input)
		target := "daily"
		if len(parts) > 1 {
			target = parts[1]
		}
		if err := Reindex(db, cfg, target); err != nil {
			fmt.Println("reindex error:", err)
		}

		// ---------- DEBUG CHAT ----------
	case strings.HasPrefix(input, "/debug chat "):
		raw := strings.TrimPrefix(input, "/debug chat ")
		DebugChat(cfg, db, raw)

	default:
		fmt.Println("unknown command, try /help")
	}
}
