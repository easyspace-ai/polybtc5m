package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/silver/pmvibes/internal/config"
	"github.com/silver/pmvibes/internal/logging"
	"github.com/silver/pmvibes/internal/server"
	"github.com/silver/pmvibes/internal/store"
)

func main() {
	config.LoadDotEnv()

	logger := slog.New(logging.NewHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("shutting down...")
		cancel()
	}()

	cfg := server.Config{
		Port: config.GetEnv("PORT", "8080"),
	}

	eventLog, err := store.NewEventLog(config.GetEnv("REDIS_URL", ""), logger)
	if err != nil {
		slog.Error("event log", "err", err)
		os.Exit(1)
	}
	defer eventLog.Close()

	srv := server.New(cfg, logger, eventLog)

	slog.Info("starting Polymarket 02 server with BTC 5m simulator", "port", cfg.Port)
	slog.Info("finance dashboard UI", "url", "http://localhost:"+cfg.Port+"/")
	slog.Info("view simulation status at", "url", "http://localhost:"+cfg.Port+"/finance")
	slog.Info("persisted audit trail", "url", "http://localhost:"+cfg.Port+"/finance/history/1")
	if config.GetEnv("REDIS_URL", "") != "" {
		slog.Info("simulation events persisted to Redis")
	} else {
		slog.Info("REDIS_URL not set; simulation events kept in process memory only")
	}

	if err := srv.Run(ctx); err != nil && err.Error() != "http: Server closed" {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}


