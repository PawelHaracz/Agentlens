package model

import (
	"errors"
	"sort"
	"strings"
)

// A2AOAuthFlow represents a single OAuth 2.0 flow variant.
type A2AOAuthFlow struct {
	// TODO: validate flow_type enum (authorizationCode|clientCredentials|deviceCode|implicit|password)
	FlowType         string            `json:"flow_type"` // "authorizationCode" | "clientCredentials" | "deviceCode" | "implicit" | "password"
	AuthorizationURL string            `json:"authorization_url,omitempty"`
	TokenURL         string            `json:"token_url,omitempty"`
	RefreshURL       string            `json:"refresh_url,omitempty"`
	DeviceAuthURL    string            `json:"device_auth_url,omitempty"` // deviceCode flow only
	Scopes           map[string]string `json:"scopes,omitempty"`          // scope -> description
	Deprecated       bool              `json:"deprecated,omitempty"`      // true for implicit/password
}

// A2ASecurityScheme represents a security scheme for A2A communication.
// kind: "a2a.security_scheme"
//
// This is a union type discriminated by Type. Only fields relevant to the
// scheme type are populated. Consumers switch on Type to determine which
// fields to read.
type A2ASecurityScheme struct {
	// Common
	SchemeName  string `json:"scheme_name"` // key from Agent Card's securitySchemes map
	Type        string `json:"type"`        // "apiKey" | "http" | "oauth2" | "openIdConnect" | "mutualTls"
	Description string `json:"description,omitempty"`

	// apiKey
	APIKeyLocation string `json:"api_key_location,omitempty"` // "header" | "query" | "cookie"
	APIKeyName     string `json:"api_key_name,omitempty"`     // e.g. "X-API-Key"

	// http
	HTTPScheme   string `json:"http_scheme,omitempty"`   // "Bearer" | "Basic" | "Digest"
	BearerFormat string `json:"bearer_format,omitempty"` // e.g. "JWT"

	// oauth2
	OAuthFlows        []A2AOAuthFlow `json:"oauth_flows,omitempty"`
	OAuth2MetadataURL string         `json:"oauth2_metadata_url,omitempty"`

	// openIdConnect
	OpenIDConnectURL string `json:"openid_connect_url,omitempty"`

	// Backward compat (v0.3 parser used these)
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
}

func (a *A2ASecurityScheme) Kind() string { return "a2a.security_scheme" }

func (a *A2ASecurityScheme) Validate() error {
	if a.SchemeName == "" {
		return errors.New("a2a.security_scheme: scheme_name is required")
	}
	if a.Type == "" {
		return errors.New("a2a.security_scheme: type is required")
	}
	return nil
}

// A2ASecurityRequirement declares which scheme(s) a client MUST use.
// kind: "a2a.security_requirement"
//
// Map key = scheme name (matching a key in SecuritySchemes).
// Map value = list of required scopes (empty = no specific scopes).
// Multiple A2ASecurityRequirement entries on an AgentType are OR'd:
// the client must satisfy at least one complete entry.
type A2ASecurityRequirement struct {
	Schemes  map[string][]string `json:"schemes"`
	SkillRef string              `json:"skill_ref,omitempty"` // non-empty = per-skill override
}

func (a *A2ASecurityRequirement) Kind() string { return "a2a.security_requirement" }

func (a *A2ASecurityRequirement) Validate() error {
	if len(a.Schemes) == 0 {
		return errors.New("a2a.security_requirement: schemes is required")
	}
	return nil
}

// derivedName returns a stable string that uniquely identifies this requirement
// within an agent type, matching the name derivation logic in capabilityToRow.
// Format: ["skill:<skillRef>:"]<schemeName>:<sortedScopes>[+...] sorted by scheme key
func (a *A2ASecurityRequirement) derivedName() string {
	keys := make([]string, 0, len(a.Schemes))
	for k := range a.Schemes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		scopes := make([]string, len(a.Schemes[k]))
		copy(scopes, a.Schemes[k])
		sort.Strings(scopes)
		parts = append(parts, k+":"+strings.Join(scopes, ","))
	}
	name := strings.Join(parts, "+")
	if a.SkillRef != "" {
		name = "skill:" + a.SkillRef + ":" + name
	}
	return name
}
