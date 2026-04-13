package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracedStore_ListCreatesSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	// Use in-memory SQLite store as the underlying store
	underlying, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = underlying.Close() })

	traced := telemetry.NewTracedStore(underlying, "sqlite", telemetry.WithTracerProvider(tp))

	ctx := context.Background()
	_, err = traced.List(ctx, store.ListFilter{})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "store.catalog.list", spans[0].Name)

	attrMap := make(map[string]string)
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsString()
	}
	assert.Equal(t, "sqlite", attrMap["db.system"])
	assert.Equal(t, "list", attrMap["db.operation"])
}

func TestTracedStore_GetCreatesSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	underlying, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = underlying.Close() })

	traced := telemetry.NewTracedStore(underlying, "sqlite", telemetry.WithTracerProvider(tp))

	ctx := context.Background()
	_, err = traced.Get(ctx, "nonexistent-id")
	// Get may return nil, nil for not found — that's fine
	_ = err

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "store.catalog.get", spans[0].Name)
}

func TestTracedStore_UpdateHealthCreatesSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	underlying, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = underlying.Close() })

	traced := telemetry.NewTracedStore(underlying, "sqlite", telemetry.WithTracerProvider(tp))

	ctx := context.Background()
	now := time.Now()
	_ = traced.UpdateHealth(ctx, "some-id", model.Health{
		State:        model.LifecycleActive,
		LastProbedAt: &now,
	})

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "store.catalog.update_health", spans[0].Name)
}
