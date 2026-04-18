package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// ServiceAccountHandler handles service-account CRUD + credential rotation.
type ServiceAccountHandler struct {
	partyStore *store.PartyStore
	credStore  *store.ApiClientCredentialStore
	credCache  *credcache.Cache
}

// NewServiceAccountHandler creates a handler. All dependencies are required.
func NewServiceAccountHandler(
	ps *store.PartyStore,
	cs *store.ApiClientCredentialStore,
	cc *credcache.Cache,
) *ServiceAccountHandler {
	return &ServiceAccountHandler{partyStore: ps, credStore: cs, credCache: cc}
}

// List handles GET /api/v1/service-accounts.
func (h *ServiceAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	parties, err := h.partyStore.ListParties(r.Context(), model.PartyKindServiceAccount)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to list service accounts")
		return
	}
	if parties == nil {
		parties = []model.Party{}
	}
	JSONResponse(w, http.StatusOK, parties)
}

// Get handles GET /api/v1/service-accounts/{id}.
func (h *ServiceAccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	party, err := h.partyStore.GetParty(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get service account")
		return
	}
	if party == nil || party.Kind != model.PartyKindServiceAccount {
		ErrorResponse(w, http.StatusNotFound, "service account not found")
		return
	}
	JSONResponse(w, http.StatusOK, party)
}

// Create handles POST /api/v1/service-accounts.
// Returns the one-time secret in the response body; never stored plaintext.
func (h *ServiceAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}

	party, err := h.partyStore.CreateServiceAccount(r.Context(), req.Name)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create service account")
		return
	}

	secret, cred, err := issueCredential(r.Context(), party.ID, h.credStore)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to issue credential")
		return
	}

	JSONResponse(w, http.StatusCreated, map[string]any{
		"party":         party,
		"client_id":     cred.ClientID,
		"secret":        secret, // shown ONCE; never persisted in plaintext
		"secret_format": "agentlens_sk_<client_id>.<secret>",
	})
}

// RotateSecret handles PATCH /api/v1/service-accounts/{id}/secret.
// Returns new one-time secret; 409 on concurrent rotation (M-new-2).
func (h *ServiceAccountHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	newSecret, newCred, err := buildCredential(id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to build credential")
		return
	}
	if err := h.credStore.RotateSecret(r.Context(), id, newCred); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			ErrorResponse(w, http.StatusConflict, "concurrent rotation detected; retry")
			return
		}
		ErrorResponse(w, http.StatusInternalServerError, "rotation failed")
		return
	}
	h.credCache.Invalidate(newCred.ClientID)
	JSONResponse(w, http.StatusOK, map[string]any{
		"client_id": newCred.ClientID,
		"secret":    newSecret,
	})
}

// Delete handles DELETE /api/v1/service-accounts/{id}.
// H6-residual: enumerates active credentials and invalidates credcache BEFORE cascade.
func (h *ServiceAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	clientIDs, err := h.partyStore.EnumerateActiveCredentials(r.Context(), id)
	if err != nil {
		slog.WarnContext(r.Context(), "sa delete: failed to enumerate credentials", "err", err)
	}
	for _, cid := range clientIDs {
		h.credCache.Invalidate(cid)
	}

	if err := h.partyStore.DeleteParty(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to delete service account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueCredential creates a new credential for partyID and returns the one-time secret string.
func issueCredential(ctx context.Context, partyID string, cs *store.ApiClientCredentialStore) (string, *model.ApiClientCredential, error) {
	rawSecret, cred, err := buildCredential(partyID)
	if err != nil {
		return "", nil, err
	}
	if err := cs.Create(ctx, cred); err != nil {
		return "", nil, fmt.Errorf("storing credential: %w", err)
	}
	return rawSecret, cred, nil
}

// buildCredential generates a client_id + bcrypt-hashed secret without persisting.
func buildCredential(partyID string) (rawSecret string, cred *model.ApiClientCredential, err error) {
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("generating client_id: %w", err)
	}
	clientID := hex.EncodeToString(b)[:16]

	sb := make([]byte, 32)
	if _, err = rand.Read(sb); err != nil {
		return "", nil, fmt.Errorf("generating secret: %w", err)
	}
	rawSecret = hex.EncodeToString(sb)

	hash, err := bcrypt.GenerateFromPassword([]byte(rawSecret), 12)
	if err != nil {
		return "", nil, fmt.Errorf("hashing secret: %w", err)
	}

	cred = &model.ApiClientCredential{
		PartyID:    partyID,
		ClientID:   clientID,
		SecretHash: string(hash),
	}
	return fmt.Sprintf("agentlens_sk_%s.%s", clientID, rawSecret), cred, nil
}
