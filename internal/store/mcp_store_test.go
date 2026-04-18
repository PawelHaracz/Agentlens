package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// openMCPStores opens an in-memory SQLite DB, runs all migrations, and returns
// the three MCP-related stores plus a party store for seeding.
func openMCPStores(t *testing.T) (*store.ApiClientCredentialStore, *store.MCPSessionStore, *store.UserExternalIdentityStore, *store.PartyStore) {
	t.Helper()
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)
	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))
	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		_ = sqlDB.Close()
	})
	return store.NewApiClientCredentialStore(database),
		store.NewMCPSessionStore(database),
		store.NewUserExternalIdentityStore(database),
		store.NewPartyStore(database)
}

func seedServiceAccountParty(t *testing.T, ps *store.PartyStore) *model.Party {
	t.Helper()
	party, err := ps.CreateServiceAccount(context.Background(), "test-sa-"+uuid.New().String()[:8])
	require.NoError(t, err)
	return party
}

// TestSessionPrincipalRef_TypeEnum verifies the PrincipalType enum values are
// well-defined in the foundation layer (no internal/auth import needed).
func TestSessionPrincipalRef_TypeEnum(t *testing.T) {
	tests := []struct {
		pt   model.PrincipalType
		want string
	}{
		{model.PrincipalTypeUserLocal, "user_local"},
		{model.PrincipalTypeUserFederated, "user_federated"},
		{model.PrincipalTypeServiceAccount, "service_account"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, string(tc.pt))
	}
	ref := model.SessionPrincipalRef{
		ID:                   "id-1",
		Kind:                 model.PrincipalTypeServiceAccount,
		PartyID:              "party-1",
		Permissions:          []string{"catalog:read"},
		AccessibleProjectIDs: []string{"proj-1"},
		AuthMethod:           "api_key",
	}
	assert.Equal(t, "id-1", ref.ID)
	assert.Equal(t, model.PrincipalTypeServiceAccount, ref.Kind)
}

// TestPartyStore_CreateServiceAccount_KindEnum verifies the kind value is
// persisted correctly and passes ValidPartyKinds check.
func TestPartyStore_CreateServiceAccount_KindEnum(t *testing.T) {
	_, _, _, ps := openMCPStores(t)
	ctx := context.Background()

	party, err := ps.CreateServiceAccount(ctx, "my-service-account")
	require.NoError(t, err)
	assert.Equal(t, model.PartyKindServiceAccount, party.Kind)
	assert.Equal(t, "my-service-account", party.Name)
	assert.NotEmpty(t, party.ID)

	// Verify ValidPartyKinds recognises service_account.
	assert.True(t, model.ValidPartyKinds[model.PartyKindServiceAccount])
}

// TestApiClientCredentialStore_PartialUniqueIndex_OneActivePerParty verifies
// the partial unique index prevents two active credentials for the same party.
func TestApiClientCredentialStore_PartialUniqueIndex_OneActivePerParty(t *testing.T) {
	cs, _, _, ps := openMCPStores(t)
	ctx := context.Background()
	party := seedServiceAccountParty(t, ps)

	cred1 := &model.ApiClientCredential{
		PartyID:    party.ID,
		ClientID:   "client-a",
		SecretHash: "hash-a",
	}
	require.NoError(t, cs.Create(ctx, cred1))

	// Inserting a second active credential for the same party must fail.
	cred2 := &model.ApiClientCredential{
		PartyID:    party.ID,
		ClientID:   "client-b",
		SecretHash: "hash-b",
	}
	err := cs.Create(ctx, cred2)
	require.Error(t, err, "second active credential should be rejected by partial unique index")
}

