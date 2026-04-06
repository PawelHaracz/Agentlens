package api

import (
	"io"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// ValidateAgentCard handles POST /api/v1/catalog/validate.
func (h *Handler) ValidateAgentCard(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer func() { _ = r.Body.Close() }()

	parser, ok := h.parsers.Parser(model.ProtocolA2A)
	if !ok {
		ErrorResponse(w, http.StatusInternalServerError, "a2a parser not available")
		return
	}

	result := parser.Validate(raw)
	if result.Valid {
		JSONResponse(w, http.StatusOK, result)
	} else {
		JSONResponse(w, http.StatusUnprocessableEntity, result)
	}
}
