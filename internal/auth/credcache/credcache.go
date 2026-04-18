// Package credcache provides a short-lived in-process cache for successful
// service-account API-key bcrypt verification outcomes. See ADR-015.
package credcache

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

const (
	DefaultTTL     = 10 * time.Second
	DefaultMaxSize = 1024
	// secretFingerprintLen is the number of bytes of the raw secret used in the
	// fingerprint. Not sufficient to reconstruct the secret.
	secretFingerprintLen = 16
)

// entry is one LRU cache element.
type entry struct {
	clientID  string // stored for O(n) Invalidate scan
	fp        string // fingerprint(clientID, secret)
	ref       *model.SessionPrincipalRef
	expiresAt time.Time
}

// Cache is a concurrency-safe, TTL-bounded LRU cache for API-key bcrypt results.
//
// Concurrency model (M-new-3 from spec-audit-rev2):
//   - Get acquires read lock for lookup, write lock only for LRU promotion.
//   - Invalidate acquires write lock but does NOT cancel in-flight requests that
//     have already completed Get and are executing downstream. Documented staleness
//     window is ≤ max(TTL, longest in-flight request duration).
type Cache struct {
	mu      sync.RWMutex
	items   map[string]*list.Element // fingerprint → lru element
	lru     *list.List
	maxSize int
	ttl     time.Duration
}

// New returns a Cache with defaults (1024 entries, 10s TTL).
func New() *Cache { return NewWithOptions(DefaultMaxSize, DefaultTTL) }

// NewWithOptions creates a Cache with custom capacity and TTL.
func NewWithOptions(maxSize int, ttl time.Duration) *Cache {
	return &Cache{
		items:   make(map[string]*list.Element, maxSize),
		lru:     list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get looks up a cached bcrypt result for (clientID, secret).
// Returns (ref, true) on a valid (non-expired) hit.
// Returns (nil, false) on miss or expired entry.
func (c *Cache) Get(clientID, secret string) (*model.SessionPrincipalRef, bool) {
	fp := fingerprint(clientID, secret)

	c.mu.RLock()
	el, ok := c.items[fp]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	e := el.Value.(*entry)
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		c.evict(el)
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.lru.MoveToFront(el)
	c.mu.Unlock()

	return e.ref, true
}

// Put stores a verified principal reference for (clientID, secret).
func (c *Cache) Put(clientID, secret string, ref *model.SessionPrincipalRef) {
	fp := fingerprint(clientID, secret)

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[fp]; ok {
		e := el.Value.(*entry)
		e.ref = ref
		e.expiresAt = time.Now().Add(c.ttl)
		c.lru.MoveToFront(el)
		return
	}

	for c.lru.Len() >= c.maxSize {
		c.evict(c.lru.Back())
	}

	e := &entry{clientID: clientID, fp: fp, ref: ref, expiresAt: time.Now().Add(c.ttl)}
	c.items[fp] = c.lru.PushFront(e)
}

// Invalidate removes all cached entries for clientID.
// Holds write lock but does NOT cancel in-flight requests past Get (M-new-3).
// O(n) scan over ≤1024 entries — acceptable for this cache size.
func (c *Cache) Invalidate(clientID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toRemove []*list.Element
	for el := c.lru.Front(); el != nil; el = el.Next() {
		if el.Value.(*entry).clientID == clientID {
			toRemove = append(toRemove, el)
		}
	}
	for _, el := range toRemove {
		c.evict(el)
	}
}

// Len returns the number of currently cached entries (for tests/metrics).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// evict removes el from the LRU and map. Must hold write lock.
func (c *Cache) evict(el *list.Element) {
	if el == nil {
		return
	}
	c.lru.Remove(el)
	delete(c.items, el.Value.(*entry).fp)
}

// fingerprint derives a non-reversible cache key from clientID and the
// first secretFingerprintLen bytes of secret.
func fingerprint(clientID, secret string) string {
	trunc := secret
	if len(trunc) > secretFingerprintLen {
		trunc = trunc[:secretFingerprintLen]
	}
	sum := sha256.Sum256([]byte(clientID + ":" + trunc))
	return fmt.Sprintf("%x", sum)
}
