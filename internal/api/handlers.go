package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Handler holds dependencies for all API handlers.
type Handler struct {
	store store.Store
}

// NewHandler creates a new Handler with the given store.
func NewHandler(s store.Store) *Handler {
	return &Handler{store: s}
}

// Healthz handles GET /healthz.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListAgents handles GET /api/v1/agents.
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	filter := store.ListFilter{}
	q := r.URL.Query()

	if v := q.Get("protocol"); v != "" {
		p := model.Protocol(v)
		filter.Protocol = &p
	}
	if v := q.Get("status"); v != "" {
		s := model.Status(v)
		filter.Status = &s
	}
	if v := q.Get("source"); v != "" {
		s := model.SourceType(v)
		filter.Source = &s
	}
	if v := q.Get("team"); v != "" {
		filter.Team = v
	}
	if v := q.Get("q"); v != "" {
		filter.Query = v
	}
	if v := q.Get("tags"); v != "" {
		filter.Tags = strings.Split(v, ",")
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	agents, err := h.store.List(r.Context(), filter)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	if agents == nil {
		agents = []model.Agent{}
	}
	JSONResponse(w, http.StatusOK, agents)
}

// GetAgent handles GET /api/v1/agents/{id}.
func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		ErrorResponse(w, http.StatusNotFound, "agent not found")
		return
	}
	JSONResponse(w, http.StatusOK, agent)
}

// CreateAgent handles POST /api/v1/agents.
func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var agent model.Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now().UTC()
	agent.ID = uuid.NewString()
	agent.Source = model.SourcePush
	agent.Status = model.StatusUnknown
	agent.CreatedAt = now
	agent.UpdatedAt = now
	agent.LastSeen = now

	if err := h.store.Create(r.Context(), &agent); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create agent")
		return
	}
	JSONResponse(w, http.StatusCreated, agent)
}

// DeleteAgent handles DELETE /api/v1/agents/{id}.
func (h *Handler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		ErrorResponse(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAgentCard handles GET /api/v1/agents/{id}/card.
func (h *Handler) GetAgentCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	if agent == nil {
		ErrorResponse(w, http.StatusNotFound, "agent not found")
		return
	}
	if len(agent.RawCard) == 0 {
		ErrorResponse(w, http.StatusNotFound, "no card available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(agent.RawCard)
}

// SearchSkills handles GET /api/v1/skills.
func (h *Handler) SearchSkills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	agents, err := h.store.SearchSkills(r.Context(), q)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to search skills")
		return
	}
	if agents == nil {
		agents = []model.Agent{}
	}
	JSONResponse(w, http.StatusOK, agents)
}

// GetStats handles GET /api/v1/stats.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	JSONResponse(w, http.StatusOK, stats)
}
