package models

import (
	"fmt"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

func Create(cfg *config.Config) (Channel, error) {
	switch cfg.ModelChannel {
	case config.ModelGatewayAPI:
		return newGatewayChannel(cfg), nil
	case config.ModelCodexLocal:
		return newCodexLocalChannel(cfg), nil
	default:
		return nil, fmt.Errorf("unknown model channel: %s", cfg.ModelChannel)
	}
}

func List() []config.ModelChannel {
	return []config.ModelChannel{config.ModelGatewayAPI, config.ModelCodexLocal}
}
