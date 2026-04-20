package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// prmDocument is the RFC 9728 OAuth Protected Resource Metadata document.
// Registered at /.well-known/oauth-protected-resource (conditionally — only when
// federation is enabled; see L-new-1 from spec-audit-rev2.md).
type prmDocument struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// NewPRMHandler returns an http.Handler that serves the RFC 9728 Protected
// Resource Metadata document. issuerURL is the Dex issuer (the authorization
// server); resourceURL is the canonical MCP public URL.
//
// The composition root (cmd/agentlens/main.go) MUST check cfg.Federation.Enabled
// before registering this handler — it must NOT be registered unconditionally.
func NewPRMHandler(resourceURL, issuerURL string) http.Handler {
	doc := prmDocument{
		Resource:               resourceURL,
		AuthorizationServers:   []string{issuerURL},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        []string{"openid", "profile", "email"},
	}
	docBytes, err := json.Marshal(doc)
	if err != nil {
		// Build-time failure — document structure is static.
		panic("handlers_prm: failed to marshal PRM document: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.DebugContext(r.Context(), "prm: serving oauth-protected-resource metadata")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(docBytes); err != nil {
			slog.WarnContext(r.Context(), "prm: failed to write response", "err", err)
		}
	})
}
