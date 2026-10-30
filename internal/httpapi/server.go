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
