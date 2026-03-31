// Command agentlens starts the AgentLens agent catalog server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/db"
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

	// 1. Load config
	cfg, err := config.Load(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *portFlag != 0 {
		cfg.Port = *portFlag
	}

	// 2. Setup structured logging
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

	// 3. Open DB based on config dialect
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
			sqlDB.Close()
		}
	}()

	// 4. Run migrations
	migrator := db.NewMigrator(database, db.AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	// 5. Bootstrap admin
	userStore := store.NewUserStore(database)
	password, err := auth.BootstrapAdmin(context.Background(), userStore)
	if err != nil {
		slog.Error("failed to bootstrap admin", "err", err)
		os.Exit(1)
	}
	if password != "" {
		fmt.Println("============================================")
		fmt.Println("  INITIAL ADMIN CREDENTIALS")
		fmt.Println("  Username: admin")
		fmt.Printf("  Password: %s\n", password)
		fmt.Println("  CHANGE THIS PASSWORD IMMEDIATELY")
		fmt.Println("============================================")
	}

	// 6. Init stores
	catalogStore := store.NewSQLStore(database)
	roleStore := store.NewRoleStore(database)
	settingsStore := store.NewSettingsStore(database)

	// 7. Init JWT service
	jwtService := auth.NewJWTService(auth.JWTConfig{
		Secret:        cfg.Auth.JWTSecret,
		Expiration:    cfg.Auth.SessionDuration,
		RefreshWindow: time.Hour,
	})

	// Validate license
	lic := kernel.ValidateLicense(cfg.LicenseKey)
	core := kernel.NewCore(catalogStore, cfg, logger, lic)

	// 9. Plugin manager setup
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
	defer func() {
		if err := pm.StopAll(ctx); err != nil {
			slog.Error("failed to stop plugins", "err", err)
		}
	}()

	// 10. Discovery sources
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
		go func() {
			if err := mgr.Run(ctx); err != nil {
				slog.Error("discovery manager error", "err", err)
			}
		}()
	}

	// 8. Create router with full RouterDeps & 11. HTTP server with graceful shutdown
	router := api.NewRouter(api.RouterDeps{
		Store:         catalogStore,
		UserStore:     userStore,
		RoleStore:     roleStore,
		SettingsStore: settingsStore,
		JWTService:    jwtService,
	})
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, router)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
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
