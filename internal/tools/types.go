package tools

import (
	"context"
	"encoding/json"
)

// Tool is a single callable function exposed to the model.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// OpenAITool is the chat.completions tools[] item shape.
type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

func ToOpenAITools(list []Tool) []OpenAITool {
	out := make([]OpenAITool, 0, len(list))
	for _, t := range list {
		item := OpenAITool{Type: "function"}
		item.Function.Name = t.Name()
		item.Function.Description = t.Description()
		item.Function.Parameters = t.Parameters()
		out = append(out, item)
	}
	return out
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}
