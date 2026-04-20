package api

import (
	"context"
	"net/http"
	"net/http/httptest"
)

// LoopbackFunc dispatches an in-process HTTP call through the chi router.
// Defined here so the composition root can pass it to the MCP plugin.
// The plugin package defines its own parallel type alias to avoid importing
// internal/api (arch-go boundary — plugins cannot import internal/api).
type LoopbackFunc func(ctx context.Context, method, path, query string) (body []byte, status int, err error)

// BuildLoopbackFunc wraps handler (the chi root router) so that tools can
// call REST endpoints in-process without a real network round-trip.
//
// M-new-1 compliance: the outer request context (carrying SessionPrincipalRef
// and AccessibleProjectIDs) is passed via .WithContext(ctx) so the inner
// RequireAuth and ScopeByAccessibleProjects middleware see the same values.
// This means user-supplied ?projects= query params in MCP tool args have no
// effect — the context filter takes precedence at the handler level.
func BuildLoopbackFunc(handler http.Handler) LoopbackFunc {
	return func(ctx context.Context, method, path, query string) ([]byte, int, error) {
		target := path
		if query != "" {
			target = path + "?" + query
		}
		req, err := http.NewRequestWithContext(ctx, method, target, nil)
		if err != nil {
			return nil, 0, err
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Body.Bytes(), w.Code, nil
	}
}
