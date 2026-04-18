package mcpserver

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const lastSeenChanCap = 1024

// lastSeenUpdate is a single async last_seen_at update request.
type lastSeenUpdate struct {
	sessionID string
	at        time.Time
}

// asyncWorker batches last_seen_at writes to reduce DB write pressure.
// Drops updates on channel-full and records the drop count as a metric.
type asyncWorker struct {
	ch      chan lastSeenUpdate
	store   sessionStore
	drops   metric.Int64Counter
	flushCh chan struct{}
}

func newAsyncWorker(store sessionStore) *asyncWorker {
	meter := otel.Meter("agentlens.mcp")
	drops, _ := meter.Int64Counter("agentlens_mcp_last_seen_drops_total",
		metric.WithDescription("Updates dropped because the async channel was full"))
	return &asyncWorker{
		ch:      make(chan lastSeenUpdate, lastSeenChanCap),
		store:   store,
		drops:   drops,
		flushCh: make(chan struct{}, 1),
	}
}

// Enqueue queues a last_seen_at update. Drops silently if channel is full.
func (w *asyncWorker) Enqueue(sessionID string, at time.Time) {
	select {
	case w.ch <- lastSeenUpdate{sessionID: sessionID, at: at}:
	default:
		if w.drops != nil {
			w.drops.Add(context.Background(), 1)
		}
	}
}

// Run processes the channel until ctx is cancelled, flushing on a 30s ticker.
func (w *asyncWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.drain(ctx)
			return
		case u := <-w.ch:
			w.apply(ctx, u)
		case <-ticker.C:
			w.flush(ctx)
		case <-w.flushCh:
			w.flush(ctx)
		}
	}
}

// Flush signals an immediate flush (called from Stop).
func (w *asyncWorker) Flush() {
	select {
	case w.flushCh <- struct{}{}:
	default:
	}
}

func (w *asyncWorker) apply(ctx context.Context, u lastSeenUpdate) {
	if err := w.store.UpdateLastSeen(ctx, u.sessionID, u.at); err != nil {
		slog.WarnContext(ctx, "mcp: failed to update last_seen_at", "err", err)
	}
}

func (w *asyncWorker) flush(ctx context.Context) {
	for {
		select {
		case u := <-w.ch:
			w.apply(ctx, u)
		default:
			return
		}
	}
}

func (w *asyncWorker) drain(ctx context.Context) {
	for {
		select {
		case u := <-w.ch:
			w.apply(ctx, u)
		default:
			return
		}
	}
}
