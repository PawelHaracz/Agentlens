package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestParseMCPCard_Valid(t *testing.T) {
	raw := []byte(`{
		"name": "Code MCP",
		"description": "Code execution server",
		"version": "2.0.0",
		"remotes": [{"url": "http://mcp.example.com"}],
		"tools": [
			{"name": "run-code", "description": "Executes code"},
			{"name": "lint", "description": "Lints code"}
		]
	}`)

	at, err := discovery.ParseMCPCard(raw)
	require.NoError(t, err)
	assert.Equal(t, "http://mcp.example.com", at.Endpoint)
	assert.Equal(t, "2.0.0", at.Version)
	assert.Equal(t, model.ProtocolMCP, at.Protocol)

	// Find tool capabilities
	var tools []*model.MCPTool
	for _, cap := range at.Capabilities {
		if t, ok := cap.(*model.MCPTool); ok {
			tools = append(tools, t)
		}
	}
	require.Len(t, tools, 2)
	assert.Equal(t, "run-code", tools[0].Name)
}

func TestParseMCPCard_MissingName(t *testing.T) {
	raw := []byte(`{"description": "no name"}`)
	_, err := discovery.ParseMCPCard(raw)
	require.Error(t, err)
}

func TestParseMCPCard_NoRemotes(t *testing.T) {
	raw := []byte(`{"name": "No Remotes MCP"}`)
	_, err := discovery.ParseMCPCard(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remotes")
}
