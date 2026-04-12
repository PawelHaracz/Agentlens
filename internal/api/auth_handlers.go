package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userStore   *store.UserStore
	roleStore   *store.RoleStore
	jwt         *auth.JWTService
	authMetrics *telemetry.AuthMetrics
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userStore *store.UserStore, roleStore *store.RoleStore, jwt *auth.JWTService) *AuthHandler {
	return &AuthHandler{
		userStore:   userStore,
		roleStore:   roleStore,
		jwt:         jwt,
		authMetrics: telemetry.NewAuthMetrics(),
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

// recordAuthOutcome records a span event and auth metric for a login attempt.
func recordAuthOutcome(ctx context.Context, metrics *telemetry.AuthMetrics, username, result, reason string) {
	trace.SpanFromContext(ctx).AddEvent("auth.login", trace.WithAttributes(
		attribute.String("username", username),
		attribute.String("result", result),
		attribute.String("reason", reason),
	))
	metrics.RecordLogin(ctx, result, reason)
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		ErrorResponse(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.userStore.GetByUsername(r.Context(), req.Username)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		recordAuthOutcome(r.Context(), h.authMetrics, req.Username, "failure", "invalid_credentials")
		ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Check if the account is active.
	if !user.IsActive {
		recordAuthOutcome(r.Context(), h.authMetrics, req.Username, "failure", "account_disabled")
		ErrorResponse(w, http.StatusForbidden, "account is disabled")
		return
	}

	// Check if the account is locked.
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		recordAuthOutcome(r.Context(), h.authMetrics, req.Username, "failure", "account_locked")
		ErrorResponse(w, http.StatusLocked, "account is locked, try again later")
		return
	}

	// Check password.
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		_ = h.userStore.IncrementFailedAttempts(r.Context(), user.ID)
		recordAuthOutcome(r.Context(), h.authMetrics, req.Username, "failure", "invalid_credentials")
		ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Successful authentication.
	recordAuthOutcome(r.Context(), h.authMetrics, req.Username, "success", "")
	_ = h.userStore.ResetFailedAttempts(r.Context(), user.ID)
	_ = h.userStore.UpdateLastLogin(r.Context(), user.ID)

	// Get role for token generation.
	role, err := h.roleStore.GetByID(r.Context(), user.RoleID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := h.jwt.GenerateToken(user, role)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set httpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "agentlens_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	JSONResponse(w, http.StatusOK, loginResponse{
		Token: token,
		User:  user,
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Clear the auth cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "agentlens_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	})
	JSONResponse(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	user, err := h.userStore.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		ErrorResponse(w, http.StatusUnauthorized, "user not found")
		return
	}

	if !user.IsActive {
		ErrorResponse(w, http.StatusUnauthorized, "account is disabled")
		return
	}

	role, err := h.roleStore.GetByID(r.Context(), user.RoleID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}

	token, err := h.jwt.GenerateToken(user, role)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "agentlens_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	JSONResponse(w, http.StatusOK, map[string]string{"token": token})
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	user, err := h.userStore.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	JSONResponse(w, http.StatusOK, user)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles PUT /api/v1/auth/password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		ErrorResponse(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	user, err := h.userStore.GetByID(r.Context(), userID)
	if err != nil || user == nil {
		ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	if !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		ErrorResponse(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	user.PasswordHash = hash
	if err := h.userStore.Update(r.Context(), user); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	JSONResponse(w, http.StatusOK, map[string]string{"message": "password changed"})
}
