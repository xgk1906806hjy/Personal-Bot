package models

import "unicode/utf8"

// EstimateTokens 用 rune 数粗估 token（偏 CJK 友好启发式）。
func EstimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n <= 0 {
		return 0
	}
	return n
}

// EstimateUsage 在无 API usage 时根据文本估算消耗。
func EstimateUsage(prompt, completion string) TokenUsage {
	p := EstimateTokens(prompt)
	c := EstimateTokens(completion)
	return TokenUsage{
		PromptTokens:     p,
		CompletionTokens: c,
		TotalTokens:      p + c,
		Estimated:        true,
	}
}
