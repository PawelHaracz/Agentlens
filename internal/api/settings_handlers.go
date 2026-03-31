package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/store"
)

// SettingsHandler handles settings endpoints.
type SettingsHandler struct {
	settingsStore *store.SettingsStore
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(settingsStore *store.SettingsStore) *SettingsHandler {
	return &SettingsHandler{settingsStore: settingsStore}
}

// GetAll handles GET /api/v1/settings.
func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetAll(r.Context())
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	JSONResponse(w, http.StatusOK, settings)
}

// GetByCategory handles GET /api/v1/settings/{category}.
func (h *SettingsHandler) GetByCategory(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	settings, err := h.settingsStore.GetByCategory(r.Context(), category)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	JSONResponse(w, http.StatusOK, settings)
}

// Update handles PUT /api/v1/settings.
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for key, value := range updates {
		if err := h.settingsStore.Set(r.Context(), key, value); err != nil {
			ErrorResponse(w, http.StatusInternalServerError, "failed to update setting: "+key)
			return
		}
	}

	JSONResponse(w, http.StatusOK, map[string]string{"message": "settings updated"})
}
