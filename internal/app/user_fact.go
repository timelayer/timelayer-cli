package app

import (
	"database/sql"
	"strings"
)

/*
================================================
User Fact Extraction (V2)
不靠枚举、不靠 prompt、不靠模型自述
================================================
*/

// RawLine 表示一条原始对话记录
type RawLine struct {
	Role    string
	Content string
}

/*
------------------------------------------------
核心入口
------------------------------------------------
*/

// isUserFactV2 判断一组 user → assistant 是否构成“用户事实”
func isUserFactV2(user RawLine, assistant RawLine) bool {
	// 1️⃣ 必须是 user → assistant 顺序
	if user.Role != "user" || assistant.Role != "assistant" {
		return false
	}

	u := normalizeText(user.Content)
	a := normalizeText(assistant.Content)

	// 2️⃣ user 必须是“自我陈述”
	if !looksLikeSelfStatement(u) {
		return false
	}

	// 3️⃣ assistant 必须确认 / 使用该事实
	if !assistantAffirmsUser(u, a) {
		return false
	}

	return true
}

/*
------------------------------------------------
子判定逻辑
------------------------------------------------
*/

// 判断是否像“用户在说自己”
func looksLikeSelfStatement(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	// 必须以第一人称开始
	if !strings.HasPrefix(text, "我") {
		return false
	}

	// 排除疑问句
	if strings.HasSuffix(text, "吗") ||
		strings.HasSuffix(text, "?") ||
		strings.HasSuffix(text, "？") {
		return false
	}

	// 排除指令 / 请求
	if strings.Contains(text, "帮我") ||
		strings.Contains(text, "请你") {
		return false
	}

	return true
}

// 判断 assistant 是否确认 / 使用了 user 的陈述
func assistantAffirmsUser(userText, assistantText string) bool {
	// assistant 必须使用第二人称
	if !strings.Contains(assistantText, "你") {
		return false
	}

	// 抽取 user 的核心信息
	core := extractUserCore(userText)
	if core == "" {
		return false
	}

	// assistant 必须复现该核心
	return strings.Contains(assistantText, core)
}

// 抽取 user 陈述的“核心事实部分”
func extractUserCore(text string) string {
	// 去掉“我”
	text = strings.TrimSpace(strings.TrimPrefix(text, "我"))

	// 去掉常见标点
	text = strings.Trim(text, "。！! ")

	// 限制长度（防 prompt 注入）
	r := []rune(text)
	if len(r) > 20 {
		text = string(r[:20])
	}

	return text
}

// 文本标准化（降低噪音）
func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, "。", "")
	s = strings.ReplaceAll(s, "！", "")
	s = strings.ReplaceAll(s, "？", "")
	return s
}

/*
------------------------------------------------
辅助：从 raw 对话中提取所有用户事实
------------------------------------------------
*/

func ExtractUserFactsFromRaw(lines []RawLine) []string {
	var facts []string

	for i := 0; i+1 < len(lines); i++ {
		if isUserFactV2(lines[i], lines[i+1]) {
			facts = append(facts, lines[i].Content)
		}
	}

	return facts
}

func RememberFact(lw *LogWriter, cfg Config, db *sql.DB, content string) error {
	if content == "" {
		return nil
	}

	// 1️⃣ 构造“第一人称事实陈述”
	userText := "我确认一个事实：" + content
	_ = lw.WriteRecord(map[string]string{
		"role":    "user",
		"content": userText,
	})

	// 2️⃣ 构造 assistant 的“确认复述”
	assistantText := "我理解了，你提到" + content
	_ = lw.WriteRecord(map[string]string{
		"role":    "assistant",
		"content": assistantText,
	})

	return nil
}

func ForgetFact(lw *LogWriter, cfg Config, db *sql.DB, content string) error {
	if content == "" {
		return nil
	}

	// 1️⃣ 用户显式撤回事实（第一人称）
	userText := "我撤回之前的事实：" + content
	_ = lw.WriteRecord(map[string]string{
		"role":    "user",
		"content": userText,
	})

	// 2️⃣ assistant 明确确认撤回
	assistantText := "我理解了，你明确表示之前关于「" + content + "」的事实不再成立。"
	_ = lw.WriteRecord(map[string]string{
		"role":    "assistant",
		"content": assistantText,
	})

	return nil
}
