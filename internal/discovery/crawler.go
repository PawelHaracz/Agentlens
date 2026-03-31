// Package discovery provides agent discovery implementations.
package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Crawler fetches raw agent card data from HTTP endpoints.
type Crawler struct {
	client *http.Client
}

// NewCrawler creates a new Crawler with a 10-second timeout.
func NewCrawler() *Crawler {
	return &Crawler{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchCard performs an HTTP GET to url and returns the body bytes.
func (c *Crawler) FetchCard(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching card from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return body, nil
}
