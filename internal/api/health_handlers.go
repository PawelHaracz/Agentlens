package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// HealthProber is implemented by plugins/health.Plugin.
// Defined here to avoid the api package importing the plugins package.
type HealthProber interface {
	ProbeEntry(ctx context.Context, id string) (model.Health, error)
}

// HealthHandler handles lifecycle and on-demand probe endpoints.
type HealthHandler struct {
	store       store.Store
	prober      HealthProber // may be nil if health check is disabled
	rateLimiter *probeRateLimiter
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(s store.Store, prober HealthProber) *HealthHandler {
	return &HealthHandler{
		store:       s,
		prober:      prober,
		rateLimiter: &probeRateLimiter{lastCall: make(map[string]time.Time)},
	}
}

// PatchLifecycle handles PATCH /api/v1/catalog/{id}/lifecycle.
// Allowed states: "deprecated", "active".
func (h *HealthHandler) PatchLifecycle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	state := model.LifecycleState(body.State)
	switch state {
	case model.LifecycleDeprecated, model.LifecycleActive:
		// valid manual transitions
	default:
		ErrorResponse(w, http.StatusBadRequest, "state must be one of: deprecated, active")
		return
	}

	entry, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get entry")
		return
	}
	if entry == nil {
		ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
		return
	}

	if err := h.store.SetLifecycle(r.Context(), id, state); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to update lifecycle state")
		return
	}

	// Audit log — the enterprise audit plugin is currently a stub, so we log via slog.
	// TODO: integrate with enterprise audit plugin hooks when they are implemented.
	slog.Info("lifecycle state changed",
		"entry_id", id,
		"new_state", string(state),
		"previous_state", string(entry.Status),
	)

	updated, err := h.store.Get(r.Context(), id)
	if err != nil || updated == nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to retrieve updated entry")
		return
	}
	JSONResponse(w, http.StatusOK, updated)
}

// ProbeEntry handles POST /api/v1/catalog/{id}/probe.
// Rate-limited to one call per entry per 5 seconds.
func (h *HealthHandler) ProbeEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !h.rateLimiter.allow(id, 5*time.Second) {
		ErrorResponse(w, http.StatusTooManyRequests, "probe rate limit: max 1 request per entry per 5s")
		return
	}

	if h.prober == nil {
		ErrorResponse(w, http.StatusServiceUnavailable, "health prober not available")
		return
	}

	health, err := h.prober.ProbeEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrEntryNotFound) {
			ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
			return
		}
		slog.Error("on-demand probe failed", "entry_id", id, "err", err)
		ErrorResponse(w, http.StatusInternalServerError, "probe failed")
		return
	}

	JSONResponse(w, http.StatusOK, healthToDTO(health))
}

// healthToDTO converts a model.Health to the JSON response shape.
func healthToDTO(h model.Health) map[string]any {
	return map[string]any{
		"state":               string(h.State),
		"lastProbedAt":        h.LastProbedAt,
		"lastSuccessAt":       h.LastSuccessAt,
		"latencyMs":           h.LatencyMs,
		"consecutiveFailures": h.ConsecutiveFailures,
		"lastError":           h.LastError,
	}
}

// probeRateLimiter tracks last probe call time per entry ID.
// Stale entries (older than the window) are periodically evicted to keep the
// map bounded in long-running deployments.
type probeRateLimiter struct {
	mu          sync.Mutex
	lastCall    map[string]time.Time
	lastCleanup time.Time
}

func (r *probeRateLimiter) allow(id string, window time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Evict expired entries whenever the cleanup interval has elapsed.
	if r.lastCleanup.IsZero() || now.Sub(r.lastCleanup) >= window {
		for entryID, last := range r.lastCall {
			if now.Sub(last) >= window {
				delete(r.lastCall, entryID)
			}
		}
		r.lastCleanup = now
	}

	if last, ok := r.lastCall[id]; ok && now.Sub(last) < window {
		return false
	}

	r.lastCall[id] = now
	return true
}
