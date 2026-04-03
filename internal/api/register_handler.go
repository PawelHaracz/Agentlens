package api

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// RegisterAgentCard handles POST /api/v1/catalog/register.
// It accepts a raw A2A agent card JSON, validates and parses it via the
// protocol parser (Product Archetype: raw card → CatalogEntry), and persists
// the resulting CatalogEntry.
func (h *Handler) RegisterAgentCard(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer func() { _ = r.Body.Close() }()

	if len(raw) == 0 {
		ErrorResponse(w, http.StatusBadRequest, "request body is empty")
		return
	}

	// Look up the A2A parser via kernel (microkernel pattern).
	parser, ok := h.parsers.Parser(model.ProtocolA2A)
	if !ok {
		ErrorResponse(w, http.StatusInternalServerError, "a2a parser not available")
		return
	}

	// Phase 1: Validate the card structure.
	result := parser.Validate(raw)
	if !result.Valid {
		JSONResponse(w, http.StatusUnprocessableEntity, result)
		return
	}

	// Phase 2: Parse the validated card into a CatalogEntry via the parser plugin.
	entry, err := parser.Parse(raw, model.SourcePush)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
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

	JSONResponse(w, http.StatusCreated, entry)
}
