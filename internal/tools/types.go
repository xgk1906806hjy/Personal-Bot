// Package tools 提供 gateway_api 可调用的本地工具（文件、shell、workdir、联网检索）。
// 工具经 OpenAI tools 协议暴露；路径通过 SessionKey 解析到会话级 workdir。
package tools

import (
	"context"
	"encoding/json"
)

// Tool 是暴露给模型的单个可调用函数。
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// OpenAITool 对应 chat.completions 的 tools[] 项。
type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// ToOpenAITools 把内部 Tool 列表转成网关 API 所需的 JSON 形态。
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
