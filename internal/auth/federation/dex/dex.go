// Package dex implements the federation.Provider interface using Dex as the
// OIDC provider via coreos/go-oidc/v3 and go-jose/v4.
package dex

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/PawelHaracz/agentlens/internal/auth/federation"
	"github.com/PawelHaracz/agentlens/internal/config"
)

const (
	// jwksRefreshRateLimit is the minimum interval between forced JWKS refreshes.
	// Per spec M-new-2: max 1 forced refresh per provider per 10s.
	jwksRefreshRateLimit = 10 * time.Second
)

// Provider is the Dex federation.Provider implementation.
type Provider struct {
	cfg      config.DexConfig
	audience string

	mu              sync.RWMutex
	oidcProvider    *gooidc.Provider
	lastRefreshAt   time.Time
	staleServeCount metric.Int64Counter
}

// New creates a Dex provider and performs OIDC discovery against the issuer URL.
func New(ctx context.Context, cfg config.DexConfig, audience string) (*Provider, error) {
	p := &Provider{cfg: cfg, audience: audience}

	meter := otel.Meter("agentlens.mcp")
	counter, err := meter.Int64Counter("agentlens_federation_jwks_stale_serves_total",
		metric.WithDescription("Number of times a stale JWKS was served due to refresh failure"))
	if err != nil {
		slog.Warn("failed to create stale-serves metric", "err", err)
	}
	p.staleServeCount = counter

	if err := p.discover(ctx); err != nil {
		return nil, fmt.Errorf("dex OIDC discovery: %w", err)
	}
	return p, nil
}

// VerifyIDToken validates rawToken against Dex's JWKS.
// On JWKS key-not-found, performs at most one forced refresh per
// jwksRefreshRateLimit window before returning an error.
func (p *Provider) VerifyIDToken(ctx context.Context, rawToken string) (*federation.Claims, error) {
	claims, err := p.verify(ctx, rawToken)
	if err == nil {
		return claims, nil
	}

	// Attempt a single forced JWKS refresh on key-miss (M-new-2).
	if !p.canRefresh() {
		p.recordStaleServe(ctx)
		return nil, fmt.Errorf("verifying federation token: %w", err)
	}
	if refreshErr := p.discover(ctx); refreshErr != nil {
		slog.WarnContext(ctx, "JWKS refresh failed, serving stale cache", "err", refreshErr)
		p.recordStaleServe(ctx)
		return nil, fmt.Errorf("verifying federation token (stale): %w", err)
	}

	return p.verify(ctx, rawToken)
}

// HealthPing checks that the Dex JWKS endpoint is reachable.
func (p *Provider) HealthPing(ctx context.Context) error {
	p.mu.RLock()
	prov := p.oidcProvider
	p.mu.RUnlock()

	if prov == nil {
		return fmt.Errorf("dex provider not initialised")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.JWKSURL, nil)
	if err != nil {
		return fmt.Errorf("building JWKS health request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("JWKS endpoint unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// discover performs OIDC provider discovery and updates the internal provider.
func (p *Provider) discover(ctx context.Context) error {
	prov, err := gooidc.NewProvider(ctx, p.cfg.Issuer)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.oidcProvider = prov
	p.lastRefreshAt = time.Now()
	p.mu.Unlock()
	return nil
}

// canRefresh returns true if the rate-limit window has elapsed.
func (p *Provider) canRefresh() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.lastRefreshAt) >= jwksRefreshRateLimit
}

// verify performs the actual JWT verification.
func (p *Provider) verify(ctx context.Context, rawToken string) (*federation.Claims, error) {
	p.mu.RLock()
	prov := p.oidcProvider
	p.mu.RUnlock()

	if prov == nil {
		return nil, fmt.Errorf("dex provider not initialised")
	}

	verifier := prov.Verifier(&gooidc.Config{ClientID: p.audience})
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("verifying id token: %w", err)
	}

	var raw map[string]interface{}
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("extracting claims: %w", err)
	}

	claims := &federation.Claims{
		Sub:       idToken.Subject,
		Audience:  idToken.Audience,
		Issuer:    idToken.Issuer,
		Expiry:    idToken.Expiry,
		RawClaims: raw,
	}
	if email, ok := raw["email"].(string); ok {
		claims.Email = email
	}
	if name, ok := raw["name"].(string); ok {
		claims.Name = name
	}
	return claims, nil
}

func (p *Provider) recordStaleServe(ctx context.Context) {
	if p.staleServeCount != nil {
		p.staleServeCount.Add(ctx, 1)
	}
}
