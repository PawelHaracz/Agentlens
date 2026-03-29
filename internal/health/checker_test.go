package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/health"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func insertAgent(t *testing.T, s store.Store, id, endpoint string) *model.Agent {
	t.Helper()
	now := time.Now().UTC()
	a := &model.Agent{
		ID:        id,
		Name:      "Test Agent",
		Protocol:  model.ProtocolA2A,
		Endpoint:  endpoint,
		Status:    model.StatusUnknown,
		Source:    model.SourcePush,
		LastSeen:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, s.Create(context.Background(), a))
	return a
}

func TestChecker_Healthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestStore(t)
	insertAgent(t, s, "h1", ts.URL)

	checker := health.NewChecker(s, 100*time.Millisecond, 5*time.Second, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go checker.Run(ctx)
	time.Sleep(300 * time.Millisecond)

	a, err := s.Get(context.Background(), "h1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusHealthy, a.Status)
}

func TestChecker_Down(t *testing.T) {
	s := newTestStore(t)
	insertAgent(t, s, "d1", "http://localhost:19999") // nothing running here

	checker := health.NewChecker(s, 100*time.Millisecond, 500*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go checker.Run(ctx)
	time.Sleep(1 * time.Second)

	a, err := s.Get(context.Background(), "d1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDown, a.Status)
}
