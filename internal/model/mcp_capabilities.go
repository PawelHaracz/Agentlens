package model

import "fmt"

func init() {
	RegisterCapability("mcp.tool", func() Capability { return &MCPTool{} }, true)
	RegisterCapability("mcp.resource", func() Capability { return &MCPResource{} }, true)
	RegisterCapability("mcp.prompt", func() Capability { return &MCPPrompt{} }, true)
}

// MCPTool represents an MCP tool capability (kind "mcp.tool").
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// Kind returns the capability kind identifier.
func (t *MCPTool) Kind() string { return "mcp.tool" }

// Validate checks that required fields are present.
func (t *MCPTool) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("mcp.tool: name is required")
	}
	return nil
}

// MCPResource represents an MCP resource capability (kind "mcp.resource").
type MCPResource struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

// Kind returns the capability kind identifier.
func (r *MCPResource) Kind() string { return "mcp.resource" }

// Validate checks that required fields are present.
func (r *MCPResource) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("mcp.resource: name is required")
	}
	return nil
}

// MCPPrompt represents an MCP prompt capability (kind "mcp.prompt").
type MCPPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Arguments   []any  `json:"arguments,omitempty"`
}

// Kind returns the capability kind identifier.
func (p *MCPPrompt) Kind() string { return "mcp.prompt" }

// Validate checks that required fields are present.
func (p *MCPPrompt) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("mcp.prompt: name is required")
	}
	return nil
}
