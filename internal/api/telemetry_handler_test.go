package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestReadyz_Healthy(t *testing.T) {
	handler := api.NewReadyzHandler(func() error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok"`)
}

func TestReadyz_Unhealthy(t *testing.T) {
	handler := api.NewReadyzHandler(func() error {
		return fmt.Errorf("connection refused")
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}

func TestTelemetryConfigHandler_Disabled(t *testing.T) {
	handler := api.NewTelemetryConfigHandler(false, "", "agentlens-web")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
}

func TestTelemetryConfigHandler_Enabled(t *testing.T) {
	handler := api.NewTelemetryConfigHandler(true, "http://collector:4318/v1/traces", "agentlens-web")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
	assert.Contains(t, w.Body.String(), `"endpoint"`)
	assert.Contains(t, w.Body.String(), `"serviceName"`)
}
