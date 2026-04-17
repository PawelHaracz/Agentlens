package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/store"
)

type assignProjectRequest struct {
	ProjectID string `json:"project_id"`
}

// AssignCatalogToProjectHandler assigns a catalog entry to an additional project.
// Requires catalog:write globally.
func AssignCatalogToProjectHandler(ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalogID := chi.URLParam(r, "id")
		var req assignProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" {
			ErrorResponse(w, http.StatusBadRequest, "project_id is required")
			return
		}
		if err := ps.AssignToProject(r.Context(), catalogID, req.ProjectID); err != nil {
			slog.Error("assigning catalog to project", "err", err)
			ErrorResponse(w, http.StatusInternalServerError, "failed to assign")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// RemoveCatalogFromProjectHandler removes a catalog entry from a project.
func RemoveCatalogFromProjectHandler(ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalogID := chi.URLParam(r, "id")
		projectID := chi.URLParam(r, "projectID")
		if err := ps.RemoveFromProject(r.Context(), catalogID, projectID); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to remove")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListCatalogProjectsHandler lists all projects a catalog entry belongs to.
func ListCatalogProjectsHandler(ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalogID := chi.URLParam(r, "id")
		projects, err := ps.ListProjectsForCatalogEntry(r.Context(), catalogID)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		JSONResponse(w, http.StatusOK, projects)
	}
}

// registerCatalogProjectRoutes mounts catalog↔project scoping endpoints.
// Called from registerPartyRoutes — already inside /api/v1 subrouter with auth.
// Uses relative path (no /api/v1/ prefix).
func registerCatalogProjectRoutes(r chi.Router, ps *store.PartyStore) {
	r.Route("/catalog/{id}/projects", func(r chi.Router) {
		r.Get("/", ListCatalogProjectsHandler(ps))
		r.With(RequirePermission(auth.PermCatalogWrite)).Post("/", AssignCatalogToProjectHandler(ps))
		r.With(RequirePermission(auth.PermCatalogWrite)).Delete("/{projectID}", RemoveCatalogFromProjectHandler(ps))
	})
}
