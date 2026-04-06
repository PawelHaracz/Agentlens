package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport lets tests control what the HTTP "server" returns without
// making real network connections.
type mockTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return m.fn(r)
}

func jsonResp(body string, status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// fetcherWith replaces the transport of a real NewCardFetcher so that all
// SSRF protections (ValidateURL, CheckRedirect, Dialer.Control) are intact
// while the "network" is controlled by fn.
func fetcherWith(fn func(*http.Request) (*http.Response, error)) *CardFetcher {
	f := NewCardFetcher()
	f.client.Transport = &mockTransport{fn: fn}
	return f
}

// ---------------------------------------------------------------------------
// ValidateURL
// ---------------------------------------------------------------------------

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty string", url: "", wantErr: true},
		{name: "whitespace only", url: "   ", wantErr: true},
		{name: "ftp scheme", url: "ftp://example.com/file", wantErr: true},
		{name: "file scheme", url: "file:///etc/passwd", wantErr: true},
		{name: "no host", url: "https:///path", wantErr: true},
		// private / loopback addresses
		{name: "loopback 127.0.0.1", url: "http://127.0.0.1/card", wantErr: true},
		{name: "loopback 127.0.0.2", url: "http://127.0.0.2/card", wantErr: true},
		{name: "localhost hostname", url: "http://localhost/card", wantErr: true},
		{name: "localhost with dot", url: "http://localhost./card", wantErr: true},
		{name: "private 10.x", url: "http://10.0.0.1/card", wantErr: true},
		{name: "private 192.168.x", url: "http://192.168.1.1/card", wantErr: true},
		{name: "private 172.16.x", url: "http://172.16.0.1/card", wantErr: true},
		{name: "link-local AWS metadata", url: "http://169.254.169.254/card", wantErr: true},
		{name: "IPv6 loopback", url: "http://[::1]/card", wantErr: true},
		// valid public URLs
		{name: "https public", url: "https://example.com/agent.json", wantErr: false},
		{name: "http public with path and query", url: "http://example.com/path?q=1", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateURL(tc.url)
			if tc.wantErr {
				assert.Error(t, err, "expected an error for %q", tc.url)
			} else {
				assert.NoError(t, err, "expected no error for %q", tc.url)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isPrivateIP
// ---------------------------------------------------------------------------

func TestIsPrivateIP_Ranges(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"10.0.0.0", true},
		{"10.255.255.255", true},
		{"172.16.0.0", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"169.254.0.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"fc00::1", true},
		{"fdff:ffff::1", true},
		{"fe80::1", true},
		// public
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
	}

	for _, tc := range tests {
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip, "bad test IP literal: %s", tc.ip)
			assert.Equal(t, tc.want, isPrivateIP(ip))
		})
	}
}

// ---------------------------------------------------------------------------
// detectProtocol
// ---------------------------------------------------------------------------

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		body     string
		expected string
	}{
		{
			name:     "well-known agent URL → a2a",
			rawURL:   "https://example.com/.well-known/agent.json",
			body:     `{}`,
			expected: "a2a",
		},
		{
			name:     "mcp path in URL → mcp",
			rawURL:   "https://example.com/mcp",
			body:     `{}`,
			expected: "mcp",
		},
		{
			name:     "body has skills field → a2a",
			rawURL:   "https://example.com/card",
			body:     `{"name":"agent","skills":[]}`,
			expected: "a2a",
		},
		{
			name:     "body has tools field → mcp",
			rawURL:   "https://example.com/card",
			body:     `{"name":"server","tools":[]}`,
			expected: "mcp",
		},
		{
			name:     "no hints → empty",
			rawURL:   "https://example.com/card",
			body:     `{"name":"unknown"}`,
			expected: "",
		},
		{
			name:     "invalid JSON body → empty",
			rawURL:   "https://example.com/card",
			body:     `not json`,
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectProtocol(tc.rawURL, []byte(tc.body))
			assert.Equal(t, tc.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Fetch — behaviour tests (transport mocked, SSRF guards intact)
// ---------------------------------------------------------------------------

const validAgentJSON = `{
  "name": "Test Agent",
  "version": "1.0",
  "skills": [{"id": "s1", "name": "skill"}]
}`

func TestFetch_HappyPath(t *testing.T) {
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		return jsonResp(validAgentJSON, http.StatusOK), nil
	})

	result, err := f.Fetch(context.Background(), "https://example.com/.well-known/agent.json")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "a2a", result.DetectedProtocol)
	assert.Contains(t, string(result.RawJSON), "Test Agent")
}

func TestFetch_Non2xx_ReturnsError(t *testing.T) {
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		return jsonResp(`{"error":"not found"}`, http.StatusNotFound), nil
	})

	_, err := f.Fetch(context.Background(), "https://example.com/agent.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetch_NonJSON_ReturnsError(t *testing.T) {
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("<html>not json</html>")),
		}, nil
	})

	_, err := f.Fetch(context.Background(), "https://example.com/agent.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid JSON")
}

// TestFetch_RedirectToPrivateIP verifies that CheckRedirect rejects a
// 302 response whose Location header points to a private address.
// This is the key regression test for the SSRF redirect-bypass fix.
func TestFetch_RedirectToPrivateIP_IsRejected(t *testing.T) {
	called := 0
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		called++
		return &http.Response{
			StatusCode: http.StatusMovedPermanently,
			Header:     http.Header{"Location": []string{"http://169.254.169.254/latest/meta-data/"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	_, err := f.Fetch(context.Background(), "https://example.com/agent.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe redirect target")
	// Transport must have been called exactly once — the redirect was NOT followed.
	assert.Equal(t, 1, called)
}

// TestFetch_TooManyRedirects verifies the redirect-count cap.
func TestFetch_TooManyRedirects_IsRejected(t *testing.T) {
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		// Always redirect to a valid public URL so each hop passes ValidateURL.
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://example.com/agent.json"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	_, err := f.Fetch(context.Background(), "https://example.com/agent.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many redirects")
}

// TestFetch_URLReconstruction_StripsFragment verifies that the Fragment is
// stripped from the outbound request URL (prevents anchor smuggling).
func TestFetch_URLReconstruction_StripsFragment(t *testing.T) {
	var capturedURL string
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		capturedURL = r.URL.String()
		return jsonResp(validAgentJSON, http.StatusOK), nil
	})

	_, err := f.Fetch(context.Background(), "https://example.com/agent.json?v=1#section")
	require.NoError(t, err)
	assert.NotContains(t, capturedURL, "#section", "fragment must be stripped")
	assert.Contains(t, capturedURL, "v=1", "query string must be preserved")
}

// TestFetch_PrivateURL_RejectedBeforeRequest verifies that the transport is
// never called when the initial URL is private.
func TestFetch_PrivateURL_RejectedBeforeRequest(t *testing.T) {
	requested := false
	f := fetcherWith(func(r *http.Request) (*http.Response, error) {
		requested = true
		return jsonResp("{}", http.StatusOK), nil
	})

	_, err := f.Fetch(context.Background(), "http://10.0.0.1/card")
	require.Error(t, err)
	assert.False(t, requested, "transport must not be called for private URLs")
}
