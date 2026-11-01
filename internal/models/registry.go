package models

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/tools"
)

// Create 按 MODEL_CHANNEL 构造主模型通道。
// gateway_api 在 TOOLS_ENABLED 时注入 tools.Registry（workdirs 来自 session.Store）。
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

// List 返回已实现的模型通道 id。
func List() []config.ModelChannel {
	return []config.ModelChannel{config.ModelGatewayAPI, config.ModelCodexLocal, config.ModelCursorLocal}
}

// CreateCursorOptional 在配置了 CURSOR_API_KEY 且主通道不是 cursor_local 时，
// 返回供 /cursor 模式使用的旁路通道；否则返回 nil。
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
