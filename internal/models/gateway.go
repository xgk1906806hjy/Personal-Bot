package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

type gatewayChannel struct {
	cfg    *config.Config
	client *http.Client

	mu        sync.Mutex
	histories map[string][]map[string]string
}

func newGatewayChannel(cfg *config.Config) *gatewayChannel {
	return &gatewayChannel{
		cfg:       cfg,
		client:    &http.Client{Timeout: 120 * time.Second},
		histories: make(map[string][]map[string]string),
	}
}

func (c *gatewayChannel) ID() config.ModelChannel { return config.ModelGatewayAPI }

func (c *gatewayChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	c.mu.Lock()
	history := append([]map[string]string{}, c.histories[conversationID]...)
	c.mu.Unlock()

	messages := make([]map[string]string, 0, len(history)+2)
	messages = append(messages, map[string]string{
		"role":    "system",
		"content": c.cfg.BotSystemPrompt,
	})
	messages = append(messages, history...)
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": req.Prompt,
	})

	body, _ := json.Marshal(map[string]any{
		"model":       c.cfg.GatewayAPIModel,
		"messages":    messages,
		"temperature": 0.2,
	})

	httpReq, err := http.NewRequest(http.MethodPost, c.cfg.GatewayAPIBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.GatewayAPIKey)
	if c.cfg.GatewayAPIExtraHeaderName != "" && c.cfg.GatewayAPIExtraHeaderValue != "" {
		httpReq.Header.Set(c.cfg.GatewayAPIExtraHeaderName, c.cfg.GatewayAPIExtraHeaderValue)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("gateway decode: %w", err)
	}
	if resp.StatusCode >= 300 {
		msg := resp.Status
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return Response{}, fmt.Errorf("gateway API error: %s", msg)
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return Response{}, fmt.Errorf("gateway API returned empty content")
	}
	text := parsed.Choices[0].Message.Content

	c.mu.Lock()
	history = append(history,
		map[string]string{"role": "user", "content": req.Prompt},
		map[string]string{"role": "assistant", "content": text},
	)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	c.histories[conversationID] = history
	c.mu.Unlock()

	return Response{
		Channel:        c.ID(),
		Text:           text,
		ConversationID: conversationID,
	}, nil
}
