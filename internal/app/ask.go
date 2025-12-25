package app

import (
	"database/sql"
	"fmt"
	"strings"
)

/*
========================
Public API
========================
*/

// Ask answers a question based on user's historical summaries.
// Default: show Top-1 reference
// With --refs: show Top-N references (appendix)
func Ask(db *sql.DB, cfg Config, input string) (string, error) {
	question, showRefs := parseAskArgs(input)

	// 1. semantic search
	hits, err := SearchWithScore(db, cfg, question)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "我没有在你的历史记录中找到相关内容，因此无法基于记忆回答这个问题。", nil
	}

	// 2. build memory context (TopK for reasoning)
	var ctx strings.Builder
	ctx.WriteString("以下是我在你过去记录中找到的相关内容：\n\n")

	for i, h := range hits {
		if i >= cfg.SearchTopK {
			break
		}
		ctx.WriteString(fmt.Sprintf(
			"- [%s %s | score %.2f]\n%s\n\n",
			h.Date,
			h.Type,
			h.Score,
			h.Text,
		))
	}

	// 3. compose prompt
	prompt := buildAskPrompt(ctx.String(), question)

	// 4. call LLM
	answer, err := callLLMNonStream(prompt)
	if err != nil {
		return "", err
	}

	// 5. append references
	var out strings.Builder
	out.WriteString(answer)

	// Top-1 reference (always)
	out.WriteString("\n\n——\n")
	out.WriteString(formatTopReference(hits[0]))

	// Optional appendix (Top-N)
	if showRefs {
		out.WriteString("\n\n附录 · 相关记录（最多 10 条）：\n")
		max := min(10, len(hits))
		for i := 0; i < max; i++ {
			out.WriteString(formatRefLine(i+1, hits[i]))
			out.WriteString("\n")
		}
	}

	// ✅ 在这里加 TTS（只读“核心回答”，不是 refs）
	Speak(answer)
	return out.String(), nil
}

/*
========================
Argument Parser
========================
*/

func parseAskArgs(input string) (question string, showRefs bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", false
	}

	var q []string
	for _, p := range parts {
		if p == "--refs" {
			showRefs = true
		} else {
			q = append(q, p)
		}
	}
	return strings.Join(q, " "), showRefs
}

/*
========================
Prompt Builder
========================
*/

func buildAskPrompt(memoryContext, question string) string {
	return fmt.Sprintf(`
你是“基于用户自身长期记忆”的智能助理，而不是百科或搜索引擎。

【重要原则】
- 你只能基于“用户自己的历史记录”来回答
- 如果历史记录不足以支撑结论，请明确说明
- 不要假装知道用户未记录的事实
- 不要覆盖或否定用户过去的认知，只能在其基础上补充或整理

【用户的历史记录】
%s

【用户当前的问题】
%s

【你的任务】
基于上述“用户自己的历史记录”，用清晰、简洁、自然语言回答问题。
如果记录中存在多个观点，请合并总结。
如果信息不足，请直接说明“不足以回答”。

请开始回答：
`, memoryContext, question)
}

/*
========================
Reference Formatting
========================
*/

func formatTopReference(h SearchHit) string {
	return fmt.Sprintf(
		"参考：你在 %s 的 %s 记录（%s）。",
		h.Date,
		h.Type,
		firstLine(h.Text),
	)
}

func formatRefLine(idx int, h SearchHit) string {
	return fmt.Sprintf(
		"%d. [%.2f] %s %s · %s",
		idx,
		h.Score,
		h.Date,
		h.Type,
		firstLine(h.Text),
	)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimPrefix(s[:i], "- ")
	}
	return strings.TrimPrefix(s, "- ")
}
