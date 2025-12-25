package app

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
)

/*
========================
Search Result Structure
========================
*/

type SearchHit struct {
	Score float64
	Type  string
	Date  string
	Text  string
}

/*
========================
Public Search API
========================
*/

func SearchWithScore(db *sql.DB, cfg Config, query string) ([]SearchHit, error) {
	// 1. embed query
	qv, qn, err := embedText(query)
	if err != nil {
		return nil, err
	}

	// 2. load all embeddings
	rows, err := db.Query(`
		SELECT s.type, s.period_key, s.json, e.vec, e.l2
		FROM embeddings e
		JOIN summaries s ON s.id = e.summary_id
		WHERE e.model = ?
	`, embedModel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit

	for rows.Next() {
		var (
			typ  string
			key  string
			js   string
			blob []byte
			l2   float64
		)
		if err := rows.Scan(&typ, &key, &js, &blob, &l2); err != nil {
			continue
		}

		dot := dotProduct(qv, blob)
		score := dot / (qn * l2)

		if score < cfg.SearchMinScore {
			continue
		}

		hits = append(hits, SearchHit{
			Score: score,
			Type:  typ,
			Date:  key,
			Text:  extractHumanText(js),
		})
	}

	// 3. sort by score desc
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})

	// 4. topK
	if len(hits) > cfg.SearchTopK {
		hits = hits[:cfg.SearchTopK]
	}

	return hits, nil
}

/*
========================
Embedding Helper
========================
*/

func embedText(text string) ([]float32, float64, error) {
	payload := map[string]any{
		"model": embedModel,
		"input": text,
	}
	b, _ := json.Marshal(payload)

	resp, err := http.Post(embedURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var er embedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, 0, err
	}
	if len(er.Data) == 0 {
		return nil, 0, fmt.Errorf("empty embedding")
	}

	vec := er.Data[0].Embedding
	return vec, l2norm(vec), nil
}

/*
========================
Human Readable Extract
========================
*/

func extractHumanText(js string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		return js
	}

	var lines []string

	// highlights
	if hs, ok := m["highlights"].([]any); ok {
		for _, h := range hs {
			lines = append(lines, fmt.Sprintf("- %v", h))
		}
	}

	// memory_candidates
	if mems, ok := m["memory_candidates"].([]any); ok {
		for _, mm := range mems {
			if x, ok := mm.(map[string]any); ok {
				if c, ok := x["content"]; ok {
					lines = append(lines, fmt.Sprintf("- %v", c))
				}
			}
		}
	}

	// fallback
	if len(lines) == 0 {
		if t, ok := m["type"]; ok {
			lines = append(lines, fmt.Sprintf("summary type: %v", t))
		}
	}

	return strings.Join(lines, "\n")
}

/*
========================
Vector Math
========================
*/

func dotProduct(q []float32, blob []byte) float64 {
	buf := bytes.NewReader(blob)
	var sum float64
	for _, v := range q {
		var x float32
		_ = binary.Read(buf, binary.LittleEndian, &x)
		sum += float64(v * x)
	}
	return sum
}

func l2norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x * x)
	}
	return math.Sqrt(s)
}
