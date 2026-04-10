package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func init() {
	RegisterCapability("a2a.skill", func() Capability { return &A2ASkill{} }, true)
	RegisterCapability("a2a.interface", func() Capability { return &A2AInterface{} }, false)
	RegisterCapability("a2a.security_scheme", func() Capability { return &A2ASecurityScheme{} }, false)
	RegisterCapability("a2a.security_requirement", func() Capability { return &A2ASecurityRequirement{} }, false)
	RegisterCapability("a2a.extension", func() Capability { return &A2AExtension{} }, false)
	RegisterCapability("a2a.signature", func() Capability { return &A2ASignature{} }, false)
}

// A2ASkill represents an A2A protocol skill capability.
// kind: "a2a.skill"
type A2ASkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// Kind returns the capability kind identifier.
func (s *A2ASkill) Kind() string { return "a2a.skill" }

// Validate checks that required fields are present.
func (s *A2ASkill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("a2a.skill: name is required")
	}
	return nil
}

// A2AExtension represents an A2A protocol extension capability.
// kind: "a2a.extension"
type A2AExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

func (a *A2AExtension) Kind() string { return "a2a.extension" }
func (a *A2AExtension) Validate() error {
	if a.URI == "" {
		return errors.New("a2a.extension: uri is required")
	}
	return nil
}

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
// Format: ["skill:<skillRef>:"]<sortedSchemeKeys joined with "+">
func (a *A2ASecurityRequirement) derivedName() string {
	keys := make([]string, 0, len(a.Schemes))
	for k := range a.Schemes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	name := strings.Join(keys, "+")
	if a.SkillRef != "" {
		name = "skill:" + a.SkillRef + ":" + name
	}
	return name
}

// A2AInterface represents an A2A agent interface binding capability.
// kind: "a2a.interface"
type A2AInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

func (a *A2AInterface) Kind() string { return "a2a.interface" }
func (a *A2AInterface) Validate() error {
	if a.URL == "" {
		return errors.New("a2a.interface: url is required")
	}
	return nil
}

// A2ASignature represents a cryptographic signature configuration capability.
// kind: "a2a.signature"
type A2ASignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

func (a *A2ASignature) Kind() string { return "a2a.signature" }
func (a *A2ASignature) Validate() error {
	if a.Algorithm == "" {
		return errors.New("a2a.signature: algorithm is required")
	}
	return nil
}
