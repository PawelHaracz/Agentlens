package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/model"
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

	// Look up parser via kernel (microkernel pattern).
	parser, ok := h.parsers.Parser(model.Protocol(protocol))
	if !ok {
		ErrorResponse(w, http.StatusBadRequest, "unsupported protocol: "+protocol)
		return
	}

	// Validate the card using the parser's own validation.
	vr := parser.Validate(result.RawJSON)
	if !vr.Valid {
		JSONResponse(w, http.StatusUnprocessableEntity, vr)
		return
	}

	// Parse the card into an AgentType.
	agentType, err := parser.Parse(result.RawJSON)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.registerAgentType(r.Context(), agentType, result.RawJSON, model.SourcePush)
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

	slog.InfoContext(r.Context(), "card imported from url",
		"url", req.URL,
		"protocol", protocol,
		"entry_id", entry.ID,
	)

	JSONResponse(w, http.StatusCreated, entry)
}
