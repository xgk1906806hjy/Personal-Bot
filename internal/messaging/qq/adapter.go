package qq

import (
	"fmt"
	"log"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
)

type Adapter struct {
	ws *WSClient
}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) ID() config.MessagingPlatform { return config.PlatformQQ }

func (a *Adapter) Mount(ctx messaging.AdapterContext) error {
	api := NewAPIClient(ctx.Config)
	switch ctx.Config.QQTransport {
	case config.QQTransportWebsocket:
		a.ws = NewWSClient(api, ctx.Orchestrator)
		a.ws.Start()
		log.Printf("[boot] messaging adapter mounted: qq → WebSocket gateway (no public domain needed)")
	case config.QQTransportWebhook:
		mountWebhook(ctx.Mux, api, ctx.Orchestrator, ctx.Config.QQAppSecret)
	case config.QQTransportBoth:
		a.ws = NewWSClient(api, ctx.Orchestrator)
		a.ws.Start()
		log.Printf("[boot] messaging adapter mounted: qq → WebSocket gateway (no public domain needed)")
		mountWebhook(ctx.Mux, api, ctx.Orchestrator, ctx.Config.QQAppSecret)
	default:
		return fmt.Errorf("unknown QQ_TRANSPORT: %s", ctx.Config.QQTransport)
	}
	return nil
}

func (a *Adapter) Close() error {
	if a.ws != nil {
		a.ws.Stop()
	}
	return nil
}
