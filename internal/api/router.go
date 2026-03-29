package api

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/PawelHaracz/agentlens/internal/store"
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

	return r
}
