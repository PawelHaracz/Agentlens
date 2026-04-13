package api

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/service"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/web"
)

// RouterDeps holds all dependencies for the router.
type RouterDeps struct {
	Kernel        kernel.Kernel
	UserStore     *store.UserStore
	RoleStore     *store.RoleStore
	SettingsStore *store.SettingsStore
	JWTService    *auth.JWTService
	// CardFetcher is optional. When nil, a default CardFetcher with SSRF protection is used.
	CardFetcher service.Fetcher
	// HealthProber is optional; enables POST /catalog/{id}/probe.
	HealthProber HealthProber
	// PromHandler is optional. When non-nil, GET /metrics is registered.
	PromHandler http.Handler
	// ReadyzPing is optional. When non-nil, GET /readyz is registered.
	ReadyzPing func(context.Context) error
	// TelemetryEnabled controls the /api/v1/telemetry/config response.
	TelemetryEnabled bool
	// TelemetryEndpoint is the OTLP collector endpoint exposed to the frontend.
	TelemetryEndpoint string
	// TelemetryServiceName is the service name exposed to the frontend.
	TelemetryServiceName string
}

// NewRouter creates and returns a configured HTTP handler with all routes.
// The handler is wrapped with otelhttp for request tracing.
func NewRouter(deps RouterDeps) http.Handler {
	h := NewHandler(deps.Kernel)
	if deps.CardFetcher != nil {
		h.cardFetcher = deps.CardFetcher
	}
	r := chi.NewRouter()

	r.Use(RecoveryMiddleware)
	r.Use(LoggerMiddleware)
	r.Use(CORSMiddleware)
	r.Use(chiMiddleware.RequestID)
	r.Use(routePatternSpanNameMiddleware)

	r.Get("/healthz", h.Healthz)

	if deps.ReadyzPing != nil {
		r.Get("/readyz", NewReadyzHandler(deps.ReadyzPing))
	}

	if deps.PromHandler != nil {
		r.Handle("/metrics", deps.PromHandler)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/telemetry/config", NewTelemetryConfigHandler(
			deps.TelemetryEnabled,
			deps.TelemetryEndpoint,
			deps.TelemetryServiceName,
		))

		if deps.JWTService != nil {
			if deps.UserStore == nil || deps.RoleStore == nil {
				panic("JWTService requires UserStore and RoleStore to be provided")
			}
			authHandler := NewAuthHandler(deps.UserStore, deps.RoleStore, deps.JWTService)
			registerAuthRoutes(r, deps, authHandler)
			registerCatalogRoutes(r, h, deps)
			registerUserRoutes(r, deps)
			registerSettingsRoutes(r, deps)
		} else {
			// No auth configured — register catalog routes without protection.
			registerUnauthenticatedCatalogRoutes(r, h, deps)
		}
	})

	// Serve SPA — all non-/api paths fall back to index.html for client routing.
	if staticFS, err := web.FS(); err == nil {
		r.Handle("/*", spaHandler(staticFS))
	}

	return otelhttp.NewHandler(r, "agentlens.http")
}

func routePatternSpanNameMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if span == nil || !span.IsRecording() {
			next.ServeHTTP(w, r)
			return
		}
		span.SetName(r.Method + " " + r.URL.Path)
		next.ServeHTTP(w, r)
		rctx := chi.RouteContext(r.Context())
		if rctx == nil {
			return
		}
		routePattern := rctx.RoutePattern()
		if routePattern == "" {
			return
		}
		span.SetName(r.Method + " " + routePattern)
		span.SetAttributes(semconv.HTTPRoute(routePattern))
	})
}

// registerAuthRoutes mounts public and protected auth endpoints.
func registerAuthRoutes(r chi.Router, deps RouterDeps, authHandler *AuthHandler) {
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/logout", authHandler.Logout)

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(deps.JWTService))
		r.Post("/auth/refresh", authHandler.Refresh)
		r.Get("/auth/me", authHandler.Me)
		r.Put("/auth/password", authHandler.ChangePassword)
	})
}

