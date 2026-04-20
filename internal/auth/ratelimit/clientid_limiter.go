// Package ratelimit provides per-client_id failure rate limiting for API-key auth.
package ratelimit

import (
	"sync"
	"time"
)

const (
	DefaultMaxFailures = 30
	DefaultWindow      = 60 * time.Second
)

type windowState struct {
	failures    int
	windowStart time.Time
}

// ClientIDLimiter tracks authentication failures per client_id and returns
// true (rate-limited → HTTP 429) after MaxFailures within Window.
// This is an online rate-limit for noisy-client detection, NOT an account
// lockout — service-account API keys must not be permanently locked.
type ClientIDLimiter struct {
	mu          sync.Mutex
	state       map[string]*windowState
	maxFailures int
	window      time.Duration
}

// New returns a limiter with the default thresholds (30 fails / 60s).
func New() *ClientIDLimiter {
	return NewWithOptions(DefaultMaxFailures, DefaultWindow)
}

// NewWithOptions creates a limiter with custom thresholds.
func NewWithOptions(maxFailures int, window time.Duration) *ClientIDLimiter {
	return &ClientIDLimiter{
		state:       make(map[string]*windowState),
		maxFailures: maxFailures,
		window:      window,
	}
}

// RecordFailure increments the failure counter for clientID.
// Returns true if the client has exceeded the threshold and should receive 429.
func (l *ClientIDLimiter) RecordFailure(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	s, ok := l.state[clientID]
	if !ok || now.Sub(s.windowStart) >= l.window {
		l.state[clientID] = &windowState{failures: 1, windowStart: now}
		return false
	}
	s.failures++
	return s.failures > l.maxFailures
}

// IsLimited reports whether clientID is currently rate-limited without
// incrementing the counter.
func (l *ClientIDLimiter) IsLimited(clientID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	s, ok := l.state[clientID]
	if !ok {
		return false
	}
	if now.Sub(s.windowStart) >= l.window {
		delete(l.state, clientID)
		return false
	}
	return s.failures > l.maxFailures
}

// Reset clears the failure counter for clientID. Call on successful auth.
func (l *ClientIDLimiter) Reset(clientID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.state, clientID)
}
