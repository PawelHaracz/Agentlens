// Package apikey validates service-account API keys of the form
// "agentlens_sk_<clientID>.<secret>".
package apikey

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/auth/ratelimit"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

const prefix = "agentlens_sk_"

// CredentialStore is the minimal interface required by Validator.
type CredentialStore interface {
	GetByClientID(ctx context.Context, clientID string) (*model.ApiClientCredential, error)
}

// Validator parses and validates service-account API keys.
type Validator struct {
	store   CredentialStore
	cache   *credcache.Cache
	limiter *ratelimit.ClientIDLimiter
}

// New creates a Validator. cache and limiter may not be nil.
func New(cs CredentialStore, cache *credcache.Cache, limiter *ratelimit.ClientIDLimiter) *Validator {
	return &Validator{store: cs, cache: cache, limiter: limiter}
}

// ErrRateLimited is returned when the client_id has exceeded the failure threshold.
var ErrRateLimited = fmt.Errorf("rate limited: too many failed attempts")

// ErrInvalidFormat is returned for tokens that don't match the expected prefix+structure.
var ErrInvalidFormat = fmt.Errorf("invalid API key format")

// ErrInvalidCredential is returned when the key doesn't match any active credential.
var ErrInvalidCredential = fmt.Errorf("invalid or revoked API key")

// Validate parses rawKey, checks the credential store (using credcache to
// short-circuit bcrypt on repeated calls within the TTL window), and returns
// the resolved principal on success.
//
// Sequence:
//  1. Parse "agentlens_sk_<clientID>.<secret>"
//  2. Check rate limit (429 if exceeded)
//  3. Check credcache — return cached ref if hit
//  4. Lookup credential row (404-like → invalid)
//  5. Check revoked_at IS NULL
//  6. bcrypt.CompareHashAndPassword
//  7. On success: cache result, reset rate limiter, return ref
//  8. On failure: record rate-limit failure, return error
func (v *Validator) Validate(ctx context.Context, rawKey string) (*model.SessionPrincipalRef, error) {
	clientID, secret, err := parse(rawKey)
	if err != nil {
		return nil, err
	}

	if v.limiter.IsLimited(clientID) {
		return nil, ErrRateLimited
	}

	if ref, hit := v.cache.Get(clientID, secret); hit {
		return ref, nil
	}

	cred, err := v.store.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("looking up credential: %w", err)
	}
	if cred == nil || cred.RevokedAt != nil {
		v.limiter.RecordFailure(clientID)
		return nil, ErrInvalidCredential
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.SecretHash), []byte(secret)); err != nil {
		limited := v.limiter.RecordFailure(clientID)
		if limited {
			return nil, ErrRateLimited
		}
		return nil, ErrInvalidCredential
	}

	ref := &model.SessionPrincipalRef{
		ID:         cred.PartyID,
		Kind:       model.PrincipalTypeServiceAccount,
		PartyID:    cred.PartyID,
		AuthMethod: "api_key",
	}
	v.cache.Put(clientID, secret, ref)
	v.limiter.Reset(clientID)
	return ref, nil
}

// parse splits "agentlens_sk_<clientID>.<secret>" into its parts.
func parse(raw string) (clientID, secret string, err error) {
	if !strings.HasPrefix(raw, prefix) {
		return "", "", ErrInvalidFormat
	}
	rest := strings.TrimPrefix(raw, prefix)
	idx := strings.Index(rest, ".")
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", ErrInvalidFormat
	}
	return rest[:idx], rest[idx+1:], nil
}

// ensure store package is imported (CredentialStore may be satisfied by it)
var _ CredentialStore = (*store.ApiClientCredentialStore)(nil)
