package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/web"
)

// RouterDeps holds all dependencies for the router.
type RouterDeps struct {
	Store         store.Store
	UserStore     *store.UserStore
	RoleStore     *store.RoleStore
	SettingsStore *store.SettingsStore
	JWTService    *auth.JWTService
}

// NewRouter creates and returns a configured chi router with all routes.
func NewRouter(deps RouterDeps) *chi.Mux {
	h := NewHandler(deps.Store)
	r := chi.NewRouter()

	r.Use(RecoveryMiddleware)
	r.Use(LoggerMiddleware)
	r.Use(CORSMiddleware)
	r.Use(chiMiddleware.RequestID)

	r.Get("/healthz", h.Healthz)

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes.
		if deps.JWTService != nil {
			if deps.UserStore == nil || deps.RoleStore == nil {
				panic("JWTService requires UserStore and RoleStore to be provided")
			}
			authHandler := NewAuthHandler(deps.UserStore, deps.RoleStore, deps.JWTService)
			r.Post("/auth/login", authHandler.Login)
			r.Post("/auth/logout", authHandler.Logout)

			// Protected auth routes.
			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(deps.JWTService))
				r.Post("/auth/refresh", authHandler.Refresh)
				r.Get("/auth/me", authHandler.Me)
				r.Put("/auth/password", authHandler.ChangePassword)
			})

			// Protected catalog routes.
			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(deps.JWTService))
				r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog", h.ListCatalog)
				r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog", h.CreateEntry)
				r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog/{id}", h.GetEntry)
				r.With(RequirePermission(auth.PermCatalogDelete)).Delete("/catalog/{id}", h.DeleteEntry)
				r.With(RequirePermission(auth.PermCatalogRead)).Get("/catalog/{id}/card", h.GetEntryCard)
				r.With(RequirePermission(auth.PermCatalogRead)).Get("/skills", h.SearchSkills)
				r.With(RequirePermission(auth.PermCatalogRead)).Get("/stats", h.GetStats)
			})

			// Protected user management routes.
			if deps.UserStore != nil && deps.RoleStore != nil {
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

			// Protected settings routes.
			if deps.SettingsStore != nil {
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
		} else {
			// No auth configured — register catalog routes without protection.
			r.Get("/catalog", h.ListCatalog)
			r.Post("/catalog", h.CreateEntry)
			r.Get("/catalog/{id}", h.GetEntry)
			r.Delete("/catalog/{id}", h.DeleteEntry)
			r.Get("/catalog/{id}/card", h.GetEntryCard)
			r.Get("/skills", h.SearchSkills)
			r.Get("/stats", h.GetStats)
		}
	})

	// Serve SPA — all non-/api paths fall back to index.html for client routing.
	if staticFS, err := web.FS(); err == nil {
		r.Handle("/*", spaHandler(staticFS))
	}

	return r
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
