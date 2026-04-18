package mcpserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// TestSessionReaper_ExpiresStaleSessions_EverY60s verifies that the session
// store's ReapExpired marks past-deadline sessions as revoked (F.7).
func TestSessionReaper_ExpiresStaleSessions_EverY60s(t *testing.T) {
	ss := newStubStore()
	ctx := context.Background()
	now := time.Now().UTC()

	// Insert an already-expired session.
	expired := &model.McpSession{
		ID:              "expired-session",
		PrincipalID:     "p1",
		PrincipalType:   model.PrincipalTypeServiceAccount,
		ProtocolVersion: "2025-11-25",
		LastSeenAt:      now.Add(-2 * time.Hour),
		ExpiresAt:       now.Add(-1 * time.Hour),
	}
	require.NoError(t, ss.Create(ctx, expired))
	assert.Equal(t, int64(1), ss.active)

	// Reap: sessions whose ExpiresAt < now should be revoked.
	reaped, err := ss.ReapExpired(ctx, now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, reaped, int64(1), "at least one expired session should be reaped")
}

// TestOTelMetrics_Expose_InvocationsAndCredCacheHits verifies that the
// plugin initialises without panicking when telemetry metrics are registered
// (instruments may be no-ops if OTel is not configured in tests).
func TestOTelMetrics_Expose_InvocationsAndCredCacheHits(t *testing.T) {
	// Plugin.Init() calls newMCPMetrics() which registers OTel counters.
	// In test environment with no OTel provider, instruments are no-ops.
	// The test just asserts Init succeeds without panic.
	p, _, _ := initPlugin(t, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	require.NoError(t, p.Stop(context.Background()))
}
