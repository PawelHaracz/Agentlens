// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Protocol represents the agent communication protocol.
type Protocol string

const (
	ProtocolA2A  Protocol = "a2a"
	ProtocolMCP  Protocol = "mcp"
	ProtocolA2UI Protocol = "a2ui"
)

// Status represents the health status of a catalog entry.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusUnknown  Status = "unknown"
)

// LifecycleState is the source of truth for the runtime state of a catalog entry.
// It replaces the old Status type for new code. The status DB column stores these values.
type LifecycleState string

const (
	LifecycleRegistered LifecycleState = "registered"
	LifecycleActive     LifecycleState = "active"
	LifecycleDegraded   LifecycleState = "degraded"
	LifecycleOffline    LifecycleState = "offline"
	LifecycleDeprecated LifecycleState = "deprecated"
)

// Health holds the runtime health state populated by the health prober.
// It is built from DB columns in SyncFromDB and is not stored directly.
type Health struct {
	State               LifecycleState
	LastProbedAt        *time.Time
	LastSuccessAt       *time.Time
	LastError           string
	LatencyMs           int64
	ConsecutiveFailures int
}

// SourceType represents how the catalog entry was discovered.
type SourceType string

const (
	SourceK8s      SourceType = "k8s"
	SourceConfig   SourceType = "config"
	SourcePush     SourceType = "push"
	SourceUpstream SourceType = "upstream"
)

// Validity represents a time-bounded availability period.
type Validity struct {
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
	LastSeen time.Time  `json:"last_seen"`
}

// IsActiveAt checks if the entry is active at the given time.
func (v Validity) IsActiveAt(t time.Time) bool {
	if v.From != nil && t.Before(*v.From) {
		return false
	}
	if v.To != nil && t.After(*v.To) {
		return false
	}
	return true
}

// CatalogEntry is the catalog wrapper (Product archetype) around an AgentType.
// It represents a discoverable listing of an agent in the catalog.
type CatalogEntry struct {
	ID          string     `json:"id"           gorm:"primaryKey;type:text"`
	AgentTypeID string     `json:"agent_type_id" gorm:"not null;type:text;index"`
	AgentType   *AgentType `json:"-"            gorm:"foreignKey:AgentTypeID"`
	DisplayName string     `json:"display_name"  gorm:"not null;type:text"`
	Description string     `json:"description"   gorm:"type:text;default:''"`
	// Status stores the LifecycleState value. Updated by the health prober and lifecycle API.
	Status     LifecycleState    `json:"-" gorm:"not null;type:text;default:'registered';index"`
	Source     SourceType        `json:"source"        gorm:"not null;type:text;index"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Categories []string          `json:"-" gorm:"-"`
	Metadata   map[string]string `json:"-" gorm:"-"`
	Validity   Validity          `json:"-" gorm:"-"`

	// Database-serialized JSON fields (used by GORM, hidden from JSON API).
	CategoriesJSON string     `json:"-" gorm:"column:categories;type:text;not null;default:'[]'"`
	MetadataJSON   string     `json:"-" gorm:"column:metadata;type:text;not null;default:'{}'"`
	ValidFrom      *time.Time `json:"-" gorm:"column:validity_from"`
	ValidTo        *time.Time `json:"-" gorm:"column:validity_to"`
	LastSeen       time.Time  `json:"-" gorm:"column:validity_last_seen;not null"`

	// Health check backing columns — managed by the health prober, hidden from direct JSON.
	HealthLastProbedAt        *time.Time `json:"-" gorm:"column:health_last_probed_at"`
	HealthLastSuccessAt       *time.Time `json:"-" gorm:"column:health_last_success_at"`
	HealthLastError           string     `json:"-" gorm:"column:health_last_error;type:text;not null;default:''"`
	HealthLatencyMs           int64      `json:"-" gorm:"column:health_latency_ms;not null;default:0"`
	HealthConsecutiveFailures int        `json:"-" gorm:"column:health_consecutive_failures;not null;default:0"`

	// Health is built by SyncFromDB. Not persisted directly.
	Health Health `json:"-" gorm:"-"`
}

// TableName overrides the GORM table name.
func (CatalogEntry) TableName() string { return "catalog_entries" }

// SyncToDB serializes public fields into GORM database columns.
func (e *CatalogEntry) SyncToDB() {
	if b, err := json.Marshal(e.Categories); err == nil {
		e.CategoriesJSON = string(b)
	}
	if b, err := json.Marshal(e.Metadata); err == nil {
		e.MetadataJSON = string(b)
	}
	e.ValidFrom = e.Validity.From
	e.ValidTo = e.Validity.To
	e.LastSeen = e.Validity.LastSeen
}

// SyncFromDB deserializes GORM database columns into public fields.
func (e *CatalogEntry) SyncFromDB() {
	if e.CategoriesJSON != "" {
		_ = json.Unmarshal([]byte(e.CategoriesJSON), &e.Categories)
	}
	if e.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(e.MetadataJSON), &e.Metadata)
	}
	e.Validity = Validity{
		From:     e.ValidFrom,
		To:       e.ValidTo,
		LastSeen: e.LastSeen,
	}
	e.Health = Health{
		State:               e.Status,
		LastProbedAt:        e.HealthLastProbedAt,
		LastSuccessAt:       e.HealthLastSuccessAt,
		LastError:           e.HealthLastError,
		LatencyMs:           e.HealthLatencyMs,
		ConsecutiveFailures: e.HealthConsecutiveFailures,
	}
}

// catalogEntryJSON is the flat, backward-compatible JSON shape for CatalogEntry.
type catalogEntryJSON struct {
	ID             string              `json:"id"`
	AgentTypeID    string              `json:"agent_type_id"`
	DisplayName    string              `json:"display_name"`
	Description    string              `json:"description"`
	Protocol       Protocol            `json:"protocol,omitempty"`
	Endpoint       string              `json:"endpoint,omitempty"`
	Version        string              `json:"version,omitempty"`
	SpecVersion    string              `json:"spec_version,omitempty"`
	Status         LifecycleState      `json:"status"`
	Source         SourceType          `json:"source"`
	Provider       *Provider           `json:"provider,omitempty"`
	Categories     []string            `json:"categories,omitempty"`
	Capabilities   json.RawMessage     `json:"capabilities,omitempty"`
	Validity       Validity            `json:"validity"`
	Metadata       map[string]string   `json:"metadata,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	Health         healthJSON          `json:"health"`
	AuthSummary    *authSummaryJSON    `json:"auth_summary,omitempty"`
	SecurityDetail *securityDetailJSON `json:"security_detail,omitempty"`
}

