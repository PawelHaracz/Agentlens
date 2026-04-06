// Package service provides shared services for the AgentLens API.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const (
	maxBodyBytes = 1 << 20 // 1 MB
	maxRedirects = 3
	fetchTimeout = 10 * time.Second
)

// FetchResult holds the raw JSON fetched from a remote URL and the auto-detected protocol.
type FetchResult struct {
	RawJSON          json.RawMessage
	DetectedProtocol string // "a2a", "mcp", or "" if undetermined
}

// Fetcher is the interface for fetching and validating agent cards from URLs.
type Fetcher interface {
	// ValidateURL checks that the URL is safe to fetch (non-empty, http/https, not private).
	ValidateURL(rawURL string) error
	// Fetch retrieves the card JSON from the URL.
	Fetch(ctx context.Context, rawURL string) (*FetchResult, error)
}

// CardFetcher fetches agent card JSON from a remote URL with SSRF protection.
type CardFetcher struct {
	client *http.Client
}

// NewCardFetcher creates a CardFetcher with safe defaults.
func NewCardFetcher() *CardFetcher {
	dialer := &net.Dialer{
		Timeout: fetchTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			// Defense-in-depth against DNS rebinding: reject any resolved address
			// that turns out to be private/loopback/link-local at connect time.
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("invalid dial address: %w", err)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("dial address is not an IP literal: %s", host)
			}
			if isPrivateIP(ip) {
				return fmt.Errorf("refusing to connect to private address %s", host)
			}
			return nil
		},
	}
	return &CardFetcher{
		client: &http.Client{
			Timeout:   fetchTimeout,
			Transport: &http.Transport{DialContext: dialer.DialContext},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("too many redirects")
				}
				// Re-validate the redirect target — a public host must not be
				// allowed to bounce us to an internal one.
				if err := ValidateURL(req.URL.String()); err != nil {
					return fmt.Errorf("unsafe redirect target: %w", err)
				}
				return nil
			},
		},
	}
}

// ValidateURL checks that the URL is safe to fetch (non-empty, http/https, not a private address).
func (f *CardFetcher) ValidateURL(rawURL string) error {
	return ValidateURL(rawURL)
}

// ValidateURL checks that the URL is safe to fetch (non-empty, http/https, not a private address).
// This is a package-level helper for use outside the Fetcher interface.
func ValidateURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("url must include a host")
	}
	return checkPrivateHost(u.Hostname())
}

// checkPrivateHost rejects hostnames that resolve to private/loopback/link-local addresses.
func checkPrivateHost(hostname string) error {
	if isPrivateHostname(hostname) {
		return fmt.Errorf("url points to a private or reserved address")
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// Cannot resolve — let the fetch fail naturally; DNS lookup is best-effort here.
		return nil
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("url resolves to a private or reserved address")
		}
	}
	return nil
}

// isPrivateHostname returns true for obviously private hostnames (without DNS lookup).
func isPrivateHostname(hostname string) bool {
	lower := strings.ToLower(hostname)
	// Reject plain "localhost" and variants.
	if lower == "localhost" || lower == "localhost." {
		return true
	}
	// Reject numeric literals that are private IPs.
	ip := net.ParseIP(hostname)
	if ip != nil {
		return isPrivateIP(ip)
	}
	return false
}

// isPrivateIP returns true for loopback, private, link-local, or site-local addresses.
func isPrivateIP(ip net.IP) bool {
	private := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // private
		"172.16.0.0/12",  // private
		"192.168.0.0/16", // private
		"169.254.0.0/16", // link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range private {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Fetch retrieves and validates the agent card from the given URL.
// It returns the raw JSON and an auto-detected protocol hint.
// ValidateURL is called as a defense-in-depth guard even when the caller
// has already validated the URL — this prevents SSRF if Fetch is called directly.
func (f *CardFetcher) Fetch(ctx context.Context, rawURL string) (*FetchResult, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, err
	}
	// Reparse and rebuild the URL from validated components so the request
	// target cannot smuggle through anything ValidateURL did not inspect.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	safeURL := (&url.URL{
		Scheme:   parsed.Scheme,
		Host:     parsed.Host,
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
	}).String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "AgentLens/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote server returned status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if !json.Valid(body) {
		return nil, fmt.Errorf("response is not valid JSON")
	}

	protocol := detectProtocol(safeURL, body)

	return &FetchResult{
		RawJSON:          json.RawMessage(body),
		DetectedProtocol: protocol,
	}, nil
}

// detectProtocol guesses the protocol from URL patterns and card content.
// Returns "a2a", "mcp", or "" if undetermined.
func detectProtocol(rawURL string, body []byte) string {
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, "/.well-known/agent") {
		return "a2a"
	}
	if strings.Contains(lower, "/mcp") {
		return "mcp"
	}

	// Inspect card content for distinguishing fields.
	var card map[string]json.RawMessage
	if err := json.Unmarshal(body, &card); err != nil {
		return ""
	}
	if _, ok := card["skills"]; ok {
		return "a2a"
	}
	if _, ok := card["tools"]; ok {
		return "mcp"
	}
	return ""
}
