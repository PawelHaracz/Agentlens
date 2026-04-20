// Package federation defines the OIDC federation provider abstraction.
package federation

import (
	"context"
	"time"
)

// Claims are the validated identity claims from a federation JWT.
type Claims struct {
	Sub      string
	Email    string
	Name     string
	Audience []string
	Issuer   string
	Expiry   time.Time
	// RawClaims holds provider-specific additional claims (e.g. groups).
	RawClaims map[string]interface{}
}

// Provider is the interface implemented by OIDC federation providers (e.g. Dex).
type Provider interface {
	// VerifyIDToken validates a raw JWT and returns verified claims.
	// Returns an error if the token is invalid, expired, or audience-mismatched.
	VerifyIDToken(ctx context.Context, rawToken string) (*Claims, error)

	// HealthPing checks reachability of the provider's JWKS endpoint.
	// Used by the /readyz chain and the federation health background loop.
	HealthPing(ctx context.Context) error
}
