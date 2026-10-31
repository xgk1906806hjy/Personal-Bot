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
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/tools"
)

type gatewayChannel struct {
	cfg    *config.Config
	client *http.Client
	tools  *tools.Registry
}

func newGatewayChannel(cfg *config.Config, registry *tools.Registry) *gatewayChannel {
	return &gatewayChannel{
		cfg:    cfg,
		client: &http.Client{Timeout: 180 * time.Second},
		tools:  registry,
	}
}

func (c *gatewayChannel) ID() config.ModelChannel { return config.ModelGatewayAPI }

func (c *gatewayChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	systemPrompt := c.cfg.BotSystemPrompt
	useTools := !req.DisableTools && c.tools != nil && c.cfg.ToolsEnabled
	if useTools {
		systemPrompt += "\n\nYou can use tools on this machine. Default start dir is (" +
			c.tools.WorkspaceRoot() +
			"), but set_workdir may switch to ANY absolute path (any drive). " +
			"When the user asks to open/switch a project: list_dir / set_workdir with the full path, then confirm. " +
			"get_workdir shows the current directory. File/command tools and later /cursor use that workdir. " +
			"For facts, news, or docs you do not know: web_search; for a known URL: fetch_url. " +
			"Available tools: " + strings.Join(c.tools.ToolNames(), ", ") + "."
	}

	messages := make([]map[string]any, 0, len(req.Messages)+2)
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": systemPrompt,
	})
	for _, m := range req.Messages {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		messages = append(messages, map[string]any{
			"role":    role,
			"content": m.Content,
		})
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": req.Prompt,
	})

	ctx := context.Background()
	if req.SessionKey != "" {
		ctx = tools.WithSessionKey(ctx, req.SessionKey)
	}
	maxRounds := c.cfg.ToolMaxRounds
	if maxRounds <= 0 {
		maxRounds = 10
	}
	if !useTools {
		maxRounds = 1
	}

	var finalText string
	var usage TokenUsage
	for round := 0; round < maxRounds; round++ {
		body := map[string]any{
			"model":       c.cfg.GatewayAPIModel,
			"messages":    messages,
			"temperature": 0.2,
		}
		if useTools {
			body["tools"] = c.tools.OpenAITools()
			body["tool_choice"] = "auto"
		}

		rawResp, err := c.postChat(body)
		if err != nil {
			return Response{}, err
		}
		usage = usage.Add(rawResp.Usage.toTokenUsage())

		msg := rawResp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			finalText = strings.TrimSpace(msg.Content)
			if finalText == "" {
				return Response{}, fmt.Errorf("gateway API returned empty content")
			}
			break
		}

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

	return Response{
		Channel:        c.ID(),
		Text:           finalText,
		ConversationID: conversationID,
		Usage:          usage,
	}, nil
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (u apiUsage) toTokenUsage() TokenUsage {
	total := u.TotalTokens
	if total == 0 {
		total = u.PromptTokens + u.CompletionTokens
	}
	return TokenUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      total,
	}
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
	Usage apiUsage `json:"usage"`
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
