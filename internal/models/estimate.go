package models

import "unicode/utf8"

// EstimateTokens rough-counts tokens from text (CJK-friendly rune heuristic).
func EstimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n <= 0 {
		return 0
	}
	return n
}

// EstimateUsage builds an estimated TokenUsage from prompt and completion text.
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
