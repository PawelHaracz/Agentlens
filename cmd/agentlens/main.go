// Command agentlens starts the AgentLens agent catalog server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/health"
	"github.com/PawelHaracz/agentlens/internal/server"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func main() {
	portFlag := flag.Int("port", 0, "HTTP port (overrides config)")
	configFlag := flag.String("config", "", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *portFlag != 0 {
		cfg.Port = *portFlag
	}

	// Setup structured logging
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	fmt.Printf("\n  AgentLens listening on :%d\n\n", cfg.Port)

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data dir", "err", err)
		os.Exit(1)
	}

	// Open SQLite store
	dbPath := filepath.Join(cfg.DataDir, "agentlens.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		slog.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build discovery sources
	var sources []discovery.Source
	if len(cfg.Sources) > 0 {
		sources = append(sources, discovery.NewStaticSource(cfg.Sources))
	}

	// Start discovery manager
	if len(sources) > 0 {
		mgr := discovery.NewManager(sources, s, cfg.PollInterval)
		go func() {
			if err := mgr.Run(ctx); err != nil {
				slog.Error("discovery manager error", "err", err)
			}
		}()
	}

	// Start health checker
	if cfg.HealthCheck.Enabled {
		hc := health.NewChecker(s,
			cfg.HealthCheck.Interval,
			cfg.HealthCheck.Timeout,
			cfg.HealthCheck.Concurrency,
		)
		go func() {
			if err := hc.Run(ctx); err != nil {
				slog.Error("health checker error", "err", err)
			}
		}()
	}

	// Start HTTP server
	router := api.NewRouter(s)
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, router)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
