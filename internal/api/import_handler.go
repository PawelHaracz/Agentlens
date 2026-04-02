package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	"github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

type importRequest struct {
	URL      string  `json:"url"`
	Protocol *string `json:"protocol,omitempty"`
}

// ImportCatalogEntry handles POST /api/v1/catalog/import.
// It fetches an agent card from the provided URL, parses it, and registers it.
func (h *Handler) ImportCatalogEntry(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate URL before attempting any network access.
	if err := h.cardFetcher.ValidateURL(req.URL); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.cardFetcher.Fetch(r.Context(), req.URL)
	if err != nil {
		ErrorResponse(w, http.StatusBadGateway, "could not fetch card from url: "+err.Error())
		return
	}

	// Determine which protocol to use.
	protocol := result.DetectedProtocol
	if req.Protocol != nil && *req.Protocol != "" {
		protocol = *req.Protocol
	}
	if protocol == "" {
		ErrorResponse(w, http.StatusBadRequest, "could not detect protocol; specify 'protocol' in the request body")
		return
	}

	// Parse the card with the appropriate parser.
	var entry *model.CatalogEntry
	switch model.Protocol(protocol) {
	case model.ProtocolA2A:
		vr := a2a.ValidateCard(result.RawJSON)
		if !vr.Valid {
			JSONResponse(w, http.StatusUnprocessableEntity, vr)
			return
		}
		parser := a2a.New()
		entry, err = parser.Parse(result.RawJSON, model.SourcePush)
		if err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
	case model.ProtocolMCP:
		parser := mcp.New()
		entry, err = parser.Parse(result.RawJSON, model.SourcePush)
		if err != nil {
			ErrorResponse(w, http.StatusUnprocessableEntity, "invalid mcp card: "+err.Error())
			return
		}
	default:
		ErrorResponse(w, http.StatusBadRequest, "protocol must be one of: a2a, mcp, a2ui")
		return
	}

	// Assign server-managed fields.
	now := time.Now().UTC()
	entry.ID = uuid.NewString()
	entry.Source = model.SourcePush
	entry.Status = model.StatusUnknown
	entry.CreatedAt = now
	entry.UpdatedAt = now
	entry.Validity.LastSeen = now

	if err := h.store.Create(r.Context(), entry); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			ErrorResponse(w, http.StatusConflict, "an entry with this endpoint already exists")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, "failed to create catalog entry")
		return
	}

	slog.InfoContext(r.Context(), "card imported from url",
		"url", req.URL,
		"protocol", protocol,
		"entry_id", entry.ID,
	)

	JSONResponse(w, http.StatusCreated, entry)
}
