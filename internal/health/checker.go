// Package health provides periodic health checking for registered agents.
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

// Checker periodically checks the health of all agents and updates their status.
type Checker struct {
	store       store.Store
	interval    time.Duration
	timeout     time.Duration
	concurrency int
	log         *slog.Logger
}

// NewChecker creates a new Checker.
func NewChecker(s store.Store, interval, timeout time.Duration, concurrency int) *Checker {
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
	agents, err := c.store.List(ctx, store.ListFilter{})
	if err != nil {
		c.log.Warn("failed to list agents for health check", "err", err)
		return
	}

	sem := make(chan struct{}, c.concurrency)
	var wg sync.WaitGroup
	for _, a := range agents {
		a := a
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c.checkOne(ctx, &a)
		}()
	}
	wg.Wait()
}

func (c *Checker) checkOne(ctx context.Context, agent *model.Agent) {
	if agent.Endpoint == "" {
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, agent.Endpoint, nil)
	if err != nil {
		c.updateStatus(ctx, agent, model.StatusDown)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		c.updateStatus(ctx, agent, model.StatusDown)
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

	c.updateStatus(ctx, agent, status)
}

func (c *Checker) updateStatus(ctx context.Context, agent *model.Agent, status model.Status) {
	now := time.Now().UTC()
	agent.Status = status
	agent.LastSeen = now
	agent.UpdatedAt = now
	if err := c.store.Update(ctx, agent); err != nil {
		c.log.Warn("failed to update agent status", "id", agent.ID, "err", err)
	}
}
