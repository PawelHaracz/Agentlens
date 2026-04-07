package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/service"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Handler holds dependencies for all API handlers.
type Handler struct {
	store       store.Store
	parsers     kernel.Kernel
	cardFetcher service.Fetcher
}

// NewHandler creates a new Handler with the given kernel.
func NewHandler(k kernel.Kernel) *Handler {
	return &Handler{
		store:       k.Store(),
		parsers:     k,
		cardFetcher: service.NewCardFetcher(),
	}
}

// Healthz handles GET /healthz.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListCatalog handles GET /api/v1/catalog.
func (h *Handler) ListCatalog(w http.ResponseWriter, r *http.Request) {
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
	if v := q.Get("categories"); v != "" {
		filter.Categories = strings.Split(v, ",")
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

	entries, err := h.store.List(r.Context(), filter)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to list catalog entries")
		return
	}
	if entries == nil {
		entries = []model.CatalogEntry{}
	}
	JSONResponse(w, http.StatusOK, entries)
}

// GetEntry handles GET /api/v1/catalog/{id}.
func (h *Handler) GetEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get catalog entry")
		return
	}
	if entry == nil {
		ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
		return
	}
	JSONResponse(w, http.StatusOK, entry)
}

// createEntryRequest is the flat request body for POST /api/v1/catalog.
// It accepts the fields previously on CatalogEntry directly, plus the
// AgentType fields (protocol, endpoint, version) for backward compatibility.
type createEntryRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Protocol    string `json:"protocol"`
	Endpoint    string `json:"endpoint"`
	Version     string `json:"version"`
}

// CreateEntry handles POST /api/v1/catalog.
func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	var req createEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields.
	if strings.TrimSpace(req.DisplayName) == "" {
		ErrorResponse(w, http.StatusBadRequest, "display_name is required")
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		ErrorResponse(w, http.StatusBadRequest, "endpoint is required")
		return
	}
	switch model.Protocol(req.Protocol) {
	case model.ProtocolA2A, model.ProtocolMCP, model.ProtocolA2UI:
		// valid
	default:
		ErrorResponse(w, http.StatusBadRequest, "protocol must be one of: a2a, mcp, a2ui")
		return
	}

	// Reject duplicate endpoints before attempting to insert.
	existing, err := h.store.FindByEndpoint(r.Context(), req.Endpoint)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to check for existing entry")
		return
	}
	if existing != nil {
		ErrorResponse(w, http.StatusConflict, "an entry with this endpoint already exists")
		return
	}

	now := time.Now().UTC()

	agentType := &model.AgentType{
		ID:            uuid.NewString(),
		Protocol:      model.Protocol(req.Protocol),
		Endpoint:      req.Endpoint,
		Version:       req.Version,
		RawDefinition: []byte("{}"),
		CreatedOn:     now,
	}
	agentType.AgentKey = model.ComputeAgentKey(agentType.Protocol, agentType.Endpoint)

	entry := &model.CatalogEntry{
		ID:          uuid.NewString(),
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Source:      model.SourcePush,
		Status:      model.LifecycleRegistered,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.Create(r.Context(), entry); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			ErrorResponse(w, http.StatusConflict, "an entry with this endpoint already exists")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, "failed to create catalog entry")
		return
	}
	JSONResponse(w, http.StatusCreated, entry)
}

// DeleteEntry handles DELETE /api/v1/catalog/{id}.
func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get catalog entry")
		return
	}
	if entry == nil {
		ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to delete catalog entry")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetEntryCard handles GET /api/v1/catalog/{id}/card.
// Returns the raw agent card definition stored in the AgentType.
func (h *Handler) GetEntryCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get catalog entry")
		return
	}
	if entry == nil {
		ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
		return
	}
	if entry.AgentType == nil || len(entry.AgentType.RawDefinition) == 0 {
		ErrorResponse(w, http.StatusNotFound, "no card available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(entry.AgentType.RawDefinition); err != nil {
		slog.Error("failed to write card response", "err", err)
	}
}

// SearchCapabilities handles GET /api/v1/skills.
func (h *Handler) SearchCapabilities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	entries, err := h.store.SearchCapabilities(r.Context(), q)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to search capabilities")
		return
	}
	if entries == nil {
		entries = []model.CatalogEntry{}
	}
	JSONResponse(w, http.StatusOK, entries)
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
