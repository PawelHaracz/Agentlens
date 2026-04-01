package a2a_test

import (
	"os"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// --------------- ValidateCard tests ---------------

func TestValidateCard_V03(t *testing.T) {
	raw := loadFixture(t, "a2a_v03_card.json")
	result := a2a.ValidateCard(raw)

	assert.True(t, result.Valid)
	assert.Equal(t, "0.3", result.SpecVersion)
	assert.Empty(t, result.Errors)

	require.NotNil(t, result.Preview)
	assert.Equal(t, "Weather Agent v0.3", result.Preview.DisplayName)
	assert.Equal(t, "a2a", result.Preview.Protocol)
	assert.Equal(t, "0.3", result.Preview.SpecVersion)
	assert.Equal(t, 2, result.Preview.SkillsCount)
	assert.Equal(t, 2, result.Preview.ExtensionsCount)
	assert.Equal(t, []string{"bearer", "apiKey"}, result.Preview.SecuritySchemes)
	assert.Equal(t, []string{"jsonrpc", "http"}, result.Preview.Interfaces)
}

func TestValidateCard_V10(t *testing.T) {
	raw := loadFixture(t, "a2a_v10_card.json")
	result := a2a.ValidateCard(raw)

	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	assert.Empty(t, result.Errors)

	require.NotNil(t, result.Preview)
	assert.Equal(t, "Translation Agent v1.0", result.Preview.DisplayName)
	assert.Equal(t, "1.0", result.Preview.SpecVersion)
	assert.Equal(t, 2, result.Preview.SkillsCount)
	assert.Equal(t, 3, result.Preview.ExtensionsCount)
	assert.Equal(t, []string{"oauth2", "mtls"}, result.Preview.SecuritySchemes)
	assert.Equal(t, []string{"jsonrpc"}, result.Preview.Interfaces)
}

func TestValidateCard_Legacy(t *testing.T) {
	raw := loadFixture(t, "a2a_legacy_card.json")
	result := a2a.ValidateCard(raw)

	assert.True(t, result.Valid)
	assert.Equal(t, "", result.SpecVersion)
	assert.Empty(t, result.Errors)

	require.NotNil(t, result.Preview)
	assert.Equal(t, "Legacy Agent", result.Preview.DisplayName)
	assert.Equal(t, 1, result.Preview.SkillsCount)
	assert.Equal(t, 0, result.Preview.ExtensionsCount)
}

func TestValidateCard_Invalid(t *testing.T) {
	raw := loadFixture(t, "a2a_invalid_card.json")
	result := a2a.ValidateCard(raw)

	assert.False(t, result.Valid)
	assert.Nil(t, result.Preview)

	// Collect field names from errors for easier assertion.
	fields := make(map[string]string)
	for _, e := range result.Errors {
		fields[e.Field] = e.Message
	}

	assert.Contains(t, fields, "name")
	assert.Contains(t, fields, "capabilities.extensions[0].uri")
	assert.Contains(t, fields, "skills[0].id")
	assert.Contains(t, fields, "skills[0].description")
}

func TestValidateCard_InvalidJSON(t *testing.T) {
	result := a2a.ValidateCard([]byte(`{not json}`))

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "json", result.Errors[0].Field)
}

func TestValidateCard_V10_DeprecatedFields(t *testing.T) {
	// Build a minimal valid card with stateTransitionHistory set.
	raw := []byte(`{
		"name": "Deprecation Test",
		"description": "Tests deprecation warnings",
		"url": "https://example.com/a2a",
		"version": "1.0.0",
		"capabilities": {
			"supportsExtendedAgentCard": true
		},
		"stateTransitionHistory": true,
		"skills": [
			{"id": "s1", "name": "Skill", "description": "A skill"}
		]
	}`)
	result := a2a.ValidateCard(raw)

	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "stateTransitionHistory")
}

// --------------- Parse tests ---------------

func TestParse_V10Card(t *testing.T) {
	raw := loadFixture(t, "a2a_v10_card.json")
	p := a2a.New()

	entry, err := p.Parse(raw, model.SourceConfig)
	require.NoError(t, err)

	assert.Equal(t, "Translation Agent v1.0", entry.DisplayName)
	assert.Equal(t, model.ProtocolA2A, entry.Protocol)
	assert.Equal(t, "1.0", entry.SpecVersion)
	// Endpoint from supportedInterfaces[0].
	assert.Equal(t, "https://translate.example.com/a2a", entry.Endpoint)
	assert.Equal(t, model.SourceConfig, entry.Source)

	// TypedMeta: 3 extensions + 2 security + 1 interface + 1 signature = 7
	assert.Len(t, entry.TypedMeta, 7)

	// Count by kind.
	kinds := map[string]int{}
	for _, m := range entry.TypedMeta {
		kinds[m.Kind()]++
	}
	assert.Equal(t, 3, kinds["a2a.extension"])
	assert.Equal(t, 2, kinds["a2a.security_scheme"])
	assert.Equal(t, 1, kinds["a2a.interface"])
	assert.Equal(t, 1, kinds["a2a.signature"])
}

func TestParse_V03Card(t *testing.T) {
	raw := loadFixture(t, "a2a_v03_card.json")
	p := a2a.New()

	entry, err := p.Parse(raw, model.SourcePush)
	require.NoError(t, err)

	assert.Equal(t, "0.3", entry.SpecVersion)
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.Equal(t, "https://weather.example.com/a2a", entry.Endpoint)
	assert.Len(t, entry.Skills, 2)

	// TypedMeta: 2 ext + 2 security + 2 iface + 1 sig = 7
	assert.Len(t, entry.TypedMeta, 7)
}

func TestParse_LegacyCard(t *testing.T) {
	raw := loadFixture(t, "a2a_legacy_card.json")
	p := a2a.New()

	entry, err := p.Parse(raw, model.SourceK8s)
	require.NoError(t, err)

	assert.Equal(t, "", entry.SpecVersion)
	// Endpoint from url (no supportedInterfaces).
	assert.Equal(t, "https://legacy.example.com/agent", entry.Endpoint)
	assert.Len(t, entry.Skills, 1)
	// No typed metadata for legacy cards.
	assert.Empty(t, entry.TypedMeta)
}
