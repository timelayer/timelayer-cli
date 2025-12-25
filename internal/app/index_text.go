package app

import (
	"encoding/json"
	"strings"
)

func extractIndexText(summaryJSON string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(summaryJSON), &m); err != nil {
		return summaryJSON
	}

	var parts []string

	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case string:
			parts = append(parts, x)
		case []any:
			for _, it := range x {
				collect(it)
			}
		case map[string]any:
			for _, vv := range x {
				collect(vv)
			}
		}
	}

	for _, k := range []string{
		"tags",
		"themes",
		"topics",
		"projects",
		"decisions",
		"patterns",
		"highlights",
		"lowlights",
		"memory_candidates",
		"next_week_focus",
		"next_month_bets",
	} {
		if v, ok := m[k]; ok {
			collect(v)
		}
	}

	text := strings.Join(parts, "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return summaryJSON
	}
	return text
}
