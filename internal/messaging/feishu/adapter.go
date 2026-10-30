package feishu

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

type Adapter struct {
	client *Client
	orch   *orchestrator.Orchestrator
	cfg    *config.Config

	mu        sync.Mutex
	processed map[string]struct{}
}

func NewAdapter() *Adapter {
	return &Adapter{processed: make(map[string]struct{})}
}

func (a *Adapter) ID() config.MessagingPlatform { return config.PlatformFeishu }

func (a *Adapter) Mount(ctx messaging.AdapterContext) error {
	a.cfg = ctx.Config
	a.orch = ctx.Orchestrator
	a.client = NewClient(ctx.Config)
	ctx.Mux.HandleFunc("/feishu/webhook", a.handleWebhook)
	log.Printf("[boot] messaging adapter mounted: feishu → POST /feishu/webhook")
	return nil
}

func (a *Adapter) Close() error { return nil }

func (a *Adapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	body, err := a.normalizeBody(raw)
	if err != nil {
		log.Printf("[feishu] normalize body failed: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// URL verification (v1 style)
	if typ, _ := envelope["type"].(string); typ == "url_verification" {
		challenge, _ := envelope["challenge"].(string)
		token, _ := envelope["token"].(string)
		if a.cfg.FeishuVerificationToken != "" && token != a.cfg.FeishuVerificationToken {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": challenge})
		return
	}

	// Event v2
	if schema, _ := envelope["schema"].(string); schema == "2.0" {
		header, _ := envelope["header"].(map[string]any)
		if a.cfg.FeishuVerificationToken != "" {
			token, _ := header["token"].(string)
			if token != a.cfg.FeishuVerificationToken {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		eventType, _ := header["event_type"].(string)
		event, _ := envelope["event"].(map[string]any)
		if eventType == "im.message.receive_v1" {
			go a.handleMessageEvent(event)
		}
		return
	}

	// Event v1 callback
	if typ, _ := envelope["type"].(string); typ == "event_callback" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *Adapter) normalizeBody(raw []byte) ([]byte, error) {
	var maybeEncrypt struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(raw, &maybeEncrypt); err == nil && maybeEncrypt.Encrypt != "" {
		return decryptEncryptString(a.cfg.FeishuEncryptKey, maybeEncrypt.Encrypt)
	}
	return raw, nil
}

func (a *Adapter) handleMessageEvent(event map[string]any) {
	message, _ := event["message"].(map[string]any)
	sender, _ := event["sender"].(map[string]any)
	senderID, _ := sender["sender_id"].(map[string]any)

	messageID, _ := message["message_id"].(string)
	chatID, _ := message["chat_id"].(string)
	userID, _ := senderID["open_id"].(string)
	messageType, _ := message["message_type"].(string)
	if messageType == "" {
		messageType = "text"
	}
	contentRaw, _ := message["content"].(string)
	if contentRaw == "" {
		contentRaw = "{}"
	}

	if messageID == "" || chatID == "" || userID == "" {
		log.Printf("[feishu] missing message/chat/user id, skip")
		return
	}

	a.mu.Lock()
	if _, ok := a.processed[messageID]; ok {
		a.mu.Unlock()
		log.Printf("[feishu] duplicate message %s, skip", messageID)
		return
	}
	a.processed[messageID] = struct{}{}
	if len(a.processed) > 5000 {
		for k := range a.processed {
			delete(a.processed, k)
			break
		}
	}
	a.mu.Unlock()

	text := extractMessageText(contentRaw, messageType)
	if text == "" {
		log.Printf("[feishu] empty text after mention strip, skip %s", messageID)
		return
	}

	log.Printf("[feishu] recv chat=%s user=%s model=%s text=%s",
		chatID, userID, a.orch.ModelChannelID(), truncateRunes(text, 80))

	if err := a.client.ReplyText(messageID, "Got it — thinking…"); err != nil {
		log.Printf("[feishu] ack failed: %v", err)
	}
	answer, err := a.orch.Handle(orchestrator.IncomingUserMessage{
		Platform:  config.PlatformFeishu,
		ChatID:    chatID,
		UserID:    userID,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		log.Printf("[feishu] handle failed: %v", err)
		_ = a.client.ReplyText(messageID, "Failed: "+err.Error())
		return
	}
	if err := a.client.ReplyText(messageID, answer); err != nil {
		log.Printf("[feishu] final reply failed: %v", err)
	}
}
