package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// RoleHandler handles role management endpoints.
type RoleHandler struct {
	roleStore *store.RoleStore
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(roleStore *store.RoleStore) *RoleHandler {
	return &RoleHandler{roleStore: roleStore}
}

// List handles GET /api/v1/roles.
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roleStore.List(r.Context())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to list roles")
		return
	}
	if roles == nil {
		roles = []model.Role{}
	}
	JSONResponse(w, http.StatusOK, roles)
}

type createRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// Create handles POST /api/v1/roles.
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}

	// Check uniqueness.
	existing, err := h.roleStore.GetByName(r.Context(), req.Name)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		ErrorResponse(w, http.StatusConflict, "role name already exists")
		return
	}

	now := time.Now().UTC()
	role := &model.Role{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		Permissions: model.JSONSlice(req.Permissions),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.roleStore.Create(r.Context(), role); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create role")
		return
	}

	JSONResponse(w, http.StatusCreated, role)
}

type updateRoleRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Permissions []string `json:"permissions"`
}

// Update handles PUT /api/v1/roles/{id}.
func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role, err := h.roleStore.GetByID(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get role")
		return
	}
	if role == nil {
		ErrorResponse(w, http.StatusNotFound, "role not found")
		return
	}

	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}
	if req.Permissions != nil {
		role.Permissions = model.JSONSlice(req.Permissions)
	}
	role.UpdatedAt = time.Now().UTC()

	if err := h.roleStore.Update(r.Context(), role); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	JSONResponse(w, http.StatusOK, role)
}

// Delete handles DELETE /api/v1/roles/{id}.
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	role, err := h.roleStore.GetByID(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get role")
		return
	}
	if role == nil {
		ErrorResponse(w, http.StatusNotFound, "role not found")
		return
	}

	if role.IsSystem {
		ErrorResponse(w, http.StatusBadRequest, "cannot delete system role")
		return
	}

	if err := h.roleStore.Delete(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to delete role")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
