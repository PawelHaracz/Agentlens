package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// UserHandler handles user management endpoints.
type UserHandler struct {
	userStore  *store.UserStore
	roleStore  *store.RoleStore
	partyStore *store.PartyStore
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userStore *store.UserStore, roleStore *store.RoleStore, partyStore *store.PartyStore) *UserHandler {
	return &UserHandler{
		userStore:  userStore,
		roleStore:  roleStore,
		partyStore: partyStore,
	}
}

// List handles GET /api/v1/users.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.userStore.List(r.Context(), 0, 0)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	JSONResponse(w, http.StatusOK, users)
}

type createUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	RoleID      string `json:"role_id"`
}

// syncPersonOrRollback creates the Person party for a freshly inserted user.
// On failure, compensates the user insert via userStore.Delete so the
// User↔Person 1:1 invariant (ADR-011) holds at rest — migration008 is a
// one-time backfill that won't sweep runtime-created orphans. No-op when
// no PartyStore is wired (test harness that doesn't care about parties).
func (h *UserHandler) syncPersonOrRollback(ctx context.Context, user *model.User) error {
	if h.partyStore == nil {
		return nil
	}
	if err := h.partyStore.CreatePersonForUser(ctx, user); err != nil {
		slog.Error("creating person for new user, rolling back user", "user_id", user.ID, "err", err)
		if delErr := h.userStore.Delete(ctx, user.ID); delErr != nil {
			slog.Error("rollback: delete user failed", "user_id", user.ID, "err", delErr)
		}
		return err
	}
	return nil
}

// Create handles POST /api/v1/users.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Username) == "" {
		ErrorResponse(w, http.StatusBadRequest, "username is required")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		ErrorResponse(w, http.StatusBadRequest, "password is required")
		return
	}
	if strings.TrimSpace(req.RoleID) == "" {
		ErrorResponse(w, http.StatusBadRequest, "role_id is required")
		return
	}

	// Check username uniqueness.
	existing, err := h.userStore.GetByUsername(r.Context(), req.Username)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		ErrorResponse(w, http.StatusConflict, "username already exists")
		return
	}

	// Validate role exists.
	role, err := h.roleStore.GetByID(r.Context(), req.RoleID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}
	if role == nil {
		ErrorResponse(w, http.StatusBadRequest, "role not found")
		return
	}

	// Validate and hash password.
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: hash,
		RoleID:       req.RoleID,
		IsActive:     true,
	}

	if err := h.userStore.Create(r.Context(), user); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if err := h.syncPersonOrRollback(r.Context(), user); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Re-fetch to get the role preloaded.
	created, err := h.userStore.GetByID(r.Context(), user.ID)
	if err != nil || created == nil {
		JSONResponse(w, http.StatusCreated, user)
		return
	}
	JSONResponse(w, http.StatusCreated, created)
}

// Get handles GET /api/v1/users/{id}.
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.userStore.GetByID(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}
	JSONResponse(w, http.StatusOK, user)
}

type updateUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	RoleID      *string `json:"role_id"`
	IsActive    *bool   `json:"is_active"`
}

// Update handles PUT /api/v1/users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.userStore.GetByID(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.RoleID != nil {
		// Validate role exists.
		role, err := h.roleStore.GetByID(r.Context(), *req.RoleID)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "internal error")
			return
		}
		if role == nil {
			ErrorResponse(w, http.StatusBadRequest, "role not found")
			return
		}
		user.RoleID = *req.RoleID
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if err := h.userStore.Update(r.Context(), user); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	if h.partyStore != nil && req.DisplayName != nil {
		if err := h.partyStore.UpdatePersonForUser(r.Context(), user); err != nil {
			slog.Error("syncing person name", "user_id", user.ID, "err", err)
		}
	}

	// Re-fetch to get the role preloaded.
	updated, err := h.userStore.GetByID(r.Context(), user.ID)
	if err != nil || updated == nil {
		JSONResponse(w, http.StatusOK, user)
		return
	}
	JSONResponse(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/users/{id}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Cannot delete yourself.
	if id == UserIDFromContext(r.Context()) {
		ErrorResponse(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	user, err := h.userStore.GetByID(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	// Check if this is the last admin by checking if any other users share the admin role.
	if user.Role != nil && user.Role.Name == "admin" {
		users, err := h.userStore.List(r.Context(), 0, 0)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "internal error")
			return
		}
		adminCount := 0
		for _, u := range users {
			if u.RoleID == user.RoleID && u.IsActive {
				adminCount++
			}
		}
		if adminCount <= 1 {
			ErrorResponse(w, http.StatusBadRequest, "cannot delete the last admin user")
			return
		}
	}

	if err := h.userStore.Delete(r.Context(), id); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
