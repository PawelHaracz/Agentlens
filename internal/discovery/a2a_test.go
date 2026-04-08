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

	at, err := discovery.ParseA2ACard(raw)
	require.NoError(t, err)
	assert.Equal(t, "http://weather.example.com", at.Endpoint)
	assert.Equal(t, "1.2.3", at.Version)
	assert.NotNil(t, at.Provider)
	assert.Equal(t, "Acme Corp", at.Provider.Organization)
	assert.Equal(t, model.ProtocolA2A, at.Protocol)
	assert.NotEmpty(t, at.Capabilities)

	// Find skill capability
	var skill *model.A2ASkill
	for _, cap := range at.Capabilities {
		if s, ok := cap.(*model.A2ASkill); ok {
			skill = s
			break
		}
	}
	require.NotNil(t, skill)
	assert.Equal(t, "get-weather", skill.Name)
	assert.Equal(t, []string{"text"}, skill.InputModes)
	assert.Equal(t, []string{"text", "json"}, skill.OutputModes)
}

func TestParseA2ACard_Minimal(t *testing.T) {
	raw := []byte(`{"name": "Minimal Agent", "url": "http://minimal.example.com"}`)
	at, err := discovery.ParseA2ACard(raw)
	require.NoError(t, err)
	assert.Equal(t, "http://minimal.example.com", at.Endpoint)
}

func TestParseA2ACard_MissingName(t *testing.T) {
	raw := []byte(`{"url": "http://example.com"}`)
	_, err := discovery.ParseA2ACard(raw)
	require.Error(t, err)
}

func TestParseA2ACard_MissingURL(t *testing.T) {
	raw := []byte(`{"name": "No URL Agent"}`)
	_, err := discovery.ParseA2ACard(raw)
	require.Error(t, err)
}
