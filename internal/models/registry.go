package models

import (
	"fmt"
	"log"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/tools"
)

func Create(cfg *config.Config) (Channel, error) {
	switch cfg.ModelChannel {
	case config.ModelGatewayAPI:
		var registry *tools.Registry
		if cfg.ToolsEnabled {
			r, err := tools.NewRegistry(tools.Options{
				Workspace:      cfg.ToolWorkspace,
				CommandTimeout: time.Duration(cfg.ToolCommandTimeoutSec) * time.Second,
				MaxOutputBytes: cfg.ToolMaxOutputBytes,
				EnableShell:    true,
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
	default:
		return nil, fmt.Errorf("unknown model channel: %s", cfg.ModelChannel)
	}
}

func List() []config.ModelChannel {
	return []config.ModelChannel{config.ModelGatewayAPI, config.ModelCodexLocal}
}
