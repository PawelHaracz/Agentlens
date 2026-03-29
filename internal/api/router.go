package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/web"
)

// NewRouter creates and returns a configured chi router with all routes.
func NewRouter(s store.Store) *chi.Mux {
	h := NewHandler(s)
	r := chi.NewRouter()

	r.Use(RecoveryMiddleware)
	r.Use(LoggerMiddleware)
	r.Use(CORSMiddleware)
	r.Use(chiMiddleware.RequestID)

	r.Get("/healthz", h.Healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/agents", h.ListAgents)
		r.Post("/agents", h.CreateAgent)
		r.Get("/agents/{id}", h.GetAgent)
		r.Delete("/agents/{id}", h.DeleteAgent)
		r.Get("/agents/{id}/card", h.GetAgentCard)
		r.Get("/skills", h.SearchSkills)
		r.Get("/stats", h.GetStats)
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
