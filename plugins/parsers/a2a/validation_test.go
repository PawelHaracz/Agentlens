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

	agent, err := p.Parse(raw)
	require.NoError(t, err)

	assert.Equal(t, model.ProtocolA2A, agent.Protocol)
	assert.Equal(t, "1.0", agent.SpecVersion)
	// Endpoint from supportedInterfaces[0].
	assert.Equal(t, "https://translate.example.com/a2a", agent.Endpoint)
	assert.NotEmpty(t, agent.AgentKey)

	// Capabilities: 2 skills + 3 extensions + 2 security + 1 interface + 1 signature = 9
	assert.Len(t, agent.Capabilities, 9)

	// Count by kind.
	kinds := map[string]int{}
	for _, c := range agent.Capabilities {
		kinds[c.Kind()]++
	}
	assert.Equal(t, 2, kinds["a2a.skill"])
	assert.Equal(t, 3, kinds["a2a.extension"])
	assert.Equal(t, 2, kinds["a2a.security_scheme"])
	assert.Equal(t, 1, kinds["a2a.interface"])
	assert.Equal(t, 1, kinds["a2a.signature"])
}

func TestParse_V03Card(t *testing.T) {
	raw := loadFixture(t, "a2a_v03_card.json")
	p := a2a.New()

	agent, err := p.Parse(raw)
	require.NoError(t, err)

	assert.Equal(t, "0.3", agent.SpecVersion)
	assert.Equal(t, "https://weather.example.com/a2a", agent.Endpoint)
	assert.NotEmpty(t, agent.AgentKey)

	// Count skill capabilities.
	skillCount := 0
	for _, c := range agent.Capabilities {
		if c.Kind() == "a2a.skill" {
			skillCount++
		}
	}
	assert.Equal(t, 2, skillCount)

	// Non-skill capabilities: 2 ext + 2 security + 2 iface + 1 sig = 7
	nonSkillCount := len(agent.Capabilities) - skillCount
	assert.Equal(t, 7, nonSkillCount)
}

func TestParse_LegacyCard(t *testing.T) {
	raw := loadFixture(t, "a2a_legacy_card.json")
	p := a2a.New()

	agent, err := p.Parse(raw)
	require.NoError(t, err)

	assert.Equal(t, "", agent.SpecVersion)
	// Endpoint from url (no supportedInterfaces).
	assert.Equal(t, "https://legacy.example.com/agent", agent.Endpoint)

	// Count skill capabilities.
	skillCount := 0
	for _, c := range agent.Capabilities {
		if c.Kind() == "a2a.skill" {
			skillCount++
		}
	}
	assert.Equal(t, 1, skillCount)

	// No meta capabilities for legacy cards (no extensions, security, interfaces, signatures).
	nonSkillCount := len(agent.Capabilities) - skillCount
	assert.Equal(t, 0, nonSkillCount)
}

// --------------- Security Schemes parsing tests (Task 4) ---------------

func TestParse_SecuritySchemes_V10_Bearer(t *testing.T) {
	data := loadFixture(t, "v10_bearer_only.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var schemes []*model.A2ASecurityScheme
	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_scheme" {
			schemes = append(schemes, cap.(*model.A2ASecurityScheme))
		}
	}

	require.Len(t, schemes, 1, "Expected 1 security scheme")

	scheme := schemes[0]
	assert.Equal(t, "httpAuth", scheme.SchemeName, "Expected SchemeName 'httpAuth'")
	assert.Equal(t, "http", scheme.Type, "Expected Type 'http'")
	assert.Equal(t, "Bearer", scheme.HTTPScheme, "Expected HTTPScheme 'Bearer'")
	assert.Equal(t, "JWT", scheme.BearerFormat, "Expected BearerFormat 'JWT'")
}

func TestParse_SecuritySchemes_V10_OAuth2(t *testing.T) {
	data := loadFixture(t, "v10_oauth2_authcode.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var schemes []*model.A2ASecurityScheme
	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_scheme" {
			schemes = append(schemes, cap.(*model.A2ASecurityScheme))
		}
	}

	require.Len(t, schemes, 1, "Expected 1 security scheme")

	scheme := schemes[0]
	assert.Equal(t, "oauth2", scheme.Type, "Expected Type 'oauth2'")
	require.Len(t, scheme.OAuthFlows, 1, "Expected 1 OAuth flow")
	assert.Equal(t, "authorizationCode", scheme.OAuthFlows[0].FlowType)
	assert.Len(t, scheme.OAuthFlows[0].Scopes, 3, "Expected 3 scopes")
}

func TestParse_SecuritySchemes_V03_Array(t *testing.T) {
	data := loadFixture(t, "v03_security_array.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var schemes []*model.A2ASecurityScheme
	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_scheme" {
			schemes = append(schemes, cap.(*model.A2ASecurityScheme))
		}
	}

	require.Len(t, schemes, 2, "Expected 2 security schemes (v0.3 array format)")

	foundHTTP := false
	for _, s := range schemes {
		if s.Type == "http" {
			foundHTTP = true
			assert.Equal(t, "Bearer", s.Method, "Expected Method 'Bearer'")
		}
	}
	assert.True(t, foundHTTP, "Expected to find http scheme in v0.3 array")
}

// --------------- Security Requirements parsing tests (Task 5) ---------------

func TestParse_SecurityRequirements(t *testing.T) {
	data := loadFixture(t, "v10_bearer_only.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var reqs []*model.A2ASecurityRequirement
	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_requirement" {
			reqs = append(reqs, cap.(*model.A2ASecurityRequirement))
		}
	}

	require.Len(t, reqs, 1, "Expected 1 security requirement")

	req := reqs[0]
	require.Len(t, req.Schemes, 1, "Expected 1 scheme in requirement")
	_, ok := req.Schemes["httpAuth"]
	assert.True(t, ok, "Expected 'httpAuth' in requirement schemes")
	assert.Equal(t, "", req.SkillRef, "Expected empty SkillRef for top-level requirement")
}

func TestParse_SecurityRequirements_MultipleSchemes(t *testing.T) {
	data := loadFixture(t, "v10_multiple_schemes.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var reqs []*model.A2ASecurityRequirement
	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_requirement" {
			reqs = append(reqs, cap.(*model.A2ASecurityRequirement))
		}
	}

	require.Len(t, reqs, 2, "Expected 2 security requirements (OR'd)")
}

// --------------- Per-Skill Security Requirements tests (Task 6) ---------------

func TestParse_SkillSecurityRequirements(t *testing.T) {
	data := loadFixture(t, "v10_skill_security.json")

	p := a2a.New()
	agentType, err := p.Parse(data)
	require.NoError(t, err)

	var topLevel []*model.A2ASecurityRequirement
	var skillLevel []*model.A2ASecurityRequirement

	for _, cap := range agentType.Capabilities {
		if cap.Kind() == "a2a.security_requirement" {
			req := cap.(*model.A2ASecurityRequirement)
			if req.SkillRef == "" {
				topLevel = append(topLevel, req)
			} else {
				skillLevel = append(skillLevel, req)
			}
		}
	}

	assert.Len(t, topLevel, 1, "Expected 1 top-level requirement")

	require.Len(t, skillLevel, 1, "Expected 1 skill-level requirement")

	skillReq := skillLevel[0]
	assert.Equal(t, "createDocument", skillReq.SkillRef)
	_, ok := skillReq.Schemes["apiKeyAuth"]
	assert.True(t, ok, "Expected 'apiKeyAuth' in skill requirement")
}
