package feishu

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	reUserMention = regexp.MustCompile(`@_user_\d+`)
	reAtToken     = regexp.MustCompile(`@_\w+`)
)

func extractMessageText(contentRaw, messageType string) string {
	if messageType != "text" {
		return fmt.Sprintf("[Unsupported message type: %s]", messageType)
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(contentRaw), &parsed); err != nil {
		return strings.TrimSpace(contentRaw)
	}
	text := parsed.Text
	text = reUserMention.ReplaceAllString(text, " ")
	text = reAtToken.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}
