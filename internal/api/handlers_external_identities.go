package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// ExternalIdentityHandler handles pending federated identity approval/rejection.
type ExternalIdentityHandler struct {
	identityStore *store.UserExternalIdentityStore
}

// NewExternalIdentityHandler creates the handler.
func NewExternalIdentityHandler(is *store.UserExternalIdentityStore) *ExternalIdentityHandler {
	return &ExternalIdentityHandler{identityStore: is}
}

// ListPending handles GET /api/v1/external-identities/pending.
func (h *ExternalIdentityHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	identities, err := h.identityStore.ListPending(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "listing pending identities", "err", err)
		ErrorResponse(w, http.StatusInternalServerError, "failed to list pending identities")
		return
	}
	var result any = identities
	if identities == nil {
		result = []model.UserExternalIdentity{}
	}
	JSONResponse(w, http.StatusOK, result)
}

// Approve handles POST /api/v1/external-identities/{id}/approve.
func (h *ExternalIdentityHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		UserID *string `json:"user_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // optional body

	if err := h.identityStore.Approve(r.Context(), id, req.UserID); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to approve identity")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reject handles POST /api/v1/external-identities/{id}/reject.
func (h *ExternalIdentityHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.identityStore.Reject(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to reject identity")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