// healthJSON is the JSON shape for the embedded Health object.
type healthJSON struct {
	State               string     `json:"state"`
	LastProbedAt        *time.Time `json:"lastProbedAt"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	LatencyMs           int64      `json:"latencyMs"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	LastError           string     `json:"lastError"`
}

// authSummaryJSON is a computed view of security schemes for catalog list responses.
type authSummaryJSON struct {
	Types    []string `json:"types"`
	Label    string   `json:"label"`
	Required bool     `json:"required"`
}

// buildAuthSummary computes an auth summary from capabilities at serialization
// time. Returns nil when no security schemes are declared (omitempty hides it).
func buildAuthSummary(capabilities []Capability) *authSummaryJSON {
	seen := map[string]bool{}
	var schemes []string
	hasRequirement := false

	for _, cap := range capabilities {
		switch c := cap.(type) {
		case *A2ASecurityScheme:
			typeStr := c.Type
			if c.Type == "http" && c.HTTPScheme != "" {
				typeStr = "http:" + c.HTTPScheme
			}
			if !seen[typeStr] {
				seen[typeStr] = true
				schemes = append(schemes, typeStr)
			}
		case *A2ASecurityRequirement:
			if c.SkillRef == "" {
				hasRequirement = true
			}
		}
	}

	if len(schemes) == 0 {
		return nil
	}

	sort.Strings(schemes)

	return &authSummaryJSON{
		Types:    schemes,
		Label:    buildAuthLabel(schemes),
		Required: hasRequirement,
	}
}

func buildAuthLabel(schemes []string) string {
	if len(schemes) == 0 {
		return "Open (no auth)"
	}

	names := make([]string, 0, len(schemes))
	for _, s := range schemes {
		switch s {
		case "http:Bearer":
			names = append(names, "Bearer JWT")
		case "http:Basic":
			names = append(names, "Basic Auth")
		case "apiKey":
			names = append(names, "API Key")
		case "oauth2":
			names = append(names, "OAuth 2.0")
		case "openIdConnect":
			names = append(names, "OIDC")
		case "mutualTls":
			names = append(names, "mTLS")
		default:
			names = append(names, s)
		}
	}

	label := strings.Join(names, " + ")
	if len(label) > 40 {
		label = label[:37] + "..."
	}

	return label
}

