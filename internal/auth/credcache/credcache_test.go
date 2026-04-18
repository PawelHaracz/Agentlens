package credcache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/model"
)

func ref(id string) *model.SessionPrincipalRef {
	return &model.SessionPrincipalRef{ID: id, Kind: model.PrincipalTypeServiceAccount, AuthMethod: "api_key"}
}

func TestCredCache_Invalidate_EvictsEntry(t *testing.T) {
	c := credcache.NewWithOptions(16, 10*time.Second)

	c.Put("client-a", "secret1", ref("party-1"))
	r, hit := c.Get("client-a", "secret1")
	require.True(t, hit)
	assert.Equal(t, "party-1", r.ID)

	// Invalidate — write-lock removes the entry.
	c.Invalidate("client-a")
	_, hit = c.Get("client-a", "secret1")
	assert.False(t, hit, "entry should be evicted after Invalidate")

	// Document M-new-3: Invalidate does NOT cancel in-flight requests that
	// have already completed Get. The staleness window is ≤ max(TTL, in-flight).
}

func TestCredCache_TTL_Expiry(t *testing.T) {
	c := credcache.NewWithOptions(16, 50*time.Millisecond)
	c.Put("client-b", "secret", ref("party-2"))

	_, hit := c.Get("client-b", "secret")
	require.True(t, hit)

	time.Sleep(80 * time.Millisecond)
	_, hit = c.Get("client-b", "secret")
	assert.False(t, hit, "entry should be expired after TTL")
}

func TestCredCache_LRU_Eviction(t *testing.T) {
	c := credcache.NewWithOptions(3, time.Minute)
	c.Put("c1", "s", ref("p1"))
	c.Put("c2", "s", ref("p2"))
	c.Put("c3", "s", ref("p3"))
	assert.Equal(t, 3, c.Len())

	// Access c1 to make c2 the LRU.
	_, _ = c.Get("c1", "s")

	// Inserting c4 should evict c2 (LRU).
	c.Put("c4", "s", ref("p4"))
	assert.Equal(t, 3, c.Len())

	_, hit := c.Get("c2", "s")
	assert.False(t, hit, "c2 should have been evicted as LRU")
	_, hit = c.Get("c1", "s")
	assert.True(t, hit, "c1 was recently accessed, should remain")
}
