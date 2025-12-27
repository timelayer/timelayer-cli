package app

import (
	"bufio"
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
CLI Command Router
========================
*/

func handleCommand(cfg Config, db *sql.DB, lw *LogWriter, reader *bufio.Reader, input string) {
	switch {

	case input == "/help":
		fmt.Println(`
/help                         show help

/chat <msg>                   chat with memory context
/ask <question>               ask with memory context
/search <query>               semantic search summaries

/daily                        generate today's daily summary
/daily --force                regenerate today's daily summary

/weekly                       generate current week's summary
/weekly --force               regenerate current week's summary

/monthly                      generate current month's summary
/monthly --force              regenerate current month's summary

/reindex daily|weekly|monthly|all   backfill embeddings

/remember <fact>              explicitly teach the system a confirmed fact
/forget <fact>                explicitly retract a previously remembered fact

/paste                        enter multi-line input (empty line submits)
`)

	// ---------- PASTE ----------
	case input == "/paste":
		fmt.Println("(paste mode, empty line to submit)")
		msg, err := readMultiline(reader)
		if err != nil {
			fmt.Println("input error:", err)
			return
		}

		msg = strings.TrimSpace(msg)
		if msg == "" {
			return
		}

		fmt.Println("\nAssistant>")
		if DefaultUseLongTermChat {
			if err := Chat(lw, cfg, db, msg); err != nil {
				fmt.Println("chat error:", err)
			}
		} else {
			answer := streamChat(msg)
			_ = lw.WriteRecord(map[string]string{"role": "user", "content": msg})
			_ = lw.WriteRecord(map[string]string{"role": "assistant", "content": answer})
		}

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

	// ---------- CHAT ----------
	case strings.HasPrefix(input, "/chat "):
		raw := strings.TrimPrefix(input, "/chat ")
		fmt.Println("\nAssistant>")
		if err := Chat(lw, cfg, db, raw); err != nil {
			fmt.Println("chat error:", err)
		}

	// ---------- REMEMBER ----------
	case strings.HasPrefix(input, "/remember "):
		content := strings.TrimSpace(strings.TrimPrefix(input, "/remember "))
		if err := RememberFact(lw, cfg, db, content); err != nil {
			fmt.Println("remember error:", err)
		} else {
			fmt.Println("[ok] fact recorded")
		}

	// ---------- FORGET ----------
	case strings.HasPrefix(input, "/forget "):
		content := strings.TrimSpace(strings.TrimPrefix(input, "/forget "))
		if err := ForgetFact(lw, cfg, db, content); err != nil {
			fmt.Println("forget error:", err)
		} else {
			fmt.Println("[ok] fact retracted")
		}

	// ---------- DAILY ----------
	case strings.HasPrefix(input, "/daily"):
		force := strings.Contains(input, "--force")
		today := time.Now().In(cfg.Location).Format("2006-01-02")
		_ = ensureDaily(cfg, db, today, force)
		fmt.Println("[ok] daily summary ensured:", today)

	// ---------- WEEKLY ----------
	case strings.HasPrefix(input, "/weekly"):
		force := strings.Contains(input, "--force")
		y, w := time.Now().In(cfg.Location).ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", y, w)
		_ = ensureWeekly(cfg, db, key, force)
		fmt.Println("[ok] weekly summary ensured:", key)

	// ---------- MONTHLY ----------
	case strings.HasPrefix(input, "/monthly"):
		force := strings.Contains(input, "--force")
		key := time.Now().In(cfg.Location).Format("2006-01")
		_ = ensureMonthly(cfg, db, key, force)
		fmt.Println("[ok] monthly summary ensured:", key)

	// ---------- REINDEX ----------
	case strings.HasPrefix(input, "/reindex"):
		parts := strings.Fields(input)
		target := "daily"
		if len(parts) > 1 {
			target = parts[1]
		}
		if err := Reindex(db, cfg, target); err != nil {
			fmt.Println("reindex error:", err)
		}

	default:
		fmt.Println("unknown command, try /help")
	}
}
