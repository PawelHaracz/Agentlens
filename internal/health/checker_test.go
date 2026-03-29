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

func insertEntry(t *testing.T, s store.Store, id, endpoint string) *model.CatalogEntry {
	t.Helper()
	now := time.Now().UTC()
	e := &model.CatalogEntry{
		ID:          id,
		DisplayName: "Test Entry",
		Protocol:    model.ProtocolA2A,
		Endpoint:    endpoint,
		Status:      model.StatusUnknown,
		Source:      model.SourcePush,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, s.Create(context.Background(), e))
	return e
}

func TestChecker_Healthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestStore(t)
	insertEntry(t, s, "h1", ts.URL)

	checker := health.NewChecker(s, 100*time.Millisecond, 5*time.Second, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go func() {
		_ = checker.Run(ctx)
	}()
	time.Sleep(300 * time.Millisecond)

	e, err := s.Get(context.Background(), "h1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusHealthy, e.Status)
}

func TestChecker_Down(t *testing.T) {
	s := newTestStore(t)
	insertEntry(t, s, "d1", "http://localhost:19999") // nothing running here

	checker := health.NewChecker(s, 100*time.Millisecond, 500*time.Millisecond, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		_ = checker.Run(ctx)
	}()
	time.Sleep(1 * time.Second)

	e, err := s.Get(context.Background(), "d1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusDown, e.Status)
}
