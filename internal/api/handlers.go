package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
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

// CreateEntry handles POST /api/v1/catalog.
// It accepts either a CatalogEntry JSON or a raw A2A agent card JSON.
// Raw agent cards are detected automatically and parsed via the A2A parser.
func (h *Handler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var entry *model.CatalogEntry

	if isAgentCard(raw) {
		parser := a2a.New()
		parsed, parseErr := parser.Parse(raw, model.SourcePush)
		if parseErr != nil {
			ErrorResponse(w, http.StatusBadRequest, parseErr.Error())
			return
		}
		entry = parsed
	} else {
		entry = &model.CatalogEntry{}
		if jsonErr := json.Unmarshal(raw, entry); jsonErr != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Validate required fields.
	if strings.TrimSpace(entry.DisplayName) == "" {
		ErrorResponse(w, http.StatusBadRequest, "display_name is required")
		return
	}
	if strings.TrimSpace(entry.Endpoint) == "" {
		ErrorResponse(w, http.StatusBadRequest, "endpoint is required")
		return
	}
	switch entry.Protocol {
	case model.ProtocolA2A, model.ProtocolMCP, model.ProtocolA2UI:
		// valid
	default:
		ErrorResponse(w, http.StatusBadRequest, "protocol must be one of: a2a, mcp, a2ui")
		return
	}

	now := time.Now().UTC()
	entry.ID = uuid.NewString()
	entry.Source = model.SourcePush
	entry.Status = model.StatusUnknown
	entry.CreatedAt = now
	entry.UpdatedAt = now
	entry.Validity.LastSeen = now

	if err := h.store.Create(r.Context(), entry); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			ErrorResponse(w, http.StatusConflict, "an entry with this endpoint already exists")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, "failed to create catalog entry")
		return
	}
	JSONResponse(w, http.StatusCreated, entry)
}

// isAgentCard checks whether raw JSON is a raw A2A agent card rather than a CatalogEntry.
// A2A cards use "name"/"url"; CatalogEntries use "display_name"/"endpoint".
func isAgentCard(raw []byte) bool {
	var probe struct {
		// CatalogEntry-specific fields (snake_case).
		DisplayName string `json:"display_name"`
		Endpoint    string `json:"endpoint"`
		// A2A card fields.
		Name                      string          `json:"name"`
		URL                       string          `json:"url"`
		SupportsExtendedAgentCard *bool           `json:"supportsExtendedAgentCard"`
		Capabilities              json.RawMessage `json:"capabilities"`
		SupportedInterfaces       json.RawMessage `json:"supportedInterfaces"`
		SecuritySchemes           json.RawMessage `json:"securitySchemes"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	// If it has CatalogEntry-specific fields, treat as CatalogEntry.
	if probe.DisplayName != "" || probe.Endpoint != "" {
		return false
	}
	// If it has A2A-specific fields, treat as agent card.
	if probe.SupportsExtendedAgentCard != nil ||
		len(probe.Capabilities) > 0 ||
		len(probe.SupportedInterfaces) > 0 ||
		len(probe.SecuritySchemes) > 0 {
		return true
	}
	// If it has "name" + "url" (A2A card fields) but no "display_name"/"endpoint", treat as card.
	if probe.Name != "" && probe.URL != "" {
		return true
	}
	return false
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
	if len(entry.RawCard) == 0 {
		ErrorResponse(w, http.StatusNotFound, "no card available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(entry.RawCard); err != nil {
		slog.Error("failed to write card response", "err", err)
	}
}

// SearchSkills handles GET /api/v1/skills.
func (h *Handler) SearchSkills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	entries, err := h.store.SearchSkills(r.Context(), q)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to search skills")
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
