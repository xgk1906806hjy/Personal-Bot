package session

import (
	"fmt"
	"log"
	"strings"
)

// Summarizer 将较早的对话轮次压缩成短摘要。
type Summarizer interface {
	Summarize(old []Message) (string, error)
}

// MaybeCompress 在消息数超过 compressAfter 时压缩历史：
// 保留最近 keepRecent 条，并在前面插入一条 system 摘要。
// 摘要失败则硬截断为 recent。
func (s *Store) MaybeCompress(ownerKey string, sum Summarizer) error {
	s.mu.Lock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	msgs := append([]Message{}, c.Messages...)
	after := s.compressAfter
	keep := s.keepRecent
	s.mu.Unlock()

	if len(msgs) <= after {
		return nil
	}
	if keep >= len(msgs) {
		return nil
	}

	old := msgs[:len(msgs)-keep]
	recent := msgs[len(msgs)-keep:]

	summary, err := sum.Summarize(old)
	if err != nil {
		log.Printf("[session] compress summarize failed, hard truncate: %v", err)
		s.ReplaceMessages(ownerKey, recent)
		return nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		s.ReplaceMessages(ownerKey, recent)
		return nil
	}

	compressed := make([]Message, 0, 1+len(recent))
	compressed = append(compressed, Message{
		Role:    "system",
		Content: "[对话摘要]\n" + summary,
	})
	compressed = append(compressed, recent...)
	s.ReplaceMessages(ownerKey, compressed)
	log.Printf("[session] compressed owner=%s old=%d -> %d (keep=%d)", ownerKey, len(msgs), len(compressed), keep)
	return nil
}

// FormatMessagesForSummary builds plain text for the summarizer prompt.
func FormatMessagesForSummary(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		role := m.Role
		if role == "" {
			role = "user"
		}
		b.WriteString(strings.ToUpper(role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

func SummaryPrompt(oldText string) string {
	return fmt.Sprintf(
		"请将下列对话压成简短中文摘要，保留关键事实、结论、未完成事项与重要路径/命令。不要寒暄，不要列表过长。\n\n%s",
		oldText,
	)
}
