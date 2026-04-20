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

	"net/http"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/api/middleware"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/auth/apikey"
	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/auth/federation"
	"github.com/PawelHaracz/agentlens/internal/auth/federation/dex"
	"github.com/PawelHaracz/agentlens/internal/auth/ratelimit"
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
	mcpserverplugin "github.com/PawelHaracz/agentlens/plugins/mcpserver"
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
	dbPingFn := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return sqlDB.PingContext(ctx)
	}

	// Run migrations
	migrator := db.NewMigrator(database, db.AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	// 7. Bootstrap admin
	userStore := store.NewUserStore(database)
	partyStore := store.NewPartyStore(database)
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

		// Create Person party for the newly bootstrapped admin user.
		// CreatePersonForUser centralizes the display_name-or-username name
		// formula and is idempotent, so re-runs (e.g. if a Person already
		// exists with the admin's user_id) are safe no-ops.
		adminUser, uErr := userStore.GetByUsername(context.Background(), "admin")
		if uErr == nil && adminUser != nil {
			if pErr := partyStore.CreatePersonForUser(context.Background(), adminUser); pErr != nil {
				slog.Warn("failed to create Person party for bootstrap admin", "err", pErr)
			}
		}
	}

	// 8. Init stores
	catalogStore := store.NewSQLStore(database)
	catalogStore.WithPartyStore(partyStore)
	roleStore := store.NewRoleStore(database)
	settingsStore := store.NewSettingsStore(database)

	// Wrap store with tracing decorator (no-op spans when telemetry disabled)
	tracedStore := telemetry.NewTracedStore(catalogStore, string(db.Dialect(cfg.Database.Dialect)))

	// Register catalog entry gauge (async metric, updated at each metrics interval)
	if err := telemetry.RegisterCatalogGauge(func(ctx context.Context) map[string]int64 {
		stats, err := tracedStore.Stats(ctx)
		if err != nil {
			return nil
		}
		counts := make(map[string]int64)
		for status, count := range stats.ByStatus {
			counts[status] = int64(count)
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
	core := kernel.NewCore(tracedStore, cfg, slog.Default(), lic)

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

	// Shared stores and credcache used by both the MCP plugin wiring
	// and the admin REST handlers — MUST be a single instance so that
	// credential invalidation from admin handlers reaches the MCP auth path.
	credStore := store.NewApiClientCredentialStore(database)
	extIdentityStore := store.NewUserExternalIdentityStore(database)
	apiCredCache := credcache.New()

	// MCP Discovery Server plugin (F.9 composition-root wiring).
	var mcpPlugin *mcpserverplugin.Plugin
	if cfg.MCP.Enabled {
		sessionStore := store.NewMCPSessionStore(database)
		mcpPlugin = mcpserverplugin.New(sessionStore)
		pm.Register(mcpPlugin)
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

	// F.9: Wrap MCP plugin handler BEFORE the router is built so NewRouter
	// mounts the wrapped (origin → auth → scope → transport) version at /api/mcp.
	if mcpPlugin != nil && mcpPlugin.Handler() != nil {
		wrapMCPHandler(ctx, mcpPlugin, mcpWireDeps{
			core:       core,
			cfg:        cfg,
			jwtService: jwtService,
			credStore:  credStore,
			credCache:  apiCredCache,
		})
	}

	// F.6: If federation enabled, extend readyz to include Dex JWKS reachability.
	if cfg.Federation.Enabled && cfg.Federation.Dex.JWKSURL != "" {
		origPing := dbPingFn
		dbPingFn = func(rCtx context.Context) error {
			if err := origPing(rCtx); err != nil {
				return err
			}
			return checkDexHealth(rCtx, cfg.Federation.Dex.JWKSURL)
		}
	}

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
		mgr := discovery.NewManager(sources, tracedStore, cfg.PollInterval)
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
	// credStore/extIdentityStore/apiCredCache were created earlier so the MCP
	// plugin wiring and the admin handlers share a single credcache instance.
	routerDeps := api.RouterDeps{
		Kernel:                core,
		UserStore:             userStore,
		RoleStore:             roleStore,
		SettingsStore:         settingsStore,
		JWTService:            jwtService,
		PartyStore:            partyStore,
		CredStore:             credStore,
		CredCache:             apiCredCache,
		ExternalIdentityStore: extIdentityStore,
		PromHandler:           telProvider.PromHandler,
		ReadyzPing:            dbPingFn,
		TelemetryEnabled:      cfg.Telemetry.Enabled,
		TelemetryEndpoint:     frontendTelemetryEndpoint(cfg.Telemetry),
		TelemetryServiceName:  "agentlens-web",
	}
	if healthPlugin != nil {
		routerDeps.HealthProber = healthPlugin
	}
	router := api.NewRouter(routerDeps)

	// F.9 (cont.): Loopback targets the root router so MCP tools can invoke
	// /api/v1/* in-process. Must run AFTER api.NewRouter.
	if mcpPlugin != nil {
		setupMCPLoopback(mcpPlugin, router)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, router)
	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func frontendTelemetryEndpoint(cfg config.TelemetryConfig) string {
	if cfg.FrontendEndpoint != "" {
		return cfg.FrontendEndpoint
	}
	if cfg.Protocol != "http" || cfg.Endpoint == "" {
		return ""
	}
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if strings.HasSuffix(endpoint, "/v1/traces") {
		return endpoint
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint + "/v1/traces"
	}
	if cfg.Insecure {
		return "http://" + endpoint + "/v1/traces"
	}
	return "https://" + endpoint + "/v1/traces"
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

// mcpWireDeps bundles the shared services needed to wrap the MCP handler
// and build its loopback. Kept as a struct so the composition root does not
// exceed the 5-parameter function limit.
type mcpWireDeps struct {
	core       kernel.Kernel
	cfg        *config.Config
	jwtService *auth.JWTService
	credStore  *store.ApiClientCredentialStore
	credCache  *credcache.Cache
}

// wrapMCPHandler builds the MCP middleware chain and re-registers the wrapped
// transport handler in the kernel (overwriting the raw transport installed by
// Plugin.Init). Called BEFORE api.NewRouter so the router mounts the wrapped
// handler. Dex provider is constructed and registered when federation is on.
func wrapMCPHandler(ctx context.Context, plugin *mcpserverplugin.Plugin, deps mcpWireDeps) {
	limiter := ratelimit.New()
	keyValidator := apikey.New(deps.credStore, deps.credCache, limiter)

	fedReg := buildFederationRegistry(ctx, deps.cfg)

	authMW := middleware.AuthDispatch(keyValidator, deps.jwtService, fedReg, nil)
	originMW := middleware.OriginValidation(deps.cfg.MCP.AllowedOrigins)
	scopeMW := middleware.ScopeByAccessibleProjects

	transport := plugin.Handler()
	wrapped := originMW(authMW(scopeMW(transport)))
	deps.core.RegisterRoutes("/api/mcp", wrapped)

	if deps.cfg.Federation.Enabled && deps.cfg.Federation.Dex.Issuer != "" && deps.cfg.MCP.PublicURL != "" {
		prmHandler := api.NewPRMHandler(deps.cfg.MCP.PublicURL, deps.cfg.Federation.Dex.Issuer)
		deps.core.RegisterRoutes("/.well-known/oauth-protected-resource", prmHandler)
	}
}

// setupMCPLoopback points the plugin's loopback function at the root API
// router so MCP tools can invoke /api/v1/* routes in-process. Called AFTER
// api.NewRouter so the root router is available.
func setupMCPLoopback(plugin *mcpserverplugin.Plugin, rootRouter http.Handler) {
	apiLB := api.BuildLoopbackFunc(rootRouter)
	plugin.SetLoopback(func(ctx context.Context, method, path, query string) ([]byte, int, error) {
		return apiLB(ctx, method, path, query)
	})
}

// buildFederationRegistry constructs and registers the configured federation
// provider (Dex in v1). Returns nil when federation is disabled or the
// provider cannot be constructed (logged).
func buildFederationRegistry(ctx context.Context, cfg *config.Config) *federation.Registry {
	if !cfg.Federation.Enabled || cfg.Federation.Provider == "" {
		return nil
	}
	reg := federation.NewRegistry()
	if cfg.Federation.Provider == "dex" && cfg.Federation.Dex.Issuer != "" {
		dexProvider, err := dex.New(ctx, cfg.Federation.Dex, cfg.Federation.Audience)
		if err != nil {
			slog.WarnContext(ctx, "mcp: dex provider init failed; federation auth will fail", "err", err)
			return reg
		}
		reg.Register("dex", dexProvider)
		slog.InfoContext(ctx, "mcp: dex federation provider registered", "issuer", cfg.Federation.Dex.Issuer)
	}
	return reg
}

// checkDexHealth verifies the Dex JWKS endpoint is reachable. Used by the
// extended readyz chain when federation is enabled.
func checkDexHealth(ctx context.Context, jwksURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return fmt.Errorf("dex health: building request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("dex health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dex health: JWKS returned %d", resp.StatusCode)
	}
	return nil
}
