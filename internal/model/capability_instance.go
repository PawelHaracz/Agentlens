package model

// CapabilityInstance represents a single capability from a single agent,
// enriched with agent metadata for the discovery view.
type CapabilityInstance struct {
	// Capability fields
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	InputModes  []string `json:"input_modes"`
	OutputModes []string `json:"output_modes"`

	// Parent agent fields (subset — not the full CatalogEntry)
	AgentID     string         `json:"agent_id"`
	AgentName   string         `json:"agent_name"`
	Protocol    Protocol       `json:"protocol"`
	Status      LifecycleState `json:"status"`
	SpecVersion string         `json:"spec_version"`

	// Provider (flattened)
	ProviderOrg *string `json:"provider_org"`
	ProviderURL *string `json:"provider_url"`

	// Health (subset)
	HealthState LifecycleState `json:"health_state"`
	LatencyMs   int64          `json:"latency_ms"`
}

// CapabilityListResult wraps a paginated list of capability instances.
type CapabilityListResult struct {
	Total int                  `json:"total"`
	Items []CapabilityInstance `json:"items"`
}
