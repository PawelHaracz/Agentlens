package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// RegisterAgentCard handles POST /api/v1/catalog/register.
// It accepts a raw A2A agent card JSON, validates and parses it via the
// protocol parser (Product Archetype: raw card → AgentType → CatalogEntry), and persists
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

	// Phase 2: Parse the validated card into an AgentType via the parser plugin.
	agentType, err := parser.Parse(raw)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.registerAgentType(r.Context(), agentType, raw, model.SourcePush)
	if err != nil {
		switch {
		case errors.Is(err, errDuplicateEndpoint):
			ErrorResponse(w, http.StatusConflict, err.Error())
		case errors.Is(err, errUpsertProvider):
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		default:
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	JSONResponse(w, http.StatusCreated, entry)
}
