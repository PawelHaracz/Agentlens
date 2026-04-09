package model

// CapabilityInstance represents a single capability from a single agent,
// enriched with agent metadata for the discovery view.
type CapabilityInstance struct {
	// Capability fields
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"input_modes,omitempty"`
	OutputModes []string `json:"output_modes,omitempty"`

	// Parent agent fields (subset — not the full CatalogEntry)
	AgentID     string         `json:"agent_id"`
	AgentName   string         `json:"agent_name"`
	Protocol    Protocol       `json:"protocol"`
	Status      LifecycleState `json:"status"`
	SpecVersion string         `json:"spec_version,omitempty"`

	// Provider (flattened)
	ProviderOrg string `json:"provider_org,omitempty"`
	ProviderURL string `json:"provider_url,omitempty"`

	// Health (subset)
	HealthState LifecycleState `json:"health_state"`
	LatencyMs   int64          `json:"latency_ms"`
}

// CapabilityListResult wraps a paginated list of capability instances.
type CapabilityListResult struct {
	Total int                  `json:"total"`
	Items []CapabilityInstance `json:"items"`
}
