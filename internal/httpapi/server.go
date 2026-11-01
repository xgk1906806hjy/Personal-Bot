// Package httpapi 提供进程级 HTTP 路由（健康检查等）。
// 各消息适配器可在 Mount 时往同一 mux 挂 webhook。
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/xgk1906806hjy/Personal-Bot/internal/app"
	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
	"github.com/xgk1906806hjy/Personal-Bot/internal/models"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

// NewMux 创建根路由；目前仅注册 GET /healthz。
func NewMux(cfg *config.Config, orch *orchestrator.Orchestrator, adapters []messaging.Adapter) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		enabled := make([]config.MessagingPlatform, 0, len(adapters))
		for _, a := range adapters {
			enabled = append(enabled, a.ID())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messaging": map[string]any{
				"enabled": enabled,
				"ready":   app.ReadyMessagingPlatforms(),
			},
			"model": map[string]any{
				"active":    orch.ModelChannelID(),
				"available": models.List(),
			},
			"runtime": "go",
		})
	})
	return mux
}
