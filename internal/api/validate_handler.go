package api

import (
	"io"
	"net/http"

	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

// ValidateAgentCard handles POST /api/v1/catalog/validate.
func (h *Handler) ValidateAgentCard(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	result := a2a.ValidateCard(raw)

	if result.Valid {
		JSONResponse(w, http.StatusOK, result)
	} else {
		JSONResponse(w, http.StatusUnprocessableEntity, result)
	}
}
