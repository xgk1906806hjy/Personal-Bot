package messaging

import (
	"net/http"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

type AdapterContext struct {
	Config       *config.Config
	Orchestrator *orchestrator.Orchestrator
	Mux          *http.ServeMux
}

// Adapter mounts platform-specific transports (HTTP webhook and/or long connections).
type Adapter interface {
	ID() config.MessagingPlatform
	Mount(ctx AdapterContext) error
	Close() error
}
