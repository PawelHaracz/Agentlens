package discovery_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestParseA2ACard_Valid(t *testing.T) {
	raw := []byte(`{
		"name": "Weather Agent",
		"description": "Provides weather info",
		"url": "http://weather.example.com",
		"version": "1.2.3",
		"provider": {"organization": "Acme Corp"},
		"skills": [
			{
				"name": "get-weather",
				"description": "Gets current weather",
				"inputModes": ["text"],
				"outputModes": ["text", "json"]
			}
		]
	}`)

	entry, err := discovery.ParseA2ACard(raw, model.SourceConfig)
	require.NoError(t, err)
	assert.Equal(t, "Weather Agent", entry.DisplayName)
	assert.Equal(t, "Provides weather info", entry.Description)
	assert.Equal(t, "http://weather.example.com", entry.Endpoint)
	assert.Equal(t, "1.2.3", entry.Version)
	assert.Equal(t, "Acme Corp", entry.Provider.Organization)
	assert.Equal(t, model.ProtocolA2A, entry.Protocol)
	require.Len(t, entry.Skills, 1)
	assert.Equal(t, "get-weather", entry.Skills[0].Name)
	assert.Equal(t, []string{"text"}, entry.Skills[0].InputModes)
	assert.Equal(t, []string{"text", "json"}, entry.Skills[0].OutputModes)
	assert.NotEmpty(t, entry.RawCard)
}

func TestParseA2ACard_Minimal(t *testing.T) {
	raw := []byte(`{"name": "Minimal Agent", "url": "http://minimal.example.com"}`)
	entry, err := discovery.ParseA2ACard(raw, model.SourceConfig)
	require.NoError(t, err)
	assert.Equal(t, "Minimal Agent", entry.DisplayName)
	assert.Equal(t, "http://minimal.example.com", entry.Endpoint)
	assert.Empty(t, entry.Provider.Organization)
	assert.Empty(t, entry.Skills)
}

func TestParseA2ACard_MissingName(t *testing.T) {
	raw := []byte(`{"url": "http://example.com"}`)
	_, err := discovery.ParseA2ACard(raw, model.SourceConfig)
	require.Error(t, err)
}

func TestParseA2ACard_MissingURL(t *testing.T) {
	raw := []byte(`{"name": "No URL Agent"}`)
	_, err := discovery.ParseA2ACard(raw, model.SourceConfig)
	require.Error(t, err)
}
