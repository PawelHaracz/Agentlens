// Package health provides the health check plugin.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Plugin implements the health checker plugin.
type Plugin struct {
	store       store.Store
	interval    time.Duration
	timeout     time.Duration
	concurrency int
	log         *slog.Logger
}

// New creates a new health checker plugin.
func New(interval, timeout time.Duration, concurrency int) *Plugin {
	return &Plugin{
		interval:    interval,
		timeout:     timeout,
		concurrency: concurrency,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "health-checker" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.store = k.Store()
	p.log = k.Logger().With("component", "health-checker")
	return nil
}

// Start starts the health check loop.
func (p *Plugin) Start(ctx context.Context) error {
	go p.run(ctx)
	return nil
}

// Stop stops the plugin (no-op, context cancellation handles it).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

func (p *Plugin) run(ctx context.Context) {
	p.log.Info("starting health checker", "interval", p.interval, "concurrency", p.concurrency)
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
	entries, err := p.store.List(ctx, store.ListFilter{})
	if err != nil {
		p.log.Warn("failed to list entries for health check", "err", err)
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
			p.checkOne(ctx, &e)
		}()
	}
	wg.Wait()
}

func (p *Plugin) checkOne(ctx context.Context, entry *model.CatalogEntry) {
	if entry.Endpoint == "" {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	client := &http.Client{Timeout: p.timeout}
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, entry.Endpoint, nil)
	if err != nil {
		p.updateStatus(ctx, entry, model.StatusDown)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		p.updateStatus(ctx, entry, model.StatusDown)
		return
	}
	resp.Body.Close()

	var status model.Status
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		status = model.StatusHealthy
	case resp.StatusCode >= 500:
		status = model.StatusDegraded
	default:
		status = model.StatusUnknown
	}

	p.updateStatus(ctx, entry, status)
}

func (p *Plugin) updateStatus(ctx context.Context, entry *model.CatalogEntry, status model.Status) {
	now := time.Now().UTC()
	entry.Status = status
	entry.Validity.LastSeen = now
	entry.UpdatedAt = now
	if err := p.store.Update(ctx, entry); err != nil {
		p.log.Warn("failed to update entry status", "id", entry.ID, "err", err)
	}
}
