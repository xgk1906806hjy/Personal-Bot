package qq

import (
	"log"
	"sync"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

var (
	processedMu sync.Mutex
	processed   = make(map[string]struct{})
)

type parsedInbound struct {
	messageID string
	chatID    string
	userID    string
	text      string
	target    ReplyTarget
}

func parseInbound(eventType string, d map[string]any) *parsedInbound {
	switch eventType {
	case "GROUP_AT_MESSAGE_CREATE":
		messageID, _ := d["id"].(string)
		groupOpenID, _ := d["group_openid"].(string)
		author, _ := d["author"].(map[string]any)
		userID, _ := author["member_openid"].(string)
		content, _ := d["content"].(string)
		if messageID == "" || groupOpenID == "" || userID == "" {
			return nil
		}
		return &parsedInbound{
			messageID: messageID,
			chatID:    groupOpenID,
			userID:    userID,
			text:      cleanText(content),
			target:    ReplyTarget{Kind: "group", GroupOpenID: groupOpenID},
		}
	case "C2C_MESSAGE_CREATE":
		messageID, _ := d["id"].(string)
		author, _ := d["author"].(map[string]any)
		userID, _ := author["user_openid"].(string)
		if userID == "" {
			userID, _ = author["id"].(string)
		}
		content, _ := d["content"].(string)
		if messageID == "" || userID == "" {
			return nil
		}
		return &parsedInbound{
			messageID: messageID,
			chatID:    userID,
			userID:    userID,
			text:      cleanText(content),
			target:    ReplyTarget{Kind: "c2c", UserOpenID: userID},
		}
	default:
		return nil
	}
}

func handleInbound(eventType string, d map[string]any, api *APIClient, orch *orchestrator.Orchestrator) {
	parsed := parseInbound(eventType, d)
	if parsed == nil {
		if eventType != "" && eventType != "READY" && eventType != "RESUMED" {
			log.Printf("[qq] ignore event type=%s", eventType)
		}
		return
	}

	processedMu.Lock()
	if _, ok := processed[parsed.messageID]; ok {
		processedMu.Unlock()
		log.Printf("[qq] duplicate message %s, skip", parsed.messageID)
		return
	}
	processed[parsed.messageID] = struct{}{}
	if len(processed) > 5000 {
		for k := range processed {
			delete(processed, k)
			break
		}
	}
	processedMu.Unlock()

	if parsed.text == "" {
		log.Printf("[qq] empty text, skip %s", parsed.messageID)
		return
	}

	log.Printf("[qq] recv chat=%s user=%s model=%s text=%s",
		parsed.chatID, parsed.userID, orch.ModelChannelID(), truncate(parsed.text, 80))

	if err := api.ReplyText(parsed.target, parsed.messageID, "Got it — thinking…"); err != nil {
		log.Printf("[qq] ack reply failed: %v", err)
	}

	answer, err := orch.Handle(orchestrator.IncomingUserMessage{
		Platform:  config.PlatformQQ,
		ChatID:    parsed.chatID,
		UserID:    parsed.userID,
		MessageID: parsed.messageID,
		Text:      parsed.text,
	})
	if err != nil {
		log.Printf("[qq] handle failed: %v", err)
		_ = api.ReplyText(parsed.target, parsed.messageID, "Failed: "+err.Error())
		return
	}
	if err := api.ReplyText(parsed.target, parsed.messageID, answer); err != nil {
		log.Printf("[qq] final reply failed: %v", err)
	}
}
