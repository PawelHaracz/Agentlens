// Package health provides the health check plugin.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// proberStore is the minimal store surface the prober needs.
// Keeping it minimal lets the health package avoid a dependency on the store package.
type proberStore interface {
	Get(ctx context.Context, id string) (*model.CatalogEntry, error)
	UpdateHealth(ctx context.Context, id string, h model.Health) error
	ListForProbing(ctx context.Context, olderThan time.Time, limit int) ([]model.CatalogEntry, error)
}

// Plugin implements the health checker plugin.
type Plugin struct {
	store            proberStore
	interval         time.Duration
	timeout          time.Duration
	concurrency      int
	degradedLatency  time.Duration
	failureThreshold int
	httpClient       *http.Client
	log              *slog.Logger
}

// New creates a Plugin from HealthCheckConfig.
func New(cfg config.HealthCheckConfig) *Plugin {
	concurrency := cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	return &Plugin{
		interval:         cfg.Interval,
		timeout:          cfg.Timeout,
		concurrency:      concurrency,
		degradedLatency:  cfg.DegradedLatency,
		failureThreshold: cfg.FailureThreshold,
		httpClient:       &http.Client{},
	}
}

// NewForTest creates a Plugin for unit tests (no kernel).
func NewForTest(degradedLatency time.Duration, failureThreshold int) *Plugin {
	return NewForTestWithTimeout(degradedLatency, failureThreshold, 5*time.Second)
}

// NewForTestWithTimeout creates a Plugin for unit tests with a custom probe timeout.
func NewForTestWithTimeout(degradedLatency time.Duration, failureThreshold int, timeout time.Duration) *Plugin {
	return &Plugin{
		interval:         30 * time.Second,
		timeout:          timeout,
		concurrency:      1,
		degradedLatency:  degradedLatency,
		failureThreshold: failureThreshold,
		httpClient:       &http.Client{Timeout: timeout},
		log:              slog.Default(),
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "health-checker" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "2.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// Init initializes the plugin with kernel dependencies.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.store = k.Store()
	p.log = k.Logger().With("component", "health-checker")
	p.httpClient = &http.Client{Timeout: p.timeout}
	return nil
}

// Start starts the health check loop.
func (p *Plugin) Start(ctx context.Context) error {
	go p.run(ctx)
	return nil
}

// Stop stops the plugin (context cancellation is sufficient).
func (p *Plugin) Stop(_ context.Context) error { return nil }

func (p *Plugin) run(ctx context.Context) {
	p.log.Info("starting health checker",
		"interval", p.interval,
		"concurrency", p.concurrency,
		"degradedLatency", p.degradedLatency,
		"failureThreshold", p.failureThreshold,
	)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkAll(ctx)
		}
	}
}

func (p *Plugin) checkAll(ctx context.Context) {
	olderThan := time.Now().UTC().Add(-p.interval)
	batchSize := p.concurrency * 4
	entries, err := p.store.ListForProbing(ctx, olderThan, batchSize)
	if err != nil {
		p.log.Warn("failed to list entries for probing", "err", err)
		return
	}

	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, e := range entries {
		e := e
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			h := p.probeOne(ctx, &e)
			if err := p.store.UpdateHealth(ctx, e.ID, h); err != nil {
				p.log.Warn("failed to persist probe result", "id", e.ID, "err", err)
			}
		}()
	}
	wg.Wait()
}

// probeOne executes a single HTTP probe and returns the resulting Health value.
// It does NOT write to the store. Deprecated entries are returned unchanged.
func (p *Plugin) probeOne(ctx context.Context, entry *model.CatalogEntry) model.Health {
	if entry.Status == model.LifecycleDeprecated {
		return entry.Health
	}

	url := resolveProbURL(entry)
	if url == "" {
		return p.noURLHealth(entry.Health)
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return p.failureHealth(entry.Health, truncateStr("invalid URL: "+err.Error(), 512))
	}

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return p.failureHealth(entry.Health, truncateStr(err.Error(), 512))
	}
	_ = resp.Body.Close()

	is2xx := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !is2xx {
		return p.failureHealth(entry.Health, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	return p.successHealth(latency)
}

func (p *Plugin) successHealth(latency time.Duration) model.Health {
	now := time.Now().UTC()
	state := model.LifecycleActive
	if latency > p.degradedLatency {
		state = model.LifecycleDegraded
	}
	return model.Health{
		State:               state,
		LastProbedAt:        &now,
		LastSuccessAt:       &now,
		LastError:           "",
		LatencyMs:           latency.Milliseconds(),
		ConsecutiveFailures: 0,
	}
}

func (p *Plugin) failureHealth(current model.Health, errMsg string) model.Health {
	now := time.Now().UTC()
	failures := current.ConsecutiveFailures + 1
	state := model.LifecycleDegraded
	if failures >= p.failureThreshold {
		state = model.LifecycleOffline
	}
	return model.Health{
		State:               state,
		LastProbedAt:        &now,
		LastSuccessAt:       current.LastSuccessAt,
		LastError:           errMsg,
		LatencyMs:           0,
		ConsecutiveFailures: failures,
	}
}

func (p *Plugin) noURLHealth(current model.Health) model.Health {
	now := time.Now().UTC()
	failures := current.ConsecutiveFailures + 1
	return model.Health{
		State:               model.LifecycleOffline,
		LastProbedAt:        &now,
		LastSuccessAt:       current.LastSuccessAt,
		LastError:           "no probeable endpoint",
		LatencyMs:           0,
		ConsecutiveFailures: failures,
	}
}

// resolveProbURL returns the URL to probe for a catalog entry.
// Falls back to Endpoint for all protocols.
// Note: A2A supportedInterfaces URL resolution via RawDefinition was removed
// when RawDefinition was dropped from AgentType (superseded by CardStorePlugin).
func resolveProbURL(entry *model.CatalogEntry) string {
	if entry.AgentType == nil {
		return ""
	}
	return entry.AgentType.Endpoint
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
