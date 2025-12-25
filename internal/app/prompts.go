package app

import (
	"os"
	"path/filepath"
)

/*
================================================
Daily / Weekly / Monthly Prompts (FINAL)
原则：
- LLM 只做“行为总结 / 模式归纳”
- 不允许生成任何用户事实
- 不允许 memory_candidates
- 不允许身份 / 名字 / 背景判断
================================================
*/

const promptDaily = `You are a conversation log summarizer.
You are NOT an assistant, NOT an analyst, and NOT a memory writer.

CRITICAL RULES (must follow strictly):
- Do NOT guess, infer, or generate any facts about the user.
- Do NOT state the user's name, identity, background, or preferences.
- Do NOT repeat assistant self-introductions or model descriptions.
- Do NOT create memory candidates or long-term facts.
- If something cannot be confirmed from explicit user statements, ignore it.

Your job is ONLY to:
1. Describe what happened in today's conversations (behavior-level).
2. Identify recurring topics or patterns.
3. Note unresolved questions or friction.

OUTPUT FORMAT (JSON only, no markdown, no extra fields):

{
  "type": "daily",
  "date": "{{DATE}}",
  "topics": [],
  "patterns": [],
  "open_questions": [],
  "highlights": [],
  "lowlights": []
}

RAW CONVERSATION LOG (JSONL):
{{TRANSCRIPT}}
`

const promptWeekly = `You are a strict summarizer.
You must output JSON only.

CRITICAL RULES:
- Do NOT infer or generate user identity or personal facts.
- Do NOT create memory candidates.
- Do NOT restate assistant or system information.
- Weekly summary is for trends and progress only.

GOAL:
Summarize patterns and progress from the past week based on daily summaries.

OUTPUT FORMAT (JSON only):

{
  "type": "weekly",
  "week_start": "{{WEEK_START}}",
  "week_end": "{{WEEK_END}}",
  "themes": [],
  "progress": [],
  "recurring_blockers": [],
  "notable_decisions": [],
  "next_week_focus": []
}

DAILY_SUMMARIES_JSON_ARRAY:
{{DAILY_JSON_ARRAY}}
`

const promptMonthly = `You are a strict summarizer.
You must output JSON only.

CRITICAL RULES:
- Do NOT infer or generate user identity or personal facts.
- Do NOT create memory candidates.
- Do NOT restate assistant or system information.
- Monthly summary is for long-term trajectory only.

GOAL:
Summarize overall direction and themes for the month.

OUTPUT FORMAT (JSON only):

{
  "type": "monthly",
  "month": "{{MONTH}}",
  "month_start": "{{MONTH_START}}",
  "month_end": "{{MONTH_END}}",
  "trajectory": [],
  "top_themes": [],
  "wins": [],
  "losses": [],
  "systems_improvements": [],
  "next_month_bets": []
}

WEEKLY_SUMMARIES_JSON_ARRAY:
{{WEEKLY_JSON_ARRAY}}
`

/*
================================================
Prompt File Management
================================================
*/

func mustEnsurePromptFiles(cfg Config) {
	_ = os.MkdirAll(cfg.PromptDir, 0755)

	// ⚠️ 强制覆盖旧 prompt，避免污染遗留
	_ = os.WriteFile(filepath.Join(cfg.PromptDir, "daily.txt"), []byte(promptDaily), 0644)
	_ = os.WriteFile(filepath.Join(cfg.PromptDir, "weekly.txt"), []byte(promptWeekly), 0644)
	_ = os.WriteFile(filepath.Join(cfg.PromptDir, "monthly.txt"), []byte(promptMonthly), 0644)
}

func mustReadPrompt(cfg Config, name string) string {
	p := filepath.Join(cfg.PromptDir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return string(b)
}