// TestApiClientCredentialStore_RotateSecret_AtomicUpdateThenInsert verifies
// that rotation revokes the old credential and creates a new one atomically.
func TestApiClientCredentialStore_RotateSecret_AtomicUpdateThenInsert(t *testing.T) {
	cs, _, _, ps := openMCPStores(t)
	ctx := context.Background()
	party := seedServiceAccountParty(t, ps)

	original := &model.ApiClientCredential{
		PartyID:    party.ID,
		ClientID:   "client-orig",
		SecretHash: "hash-orig",
	}
	require.NoError(t, cs.Create(ctx, original))

	rotated := &model.ApiClientCredential{
		PartyID:    party.ID,
		ClientID:   "client-new",
		SecretHash: "hash-new",
	}
	require.NoError(t, cs.RotateSecret(ctx, party.ID, rotated))

	// Old credential should now be revoked.
	old, err := cs.GetByClientID(ctx, "client-orig")
	require.NoError(t, err)
	require.NotNil(t, old)
	assert.NotNil(t, old.RevokedAt, "original credential should be revoked after rotation")

	// New credential should be active.
	active, err := cs.GetActiveForParty(ctx, party.ID)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, "client-new", active.ClientID)
}

// TestMcpSessionStore_SoftDelete_And_Reap verifies creation, soft-delete, and
// the TTL reaper.
func TestMcpSessionStore_SoftDelete_And_Reap(t *testing.T) {
	_, ss, _, ps := openMCPStores(t)
	ctx := context.Background()
	party := seedServiceAccountParty(t, ps)

	now := time.Now().UTC()
	sess := &model.McpSession{
		PrincipalID:     party.ID,
		PrincipalType:   model.PrincipalTypeServiceAccount,
		ProtocolVersion: "2025-11-25",
		LastSeenAt:      now,
		ExpiresAt:       now.Add(30 * time.Minute),
	}
	require.NoError(t, ss.Create(ctx, sess))
	assert.True(t, sess.IsActive(now))
	assert.False(t, sess.IsInitialized())

	// Mark initialized.
	require.NoError(t, ss.UpdateInitialized(ctx, sess.ID, now))
	fetched, err := ss.GetByID(ctx, sess.ID)
	require.NoError(t, err)
	assert.True(t, fetched.IsInitialized())

	// Soft-delete.
	require.NoError(t, ss.Revoke(ctx, sess.ID))
	revoked, err := ss.GetByID(ctx, sess.ID)
	require.NoError(t, err)
	assert.NotNil(t, revoked.RevokedAt)
	assert.False(t, revoked.IsActive(now))

	// Reap an already-expired session.
	expiredSess := &model.McpSession{
		PrincipalID:     party.ID,
		PrincipalType:   model.PrincipalTypeServiceAccount,
		ProtocolVersion: "2025-11-25",
		LastSeenAt:      now.Add(-2 * time.Hour),
		ExpiresAt:       now.Add(-1 * time.Hour),
	}
	require.NoError(t, ss.Create(ctx, expiredSess))

	reaped, err := ss.ReapExpired(ctx, now)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, reaped, int64(1))
}

// TestUserExternalIdentityStore_PendingApprovalFlow verifies the full
// pending → approved identity lifecycle.
func TestUserExternalIdentityStore_PendingApprovalFlow(t *testing.T) {
	_, _, is, _ := openMCPStores(t)
	ctx := context.Background()

	identity := &model.UserExternalIdentity{
		ProviderName: "dex",
		Sub:          "sub-12345",
		Email:        "test@example.com",
		DisplayName:  "Test User",
	}
	require.NoError(t, is.UpsertPending(ctx, identity))
	assert.NotEmpty(t, identity.ID)
	assert.Equal(t, model.ExternalIdentityStatusPending, identity.Status)

	// Upsert again (second login) — should update last_seen_at, not create duplicate.
	identity2 := &model.UserExternalIdentity{
		ProviderName: "dex",
		Sub:          "sub-12345",
		Email:        "test@example.com",
	}
	require.NoError(t, is.UpsertPending(ctx, identity2))

	pending, err := is.ListPending(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 1, "second upsert should not create a second pending row")

	// Approve.
	require.NoError(t, is.Approve(ctx, identity.ID, nil))

	fetched, err := is.GetByProviderSub(ctx, "dex", "sub-12345")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, model.ExternalIdentityStatusApproved, fetched.Status)
	assert.NotNil(t, fetched.ApprovedAt)
}