// securityDetailJSON is a convenience view for the detail endpoint that groups
// security capabilities into a structured object. Computed at serialization time.
type securityDetailJSON struct {
	SecuritySchemes      []json.RawMessage `json:"security_schemes,omitempty"`
	SecurityRequirements []json.RawMessage `json:"security_requirements,omitempty"`
}

// buildSecurityDetail constructs a security_detail view from capabilities.
// Returns nil when no security capabilities exist.
func buildSecurityDetail(capabilities []Capability) *securityDetailJSON {
	var schemeCaps []*A2ASecurityScheme
	var reqCaps []*A2ASecurityRequirement

	for _, cap := range capabilities {
		switch c := cap.(type) {
		case *A2ASecurityScheme:
			schemeCaps = append(schemeCaps, c)
		case *A2ASecurityRequirement:
			reqCaps = append(reqCaps, c)
		}
	}

	if len(schemeCaps) == 0 && len(reqCaps) == 0 {
		return nil
	}

	// Sort schemes by scheme_name for deterministic output.
	sort.Slice(schemeCaps, func(i, j int) bool {
		return schemeCaps[i].SchemeName < schemeCaps[j].SchemeName
	})

	var schemes []json.RawMessage
	for _, c := range schemeCaps {
		if b, err := json.Marshal(c); err == nil {
			schemes = append(schemes, b)
		}
	}

	// Sort requirements by their derived name (SkillRef+sorted scheme keys) for
	// deterministic output.
	sort.Slice(reqCaps, func(i, j int) bool {
		return reqCaps[i].derivedName() < reqCaps[j].derivedName()
	})

	var requirements []json.RawMessage
	for _, c := range reqCaps {
		if b, err := json.Marshal(c); err == nil {
			requirements = append(requirements, b)
		}
	}

	return &securityDetailJSON{
		SecuritySchemes:      schemes,
		SecurityRequirements: requirements,
	}
}

// MarshalJSON produces a flat, backward-compatible JSON representation that
// merges AgentType fields into the CatalogEntry output.
func (e CatalogEntry) MarshalJSON() ([]byte, error) {
	e.SyncFromDB()
	return json.Marshal(e.toCatalogEntryJSON())
}

func (e CatalogEntry) toCatalogEntryJSON() catalogEntryJSON {
	var (
		protocol     Protocol
		endpoint     string
		version      string
		specVersion  string
		provider     *Provider
		capabilities []Capability
	)
	if e.AgentType != nil {
		protocol = e.AgentType.Protocol
		endpoint = e.AgentType.Endpoint
		version = e.AgentType.Version
		specVersion = e.AgentType.SpecVersion
		provider = e.AgentType.Provider
		capabilities = e.AgentType.Capabilities
	}
	var capJSON json.RawMessage
	if len(capabilities) > 0 {
		if b, err := MarshalCapabilitiesJSON(capabilities); err == nil {
			capJSON = b
		}
	}
	var authSummary *authSummaryJSON
	if len(capabilities) > 0 {
		authSummary = buildAuthSummary(capabilities)
	}
	var secDetail *securityDetailJSON
	if len(capabilities) > 0 {
		secDetail = buildSecurityDetail(capabilities)
	}
	return catalogEntryJSON{
		ID: e.ID, AgentTypeID: e.AgentTypeID, DisplayName: e.DisplayName,
		Description: e.Description, Protocol: protocol, Endpoint: endpoint,
		Version: version, SpecVersion: specVersion, Status: e.Status,
		Source: e.Source, Provider: provider, Categories: e.Categories,
		Capabilities: capJSON, Validity: e.Validity, Metadata: e.Metadata,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		Health: healthJSON{
			State: string(e.Health.State), LastProbedAt: e.Health.LastProbedAt,
			LastSuccessAt: e.Health.LastSuccessAt, LatencyMs: e.Health.LatencyMs,
			ConsecutiveFailures: e.Health.ConsecutiveFailures, LastError: e.Health.LastError,
		},
		AuthSummary:    authSummary,
		SecurityDetail: secDetail,
	}
}