// registerCatalogRoutes mounts catalog endpoints behind auth middleware.
func registerCatalogRoutes(r chi.Router, h *Handler, deps RouterDeps) {
	hh := NewHealthHandler(deps.Kernel.Store(), deps.HealthProber)
	capHandler := NewCapabilityHandler(deps.Kernel.Store())
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(deps.JWTService))
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog", h.ListCatalog)
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog", h.CreateEntry)
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog/validate", h.ValidateAgentCard)
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog/register", h.RegisterAgentCard)
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog/import", h.ImportCatalogEntry)
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog/{id}", h.GetEntry)
		r.With(RequirePermission(auth.PermCatalogDelete)).Delete("/catalog/{id}", h.DeleteEntry)
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog/{id}/card", h.GetEntryCard)
		r.With(RequirePermission(auth.PermCatalogWrite)).Patch("/catalog/{id}/lifecycle", hh.PatchLifecycle)
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog/{id}/probe", hh.ProbeEntry)
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities", capHandler.ListCapabilities)
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities/{key}", capHandler.GetCapabilityAgents)
		r.With(RequirePermission(auth.PermCatalogRead)).Get("/stats", h.GetStats)
	})
}

// registerUserRoutes mounts user and role management endpoints behind auth middleware.
func registerUserRoutes(r chi.Router, deps RouterDeps) {
	if deps.UserStore == nil || deps.RoleStore == nil {
		return
	}
	userHandler := NewUserHandler(deps.UserStore, deps.RoleStore)
	roleHandler := NewRoleHandler(deps.RoleStore)

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(deps.JWTService))

		r.Route("/users", func(r chi.Router) {
			r.With(RequirePermission(auth.PermUsersRead)).Get("/", userHandler.List)
			r.With(RequirePermission(auth.PermUsersWrite)).Post("/", userHandler.Create)
			r.With(RequirePermission(auth.PermUsersRead)).Get("/{id}", userHandler.Get)
			r.With(RequirePermission(auth.PermUsersWrite)).Put("/{id}", userHandler.Update)
			r.With(RequirePermission(auth.PermUsersDelete)).Delete("/{id}", userHandler.Delete)
		})

		r.Route("/roles", func(r chi.Router) {
			r.With(RequirePermission(auth.PermRolesRead)).Get("/", roleHandler.List)
			r.With(RequirePermission(auth.PermRolesWrite)).Post("/", roleHandler.Create)
			r.With(RequirePermission(auth.PermRolesWrite)).Put("/{id}", roleHandler.Update)
			r.With(RequirePermission(auth.PermRolesWrite)).Delete("/{id}", roleHandler.Delete)
		})
	})
}

// registerSettingsRoutes mounts settings endpoints behind auth middleware.
func registerSettingsRoutes(r chi.Router, deps RouterDeps) {
	if deps.SettingsStore == nil {
		return
	}
	settingsHandler := NewSettingsHandler(deps.SettingsStore)

	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(deps.JWTService))

		r.Route("/settings", func(r chi.Router) {
			r.With(RequirePermission(auth.PermSettingsRead)).Get("/", settingsHandler.GetAll)
			r.With(RequirePermission(auth.PermSettingsRead)).Get("/{category}", settingsHandler.GetByCategory)
			r.With(RequirePermission(auth.PermSettingsWrite)).Put("/", settingsHandler.Update)
		})
	})
}

// registerUnauthenticatedCatalogRoutes mounts catalog endpoints without authentication.
func registerUnauthenticatedCatalogRoutes(r chi.Router, h *Handler, deps RouterDeps) {
	hh := NewHealthHandler(h.store, deps.HealthProber)
	ch := NewCapabilityHandler(h.store)
	r.Get("/catalog", h.ListCatalog)
	r.Post("/catalog", h.CreateEntry)
	r.Post("/catalog/validate", h.ValidateAgentCard)
	r.Post("/catalog/register", h.RegisterAgentCard)
	r.Post("/catalog/import", h.ImportCatalogEntry)
	r.Get("/catalog/{id}", h.GetEntry)
	r.Delete("/catalog/{id}", h.DeleteEntry)
	r.Get("/catalog/{id}/card", h.GetEntryCard)
	r.Patch("/catalog/{id}/lifecycle", hh.PatchLifecycle)
	r.Post("/catalog/{id}/probe", hh.ProbeEntry)
	r.Get("/stats", h.GetStats)
	r.Get("/capabilities", ch.ListCapabilities)
	r.Get("/capabilities/{key}", ch.GetCapabilityAgents)
}

// spaHandler serves static files and falls back to index.html for client-side routing.
func spaHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))
	return func(w http.ResponseWriter, r *http.Request) {
		// Don't serve /api paths through the SPA handler.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Try serving the actual file; fall back to index.html.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(staticFS, path); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}
