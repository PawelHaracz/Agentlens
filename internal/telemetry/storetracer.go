package telemetry

import (
	"context"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// compile-time check: *TracedStore must satisfy store.Store.
var _ store.Store = (*TracedStore)(nil)

// TracedStoreOption configures TracedStore.
type TracedStoreOption func(*tracedStoreCfg)

type tracedStoreCfg struct {
	tp trace.TracerProvider
}

// WithTracerProvider overrides the tracer provider (useful in tests).
func WithTracerProvider(tp trace.TracerProvider) TracedStoreOption {
	return func(c *tracedStoreCfg) { c.tp = tp }
}

// TracedStore wraps a store.Store with OTel tracing spans for key operations.
// The store package never imports OTel — tracing is added here at the infrastructure boundary.
type TracedStore struct {
	inner   store.Store
	dialect string
	tracer  trace.Tracer
}

// NewTracedStore creates a tracing decorator around a catalog store.
func NewTracedStore(inner store.Store, dialect string, opts ...TracedStoreOption) *TracedStore {
	cfg := &tracedStoreCfg{tp: otel.GetTracerProvider()}
	for _, o := range opts {
		o(cfg)
	}
	return &TracedStore{
		inner:   inner,
		dialect: dialect,
		tracer:  cfg.tp.Tracer("agentlens.store"),
	}
}

func (t *TracedStore) startSpan(ctx context.Context, name, operation string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name,
		trace.WithAttributes(
			attribute.String("db.system", t.dialect),
			attribute.String("db.operation", operation),
		),
	)
}

func recordErr(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// Traced operations

// Create wraps store.Create with a tracing span.
func (t *TracedStore) Create(ctx context.Context, entry *model.CatalogEntry) error {
	ctx, span := t.startSpan(ctx, "store.catalog.create", "create")
	defer span.End()
	err := t.inner.Create(ctx, entry)
	recordErr(span, err)
	return err
}

// Get wraps store.Get with a tracing span.
func (t *TracedStore) Get(ctx context.Context, id string) (*model.CatalogEntry, error) {
	ctx, span := t.startSpan(ctx, "store.catalog.get", "get")
	defer span.End()
	result, err := t.inner.Get(ctx, id)
	recordErr(span, err)
	return result, err
}

// List wraps store.List with a tracing span and records result count.
func (t *TracedStore) List(ctx context.Context, filter store.ListFilter) ([]model.CatalogEntry, error) {
	ctx, span := t.startSpan(ctx, "store.catalog.list", "list")
	defer span.End()
	results, err := t.inner.List(ctx, filter)
	recordErr(span, err)
	span.SetAttributes(attribute.Int("agentlens.store.result_count", len(results)))
	return results, err
}

// UpdateHealth wraps store.UpdateHealth with a tracing span.
func (t *TracedStore) UpdateHealth(ctx context.Context, entryID string, h model.Health) error {
	ctx, span := t.startSpan(ctx, "store.catalog.update_health", "update_health")
	defer span.End()
	err := t.inner.UpdateHealth(ctx, entryID, h)
	recordErr(span, err)
	return err
}

// ListCapabilities wraps store.ListCapabilities with a tracing span.
func (t *TracedStore) ListCapabilities(ctx context.Context, filter store.CapabilityFilter) (*model.CapabilityListResult, error) {
	ctx, span := t.startSpan(ctx, "store.skills.list", "list_capabilities")
	defer span.End()
	result, err := t.inner.ListCapabilities(ctx, filter)
	recordErr(span, err)
	return result, err
}

// ListAgentsByCapability wraps store.ListAgentsByCapability with a tracing span.
func (t *TracedStore) ListAgentsByCapability(ctx context.Context, kind, name string) ([]model.CatalogEntry, error) {
	ctx, span := t.startSpan(ctx, "store.skills.list_agents", "list_agents_by_capability")
	defer span.End()
	results, err := t.inner.ListAgentsByCapability(ctx, kind, name)
	recordErr(span, err)
	span.SetAttributes(attribute.Int("agentlens.store.result_count", len(results)))
	return results, err
}

// Pass-through operations (no tracing — not key paths)

// UpsertProvider delegates to inner store without tracing.
func (t *TracedStore) UpsertProvider(ctx context.Context, provider *model.Provider) (*model.Provider, error) {
	return t.inner.UpsertProvider(ctx, provider)
}

// Update delegates to inner store without tracing.
func (t *TracedStore) Update(ctx context.Context, entry *model.CatalogEntry) error {
	return t.inner.Update(ctx, entry)
}

// Delete delegates to inner store without tracing.
func (t *TracedStore) Delete(ctx context.Context, id string) error {
	return t.inner.Delete(ctx, id)
}

// FindByEndpoint delegates to inner store without tracing.
func (t *TracedStore) FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error) {
	return t.inner.FindByEndpoint(ctx, endpoint)
}

// Stats delegates to inner store without tracing.
func (t *TracedStore) Stats(ctx context.Context) (*store.StoreStats, error) {
	return t.inner.Stats(ctx)
}

// ListForProbing delegates to inner store without tracing.
func (t *TracedStore) ListForProbing(ctx context.Context, olderThan time.Time, limit int) ([]model.CatalogEntry, error) {
	return t.inner.ListForProbing(ctx, olderThan, limit)
}

// SetLifecycle delegates to inner store without tracing.
func (t *TracedStore) SetLifecycle(ctx context.Context, entryID string, state model.LifecycleState) error {
	return t.inner.SetLifecycle(ctx, entryID, state)
}

// Close delegates to inner store without tracing.
func (t *TracedStore) Close() error {
	return t.inner.Close()
}
