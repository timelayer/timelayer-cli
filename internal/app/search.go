package app

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
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
HTTP Client (avoid hang)
========================
*/

var searchHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
}

/*
========================
Public Search API
========================
*/

func SearchWithScore(db *sql.DB, cfg Config, query string) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// 1. embed query
	qv, qn, err := embedText(query)
	if err != nil {
		return nil, err
	}
	// ✅ 防线：避免除 0 / NaN
	if qn == 0 {
		return nil, nil
	}

	// 2. load all embeddings
	rows, err := db.Query(`
		SELECT s.type, s.period_key, s.json, e.vec, e.l2, e.dim
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
			dim  int
		)
		if err := rows.Scan(&typ, &key, &js, &blob, &l2, &dim); err != nil {
			continue
		}

		// ✅ 防线：维度必须匹配，否则 dotProduct 会产生错误分数
		if dim != len(qv) {
			continue
		}
		// ✅ 防线：避免除 0
		if l2 == 0 {
			continue
		}

		dot, ok := dotProductExactDim(qv, blob, dim)
		if !ok {
			// blob 不完整/损坏，跳过
			continue
		}

		score := dot / (qn * l2)
		if math.IsNaN(score) || math.IsInf(score, 0) {
			continue
		}

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
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", embedURL, bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := searchHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// ✅ 必须检查状态码
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("embed http error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var er embedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, 0, err
	}
	if len(er.Data) == 0 || len(er.Data[0].Embedding) == 0 {
		return nil, 0, fmt.Errorf("empty embedding")
	}

	vec := er.Data[0].Embedding
	n := l2norm(vec)
	return vec, n, nil
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

	// memory_candidates（你 prompt 已禁止生成，但兼容旧数据）
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

// dotProductExactDim: 读取 blob 中 exactly dim 个 float32，与 q 做点积；
// 任何读失败都返回 ok=false，避免“吞错导致分数乱飞”
func dotProductExactDim(q []float32, blob []byte, dim int) (sum float64, ok bool) {
	// 每个 float32 4 bytes
	if len(blob) < dim*4 {
		return 0, false
	}

	buf := bytes.NewReader(blob)

	for i := 0; i < dim; i++ {
		var x float32
		if err := binary.Read(buf, binary.LittleEndian, &x); err != nil {
			return 0, false
		}
		sum += float64(q[i] * x)
	}
	return sum, true
}

func l2norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x * x)
	}
	return math.Sqrt(s)
}
