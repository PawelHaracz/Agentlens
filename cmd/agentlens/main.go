// Command agentlens starts the AgentLens agent catalog server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	otelslogbridge "go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/server"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
	cardstorePlugin "github.com/PawelHaracz/agentlens/plugins/cardstore"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/audit"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/postgres"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/rbac"
	"github.com/PawelHaracz/agentlens/plugins/enterprise/sso"
	healthplugin "github.com/PawelHaracz/agentlens/plugins/health"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	portFlag := flag.Int("port", 0, "HTTP port (overrides config)")
	configFlag := flag.String("config", "", "Path to config file")
	flag.Parse()

	// 1. Load config
	cfg, err := config.Load(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *portFlag != 0 {
		cfg.Port = *portFlag
	}

	// 2. Setup structured logging (stdout JSON — baseline, always present)
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

	// 3. Init telemetry (before plugins)
	telProvider, err := telemetry.Init(context.Background(), cfg.Telemetry, version)
	if err != nil {
		slog.Error("failed to initialize telemetry", "err", err)
		os.Exit(1)
	}

	// 4. If enabled, replace slog.Default with fan-out handler (stdout + OTel bridge)
	if cfg.Telemetry.Enabled && telProvider.LoggerProvider != nil {
		exportLevel := parseSlogLevel(cfg.Telemetry.LogExportLevel)
		bridgeHandler := otelslogbridge.NewHandler("agentlens",
			otelslogbridge.WithLoggerProvider(telProvider.LoggerProvider))
		fanout := telemetry.NewFanoutHandler(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}),
			bridgeHandler,
			exportLevel,
		)
		slog.SetDefault(slog.New(fanout))
	}

	fmt.Printf("\n  AgentLens listening on :%d\n\n", cfg.Port)

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data dir", "err", err)
		os.Exit(1)
	}

	// 6. Open DB based on config dialect
	var database *db.DB
	switch db.Dialect(cfg.Database.Dialect) {
	case db.DialectSQLite:
		dbPath := cfg.Database.SQLite.Path
		if dbPath == "" {
			dbPath = filepath.Join(cfg.DataDir, "agentlens.db")
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			slog.Error("failed to create db directory", "err", err)
			os.Exit(1)
		}
		database, err = db.Open(db.DialectSQLite, dbPath)
	case db.DialectPostgres:
		database, err = db.Open(db.DialectPostgres, cfg.Database.Postgres.DSN())
	default:
		fmt.Fprintf(os.Stderr, "unsupported database dialect: %s\n", cfg.Database.Dialect)
		os.Exit(1)
	}
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer func() {
		sqlDB, err := database.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}()

	// Build dbPingFn for readyz health check
	sqlDB, err := database.DB.DB()
	if err != nil {
		slog.Error("failed to get underlying sql.DB", "err", err)
		os.Exit(1)
	}
	dbPingFn := func() error {
		return sqlDB.PingContext(context.Background())
	}

	// Run migrations
	migrator := db.NewMigrator(database, db.AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	// 7. Bootstrap admin
	userStore := store.NewUserStore(database)
	password, err := auth.BootstrapAdmin(context.Background(), userStore)
	if err != nil {
		slog.Error("failed to bootstrap admin", "err", err)
		os.Exit(1)
	}
	if password != "" {
		// Print credentials to stdout (NOT slog) to avoid log-aggregation exposure.
		// nosemgrep: go/clear-text-logging
		_, _ = os.Stdout.WriteString("============================================\n")
		_, _ = os.Stdout.WriteString("  INITIAL ADMIN CREDENTIALS\n")
		_, _ = os.Stdout.WriteString("  Username: admin\n")
		_, _ = os.Stdout.WriteString("  Password: " + password + "\n")
		_, _ = os.Stdout.WriteString("  CHANGE THIS PASSWORD IMMEDIATELY\n")
		_, _ = os.Stdout.WriteString("============================================\n")
	}

	// 8. Init stores
	catalogStore := store.NewSQLStore(database)
	roleStore := store.NewRoleStore(database)
	settingsStore := store.NewSettingsStore(database)

	// Register catalog entry gauge (async metric, updated at each metrics interval)
	if err := telemetry.RegisterCatalogGauge(func(ctx context.Context) map[string]int64 {
		stats, err := catalogStore.Stats(ctx)
		if err != nil {
			return nil
		}
		counts := make(map[string]int64)
		for status, count := range stats.ByStatus {
			counts["all:"+status] = int64(count)
		}
		return counts
	}); err != nil {
		slog.Warn("failed to register catalog gauge", "err", err)
	}

	// 9. Init JWT service
	jwtService := auth.NewJWTService(auth.JWTConfig{
		Secret:        cfg.Auth.JWTSecret,
		Expiration:    cfg.Auth.SessionDuration,
		RefreshWindow: time.Hour,
	})

	// Validate license
	lic := kernel.ValidateLicense(cfg.LicenseKey)
	core := kernel.NewCore(catalogStore, cfg, logger, lic)

	// 10. Plugin manager setup
	pm := kernel.NewPluginManager(core)

	// Core plugins
	pm.Register(cardstorePlugin.New(database))
	pm.Register(a2aplugin.New())
	pm.Register(mcpplugin.New())

	var healthPlugin *healthplugin.Plugin
	if cfg.HealthCheck.Enabled {
		healthPlugin = healthplugin.New(cfg.HealthCheck)
		pm.Register(healthPlugin)
	}
	// healthPlugin may be nil when health checks are disabled; RouterDeps accepts nil.

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

	// 5. Defer telemetry shutdown (runs AFTER plugin stop due to LIFO order)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown error", "err", err)
		}
	}()

	defer func() {
		if err := pm.StopAll(ctx); err != nil {
			slog.Error("failed to stop plugins", "err", err)
		}
	}()

	// 12. Discovery sources
	var sources []discovery.Source
	if len(cfg.Sources) > 0 {
		sources = append(sources, discovery.NewStaticSource(cfg.Sources))
	}
	if cfg.Kubernetes.Enabled {
		k8sClient, err := buildK8sClient()
		if err != nil {
			slog.Error("failed to create kubernetes client", "err", err)
			os.Exit(1)
		}
		namespaces := cfg.Kubernetes.Namespaces
		if len(namespaces) == 0 {
			namespaces = []string{"default"}
		}
		sources = append(sources, discovery.NewK8sSource(k8sClient, namespaces))
	}

	// Start discovery manager
	if len(sources) > 0 {
		mgr := discovery.NewManager(sources, catalogStore, cfg.PollInterval)
		// Wire card store into discovery manager if plugin loaded.
		if core.CardStore() != nil {
			mgr.SetCardStore(core.CardStore())
		}
		go func() {
			if err := mgr.Run(ctx); err != nil {
				slog.Error("discovery manager error", "err", err)
			}
		}()
	}

	// 13. Create router with full RouterDeps & 14. HTTP server with graceful shutdown
	routerDeps := api.RouterDeps{
		Kernel:               core,
		UserStore:            userStore,
		RoleStore:            roleStore,
		SettingsStore:        settingsStore,
		JWTService:           jwtService,
		PromHandler:          telProvider.PromHandler,
		ReadyzPing:           dbPingFn,
		TelemetryEnabled:     cfg.Telemetry.Enabled,
		TelemetryEndpoint:    cfg.Telemetry.Endpoint,
		TelemetryServiceName: "agentlens-web",
	}
	if healthPlugin != nil {
		routerDeps.HealthProber = healthPlugin
	}
	router := api.NewRouter(routerDeps)
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, router)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// parseSlogLevel parses a log level string into a slog.Level value.
// Defaults to slog.LevelInfo for unrecognised strings.
func parseSlogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// buildK8sClient creates a Kubernetes client using in-cluster config first,
// falling back to kubeconfig (KUBECONFIG env var or default location).
func buildK8sClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("unable to determine home directory for kubeconfig: %w", err)
			}
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("building kubernetes config: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}
	return client, nil
}
