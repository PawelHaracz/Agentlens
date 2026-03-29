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
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/server"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/audit"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/postgres"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/rbac"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/sso"
	healthplugin "github.com/PawelHaracz/agentlens/plugins/health"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
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

	// Validate license
	lic := kernel.ValidateLicense(cfg.LicenseKey)
	core := kernel.NewCore(s, cfg, logger, lic)

	// Create plugin manager and register plugins
	pm := kernel.NewPluginManager(core)

	// Core plugins
	pm.Register(a2aplugin.New())
	pm.Register(mcpplugin.New())

	if cfg.HealthCheck.Enabled {
		pm.Register(healthplugin.New(
			cfg.HealthCheck.Interval,
			cfg.HealthCheck.Timeout,
			cfg.HealthCheck.Concurrency,
		))
	}

	// Enterprise plugins (skipped with warning if no license)
	pm.Register(sso.New())
	pm.Register(rbac.New())
	pm.Register(audit.New())
	pm.Register(postgres.New())

	// Initialize all plugins
	if err := pm.InitAll(); err != nil {
		slog.Error("failed to initialize plugins", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start all plugins
	if err := pm.StartAll(ctx); err != nil {
		slog.Error("failed to start plugins", "err", err)
		os.Exit(1)
	}
	defer pm.StopAll(ctx)

	// Build discovery sources (still use internal/discovery for orchestration)
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

	// Start HTTP server
	router := api.NewRouter(s)
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, router)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
