package tools

// RegisterAll registers the 4 built-in MCP discovery tools into r.
// Called from Plugin.Init() once the loopback function is available.
//
// v2 translator path (spec §6.5): the OpenAPI-to-MCP translator will call
// RegisterAll with generated tools that carry the same ToolDescriptor shape.
// No changes to this function or Registry are required for v2 compatibility.
func RegisterAll(r *Registry, lb LoopbackFunc) {
	r.Register(ToolDescriptor{
		Name:        "agent_search",
		Description: "Search the AgentLens catalog for AI agents matching a query. Returns a list of matching agents with their metadata, capabilities, and health status. Use this to discover agents for a specific task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":    map[string]any{"type": "string", "description": "Free-text search across agent names, descriptions, and capabilities"},
				"protocol": map[string]any{"type": "string", "description": "Filter by protocol: 'a2a', 'mcp', or 'a2ui'"},
				"limit":    map[string]any{"type": "integer", "description": "Maximum results (1-100, default 20)"},
			},
			"required": []string{"query"},
		},
	}, agentSearch(lb))

	r.Register(ToolDescriptor{
		Name:        "agent_get",
		Description: "Retrieve detailed information about a specific agent by its catalog ID, including capabilities, auth requirements, and health status.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"id": map[string]any{"type": "string", "description": "Catalog entry ID"}},
			"required":   []string{"id"},
		},
	}, agentGet(lb))

	r.Register(ToolDescriptor{
		Name:        "capabilities_list",
		Description: "List all capabilities (skills, tools, resources) offered by a specific agent.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"agent_id": map[string]any{"type": "string", "description": "Catalog entry ID of the agent"}},
			"required":   []string{"agent_id"},
		},
	}, capabilitiesList(lb))

	r.Register(ToolDescriptor{
		Name:        "agent_card",
		Description: "Fetch the raw protocol card (A2A agent card or MCP server card) for an agent. Returns the verbatim JSON/YAML card as received from the agent endpoint.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"agent_id": map[string]any{"type": "string", "description": "Catalog entry ID"}},
			"required":   []string{"agent_id"},
		},
	}, agentCard(lb))
}
