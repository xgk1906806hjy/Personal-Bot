package models

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/tools"
)

func Create(cfg *config.Config, workdirs tools.WorkdirStore) (Channel, error) {
	switch cfg.ModelChannel {
	case config.ModelGatewayAPI:
		var registry *tools.Registry
		if cfg.ToolsEnabled {
			r, err := tools.NewRegistry(tools.Options{
				Workspace:      cfg.ToolWorkspace,
				CommandTimeout: time.Duration(cfg.ToolCommandTimeoutSec) * time.Second,
				MaxOutputBytes: cfg.ToolMaxOutputBytes,
				EnableShell:    true,
				EnableWeb:      cfg.WebToolsEnabled,
				HTTPTimeout:    time.Duration(cfg.ToolHTTPTimeoutSec) * time.Second,
				Workdirs:       workdirs,
			})
			if err != nil {
				return nil, fmt.Errorf("tools registry: %w", err)
			}
			registry = r
			log.Printf("[boot] tools enabled workspace=%s tools=%v", r.WorkspaceRoot(), r.ToolNames())
		} else {
			log.Printf("[boot] tools disabled")
		}
		return newGatewayChannel(cfg, registry), nil
	case config.ModelCodexLocal:
		return newCodexLocalChannel(cfg), nil
	case config.ModelCursorLocal:
		log.Printf("[boot] cursor_local cwd=%s script=%s model=%s",
			cfg.ToolWorkspace, cfg.CursorAgentScript, cfg.CursorModel)
		return newCursorLocalChannel(cfg), nil
	default:
		return nil, fmt.Errorf("unknown model channel: %s", cfg.ModelChannel)
	}
}

func List() []config.ModelChannel {
	return []config.ModelChannel{config.ModelGatewayAPI, config.ModelCodexLocal, config.ModelCursorLocal}
}

// CreateCursorOptional returns a secondary cursor_local channel when CURSOR_API_KEY
// is set and the primary MODEL_CHANNEL is not already cursor_local.
// Used for the /cursor slash bypass alongside gateway_api or codex_local.
func CreateCursorOptional(cfg *config.Config) Channel {
	if cfg == nil || strings.TrimSpace(cfg.CursorAPIKey) == "" {
		return nil
	}
	if cfg.ModelChannel == config.ModelCursorLocal {
		return nil
	}
	log.Printf("[boot] /cursor bypass ready cwd=%s script=%s model=%s",
		cfg.ToolWorkspace, cfg.CursorAgentScript, cfg.CursorModel)
	return newCursorLocalChannel(cfg)
}
