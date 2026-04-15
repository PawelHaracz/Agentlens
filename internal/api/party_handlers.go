package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

type createPartyRequest struct {
	Name string `json:"name"`
}

type addMemberRequest struct {
	PartyID string `json:"party_id"`
	Role    string `json:"role"`
}

// ListPartiesHandler returns all parties of cfg.Kind.
func ListPartiesHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parties, err := ps.ListParties(r.Context(), cfg.Kind)
		if err != nil {
			slog.Error("listing parties", "kind", cfg.Kind, "err", err)
			ErrorResponse(w, http.StatusInternalServerError, "failed to list "+cfg.URLPrefix)
			return
		}
		JSONResponse(w, http.StatusOK, parties)
	}
}

// CreatePartyHandler creates a new party of cfg.Kind.
// Requires cfg.CreatePermission in the caller's global role.
func CreatePartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.CreatePermission) {
			ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		var req createPartyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			ErrorResponse(w, http.StatusBadRequest, "name is required")
			return
		}
		p := &model.Party{ID: uuid.New().String(), Kind: cfg.Kind, Name: req.Name}
		if err := ps.CreateParty(r.Context(), p); err != nil {
			slog.Error("creating party", "kind", cfg.Kind, "err", err)
			ErrorResponse(w, http.StatusInternalServerError, "failed to create")
			return
		}
		JSONResponse(w, http.StatusCreated, p)
	}
}

// GetPartyHandler returns a single party by ID, scoped to cfg.Kind.
func GetPartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "partyID")
		p, err := ps.GetParty(r.Context(), id)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to get")
			return
		}
		if p == nil || p.Kind != cfg.Kind {
			ErrorResponse(w, http.StatusNotFound, "not found")
			return
		}
		JSONResponse(w, http.StatusOK, p)
	}
}

// DeletePartyHandler deletes a non-system party of cfg.Kind.
// Requires cfg.ManagePermission.
func DeletePartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
			ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		id := chi.URLParam(r, "partyID")
		if err := ps.DeleteParty(r.Context(), id); err != nil {
			ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// AddMemberHandler adds a party as a member with a role.
// Validates role against cfg.ValidMemberRoles.
func AddMemberHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	validRoles := make(map[string]bool, len(cfg.ValidMemberRoles))
	for _, r := range cfg.ValidMemberRoles {
		validRoles[r] = true
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
			ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		toPartyID := chi.URLParam(r, "partyID")
		var req addMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.PartyID == "" || req.Role == "" {
			ErrorResponse(w, http.StatusBadRequest, "party_id and role are required")
			return
		}
		if !validRoles[req.Role] {
			ErrorResponse(w, http.StatusBadRequest, "invalid role for this party kind")
			return
		}
		rel := &model.PartyRelationship{
			FromPartyID:      req.PartyID,
			FromRole:         req.Role,
			ToPartyID:        toPartyID,
			ToRole:           string(cfg.Kind),
			RelationshipName: cfg.MemberRelationship,
		}
		if err := ps.AddMember(r.Context(), rel); err != nil {
			slog.Error("adding member", "err", err)
			ErrorResponse(w, http.StatusInternalServerError, "failed to add member")
			return
		}
		JSONResponse(w, http.StatusCreated, rel)
	}
}

// RemoveMemberHandler removes a party from membership.
func RemoveMemberHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
			ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		toPartyID := chi.URLParam(r, "partyID")
		memberPartyID := chi.URLParam(r, "memberPartyID")
		if err := ps.RemoveMember(r.Context(), memberPartyID, toPartyID, cfg.MemberRelationship); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to remove member")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListMembersHandler lists all direct members of a party.
func ListMembersHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		toPartyID := chi.URLParam(r, "partyID")
		rels, err := ps.ListMembers(r.Context(), toPartyID, cfg.MemberRelationship)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to list members")
			return
		}
		JSONResponse(w, http.StatusOK, rels)
	}
}
