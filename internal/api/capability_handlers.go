package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// CapabilityHandler handles capability discovery endpoints.
type CapabilityHandler struct {
	store store.Store
}

// NewCapabilityHandler creates a new CapabilityHandler.
func NewCapabilityHandler(s store.Store) *CapabilityHandler {
	return &CapabilityHandler{store: s}
}

// ListCapabilities handles GET /api/v1/capabilities.
func (h *CapabilityHandler) ListCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query params
	query := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	sortParam := r.URL.Query().Get("sort")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Validate kind
	if kind != "" {
		discoverableKinds := model.DiscoverableKinds()
		valid := false
		for _, dk := range discoverableKinds {
			if dk == kind {
				valid = true
				break
			}
		}
		if !valid {
			ErrorResponse(w, http.StatusBadRequest, "invalid kind parameter")
			return
		}
	}

	// Validate sort
	sort := "name_asc"
	if sortParam != "" {
		if sortParam != "name_asc" && sortParam != "agentName_asc" {
			ErrorResponse(w, http.StatusBadRequest, "invalid sort parameter")
			return
		}
		sort = sortParam
	}

	// Parse pagination
	limit := 50
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			ErrorResponse(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
	}

	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			ErrorResponse(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
	}

	// Query store
	result, err := h.store.ListCapabilities(ctx, store.CapabilityFilter{
		Query:  query,
		Kind:   kind,
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to list capabilities", "err", err)
		ErrorResponse(w, http.StatusInternalServerError, "failed to list capabilities")
		return
	}

	JSONResponse(w, http.StatusOK, result)
}

// GetCapabilityAgents handles GET /api/v1/capabilities/{key}.
func (h *CapabilityHandler) GetCapabilityAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract key from URL
	key := chi.URLParam(r, "key")
	keyDecoded, err := url.PathUnescape(key)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid key encoding")
		return
	}

	// Split key on first ::
	parts := strings.SplitN(keyDecoded, "::", 2)
	if len(parts) != 2 {
		ErrorResponse(w, http.StatusBadRequest, "key must be in format kind::name")
		return
	}

	kind := parts[0]
	name := parts[1]

	// Query store
	entries, err := h.store.ListAgentsByCapability(ctx, kind, name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query agents for capability", "kind", kind, "name", name, "err", err)
		ErrorResponse(w, http.StatusInternalServerError, "failed to query agents")
		return
	}

	if len(entries) == 0 {
		ErrorResponse(w, http.StatusNotFound, "capability not found")
		return
	}

	// Build response
	agents := make([]capabilityAgentDTO, len(entries))
	for i, entry := range entries {
		var snippet json.RawMessage
		if entry.AgentType != nil {
			snippet = buildCapabilitySnippet(*entry.AgentType, kind, name)
		}

		var provider *model.Provider
		if entry.AgentType != nil {
			provider = entry.AgentType.Provider
		}

		specVersion := ""
		if entry.AgentType != nil {
			specVersion = entry.AgentType.SpecVersion
		}

		var protocol string
		if entry.AgentType != nil {
			protocol = string(entry.AgentType.Protocol)
		}

		entry.SyncFromDB()
		agents[i] = capabilityAgentDTO{
			ID:                entry.ID,
			DisplayName:       entry.DisplayName,
			Protocol:          protocol,
			Provider:          provider,
			Health:            buildHealthJSON(entry),
			SpecVersion:       specVersion,
			Status:            string(entry.Status),
			CapabilitySnippet: snippet,
		}
	}

	response := capabilityDetailResponse{
		Capability: capabilitySummary{
			Kind: kind,
			Name: name,
		},
		Agents: agents,
	}

	JSONResponse(w, http.StatusOK, response)
}

// Response DTOs (unexported)

type capabilityDetailResponse struct {
	Capability capabilitySummary    `json:"capability"`
	Agents     []capabilityAgentDTO `json:"agents"`
}

type capabilitySummary struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type capabilityAgentDTO struct {
	ID                string          `json:"id"`
	DisplayName       string          `json:"display_name"`
	Protocol          string          `json:"protocol"`
	Provider          *model.Provider `json:"provider"`
	Health            map[string]any  `json:"health"`
	SpecVersion       string          `json:"spec_version"`
	Status            string          `json:"status"`
	CapabilitySnippet json.RawMessage `json:"capability_snippet"`
}

func buildCapabilitySnippet(agentType model.AgentType, kind, name string) json.RawMessage {
	for _, cap := range agentType.Capabilities {
		if cap.Kind() != kind {
			continue
		}
		capJSON, err := json.Marshal(cap)
		if err != nil {
			continue
		}
		var capMap map[string]any
		if err := json.Unmarshal(capJSON, &capMap); err != nil {
			continue
		}
		if capName, ok := capMap["name"].(string); ok && capName == name {
			capMap["kind"] = cap.Kind()
			snippet, _ := json.Marshal(capMap)
			return snippet
		}
	}
	return nil
}

func buildHealthJSON(entry model.CatalogEntry) map[string]any {
	return map[string]any{
		"state":               string(entry.Health.State),
		"lastProbedAt":        entry.Health.LastProbedAt,
		"lastSuccessAt":       entry.Health.LastSuccessAt,
		"latencyMs":           entry.Health.LatencyMs,
		"consecutiveFailures": entry.Health.ConsecutiveFailures,
		"lastError":           entry.Health.LastError,
	}
}
