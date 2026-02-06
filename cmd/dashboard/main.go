package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"syncthing-dashboard/internal/collector"
	"syncthing-dashboard/internal/config"
	"syncthing-dashboard/internal/demo"
	httpapi "syncthing-dashboard/internal/http"
	"syncthing-dashboard/internal/model"
	"syncthing-dashboard/internal/syncthing"
)

type dashboardService interface {
	Start(context.Context)
	Snapshot() (model.DashboardSnapshot, bool)
	Ready() bool
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("dashboard failed: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var dashboardSvc dashboardService
	if cfg.DemoMode {
		log.Printf("SYNCTHING_BASE_URL is not set; running in demonstration mode")
		dashboardSvc = demo.NewCollector(cfg.PollInterval)
	} else {
		client := syncthing.NewClient(cfg.STBaseURL, cfg.STAPIKey, cfg.STTimeout, cfg.STInsecureSkipVerify)
		dashboardSvc = collector.New(client, cfg.PollInterval)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	dashboardSvc.Start(ctx)

	server := &http.Server{
		Addr:         cfg.HTTPListenAddr,
		Handler:      httpapi.New(dashboardSvc, cfg.PageTitle, cfg.PageSubtitle, cfg.PollInterval),
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
	}

	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		defer close(shutdownDone)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("read-only Syncthing dashboard listening on %s", cfg.HTTPListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-shutdownDone
	return nil
}
