// Package health provides periodic health checking for registered catalog entries.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Checker periodically checks the health of all catalog entries and updates their status.
type Checker struct {
	store       store.Store
	interval    time.Duration
	timeout     time.Duration
	concurrency int
	log         *slog.Logger
}

// NewChecker creates a new Checker.
func NewChecker(s store.Store, interval, timeout time.Duration, concurrency int) *Checker {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Checker{
		store:       s,
		interval:    interval,
		timeout:     timeout,
		concurrency: concurrency,
		log:         slog.With("component", "health-checker"),
	}
}

// Run starts the health check loop and blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) error {
	c.log.Info("starting health checker", "interval", c.interval, "concurrency", c.concurrency)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	entries, err := c.store.List(ctx, store.ListFilter{})
	if err != nil {
		c.log.Warn("failed to list entries for health check", "err", err)
		return
	}

	sem := make(chan struct{}, c.concurrency)
	var wg sync.WaitGroup
	for _, e := range entries {
		e := e
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c.checkOne(ctx, &e)
		}()
	}
	wg.Wait()
}

func (c *Checker) checkOne(ctx context.Context, entry *model.CatalogEntry) {
	if entry.AgentType == nil {
		return
	}
	if entry.AgentType.Endpoint == "" {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, entry.AgentType.Endpoint, nil)
	if err != nil {
		c.updateStatus(ctx, entry, model.LifecycleOffline)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		c.updateStatus(ctx, entry, model.LifecycleOffline)
		return
	}
	_ = resp.Body.Close()

	var status model.LifecycleState
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		status = model.LifecycleActive
	case resp.StatusCode >= 500:
		status = model.LifecycleDegraded
	default:
		status = model.LifecycleRegistered
	}

	c.updateStatus(ctx, entry, status)
}

func (c *Checker) updateStatus(ctx context.Context, entry *model.CatalogEntry, status model.LifecycleState) {
	now := time.Now().UTC()
	entry.Status = status
	entry.Validity.LastSeen = now
	entry.UpdatedAt = now
	if err := c.store.Update(ctx, entry); err != nil {
		c.log.Warn("failed to update entry status", "id", entry.ID, "err", err)
	}
}
