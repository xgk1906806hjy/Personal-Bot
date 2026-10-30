package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/tools"
)

type gatewayChannel struct {
	cfg    *config.Config
	client *http.Client
	tools  *tools.Registry

	mu        sync.Mutex
	histories map[string][]map[string]any
}

func newGatewayChannel(cfg *config.Config, registry *tools.Registry) *gatewayChannel {
	return &gatewayChannel{
		cfg:       cfg,
		client:    &http.Client{Timeout: 180 * time.Second},
		tools:     registry,
		histories: make(map[string][]map[string]any),
	}
}

func (c *gatewayChannel) ID() config.ModelChannel { return config.ModelGatewayAPI }

func (c *gatewayChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	c.mu.Lock()
	history := append([]map[string]any{}, c.histories[conversationID]...)
	c.mu.Unlock()

	systemPrompt := c.cfg.BotSystemPrompt
	if c.tools != nil && c.cfg.ToolsEnabled {
		systemPrompt += "\n\nYou can use tools to read/write files and run shell commands inside the workspace (" +
			c.tools.WorkspaceRoot() +
			"). Prefer tools for file and command tasks. Available tools: " +
			strings.Join(c.tools.ToolNames(), ", ") + "."
	}

	messages := make([]map[string]any, 0, len(history)+2)
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": systemPrompt,
	})
	messages = append(messages, history...)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": req.Prompt,
	})

	ctx := context.Background()
	maxRounds := c.cfg.ToolMaxRounds
	if maxRounds <= 0 {
		maxRounds = 8
	}

	var finalText string
	for round := 0; round < maxRounds; round++ {
		body := map[string]any{
			"model":       c.cfg.GatewayAPIModel,
			"messages":    messages,
			"temperature": 0.2,
		}
		if c.tools != nil && c.cfg.ToolsEnabled {
			body["tools"] = c.tools.OpenAITools()
			body["tool_choice"] = "auto"
		}

		rawResp, err := c.postChat(body)
		if err != nil {
			return Response{}, err
		}

		msg := rawResp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			finalText = strings.TrimSpace(msg.Content)
			if finalText == "" {
				return Response{}, fmt.Errorf("gateway API returned empty content")
			}
			break
		}

		// Append assistant tool-call message.
		assistantMsg := map[string]any{
			"role":       "assistant",
			"content":    msg.Content,
			"tool_calls": msg.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		for _, tc := range msg.ToolCalls {
			name := tc.Function.Name
			args := tc.Function.Arguments
			log.Printf("[tools] call %s args=%s", name, truncateForLog(args, 200))
			result, execErr := c.tools.Execute(ctx, name, args)
			if execErr != nil {
				result = mustJSONError(execErr.Error())
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      result,
			})
		}
	}

	if finalText == "" {
		return Response{}, fmt.Errorf("tool loop exceeded max rounds (%d) without a final answer", maxRounds)
	}

	c.mu.Lock()
	history = append(history,
		map[string]any{"role": "user", "content": req.Prompt},
		map[string]any{"role": "assistant", "content": finalText},
	)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	c.histories[conversationID] = history
	c.mu.Unlock()

	return Response{
		Channel:        c.ID(),
		Text:           finalText,
		ConversationID: conversationID,
	}, nil
}

type chatCompletionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *gatewayChannel) postChat(body map[string]any) (*chatCompletionResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.cfg.GatewayAPIBaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.GatewayAPIKey)
	if c.cfg.GatewayAPIExtraHeaderName != "" && c.cfg.GatewayAPIExtraHeaderValue != "" {
		httpReq.Header.Set(c.cfg.GatewayAPIExtraHeaderName, c.cfg.GatewayAPIExtraHeaderValue)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("gateway decode: %w", err)
	}
	if resp.StatusCode >= 300 {
		msg := resp.Status
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("gateway API error: %s", msg)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("gateway API returned no choices")
	}
	return &parsed, nil
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func mustJSONError(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}
