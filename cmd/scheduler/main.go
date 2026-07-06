package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
	"github.com/GregSharpe1/plex-series-scheduler/internal/logging"
	"github.com/GregSharpe1/plex-series-scheduler/internal/metrics"
	"github.com/GregSharpe1/plex-series-scheduler/internal/plex"
	"github.com/GregSharpe1/plex-series-scheduler/internal/scheduler"
)

func main() {
	var (
		configPath  string
		metricsAddr string
		runOnce     bool
	)

	flag.StringVar(&configPath, "config", "config.yaml", "Path to the YAML configuration file")
	flag.StringVar(&metricsAddr, "metrics-addr", ":9464", "Address to expose Prometheus metrics on")
	flag.BoolVar(&runOnce, "once", false, "Run the scheduler once and exit")
	flag.Parse()

	logger := logging.New()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := metrics.NewRegistry()
	if metricsAddr != "" {
		go serveMetrics(metricsAddr, registry, logger)
	}

	loader := config.NewLoader(configPath)
	engine := scheduler.New(loader, func(cfg config.PlexConfig) (plex.Client, error) {
		return plex.NewHTTPClient(cfg)
	}, registry, logger)

	if runOnce {
		if err := engine.RunOnce(ctx); err != nil {
			logger.Error("scheduler run failed", slog.Any("error", err))
			os.Exit(1)
		}
		return
	}

	if err := engine.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("scheduler exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func serveMetrics(addr string, registry *metrics.Registry, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", registry.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("metrics server listening", slog.String("addr", addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server failed", slog.Any("error", err))
	}
}
