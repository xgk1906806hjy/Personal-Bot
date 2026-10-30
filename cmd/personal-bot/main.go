package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/app"
	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/httpapi"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
	"github.com/xgk1906806hjy/Personal-Bot/internal/models"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
	"github.com/xgk1906806hjy/Personal-Bot/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[boot] config: %v", err)
	}

	model, err := models.Create(cfg)
	if err != nil {
		log.Fatalf("[boot] model: %v", err)
	}
	sessions := session.NewStore(cfg.SessionTTLHours)
	orch := orchestrator.New(cfg, model, sessions)

	adapters, err := app.CreateMessagingAdapters(cfg.MessagingChannels)
	if err != nil {
		log.Fatalf("[boot] messaging: %v", err)
	}

	mux := httpapi.NewMux(cfg, orch, adapters)
	for _, a := range adapters {
		if err := a.Mount(messaging.AdapterContext{
			Config:       cfg,
			Orchestrator: orch,
			Mux:          mux,
		}); err != nil {
			log.Fatalf("[boot] mount %s: %v", a.ID(), err)
		}
	}

	addr := cfg.Host + ":" + cfg.Port
	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("[boot] listening on http://%s", addr)
		log.Printf("[boot] messaging = %v (ready: %v)", cfg.MessagingChannels, app.ReadyMessagingPlatforms())
		log.Printf("[boot] model = %s", cfg.ModelChannel)
		log.Printf("[boot] health = /healthz")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[boot] http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("[boot] shutting down…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	for _, a := range adapters {
		_ = a.Close()
	}
}
