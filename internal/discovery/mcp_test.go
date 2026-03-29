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

	entry, err := discovery.ParseMCPCard(raw, model.SourceConfig)
	require.NoError(t, err)
	assert.Equal(t, "Code MCP", entry.DisplayName)
	assert.Equal(t, "Code execution server", entry.Description)
	assert.Equal(t, "http://mcp.example.com", entry.Endpoint)
	assert.Equal(t, "2.0.0", entry.Version)
	assert.Equal(t, model.ProtocolMCP, entry.Protocol)
	require.Len(t, entry.Skills, 2)
	assert.Equal(t, "run-code", entry.Skills[0].Name)
}

func TestParseMCPCard_MissingName(t *testing.T) {
	raw := []byte(`{"description": "no name"}`)
	_, err := discovery.ParseMCPCard(raw, model.SourceConfig)
	require.Error(t, err)
}

func TestParseMCPCard_NoRemotes(t *testing.T) {
	raw := []byte(`{"name": "No Remotes MCP"}`)
	entry, err := discovery.ParseMCPCard(raw, model.SourceConfig)
	require.NoError(t, err)
	assert.Empty(t, entry.Endpoint)
}
