// Package a2a provides the A2A protocol parser plugin.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
)

type a2aProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// Plugin implements the A2A parser plugin.
type Plugin struct {
	initialized bool
	metrics     *telemetry.ParserMetrics
}

// New creates a new A2A parser plugin.
func New() *Plugin {
	return &Plugin{
		metrics: telemetry.NewParserMetrics(),
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "a2a-parser" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeParser }

// Protocol returns the protocol this parser handles.
func (p *Plugin) Protocol() model.Protocol { return model.ProtocolA2A }

// CardPath returns the default card path for A2A.
func (p *Plugin) CardPath() string { return "/.well-known/agent-card.json" }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.initialized = true
	p.metrics = telemetry.NewParserMetrics()
	return nil
}

// Start starts the plugin (no-op for parser).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for parser).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Parse parses an A2A agent card JSON blob into an AgentType.
func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	start := time.Now()

	tracer := otel.Tracer("agentlens.parser")
	ctx, span := tracer.Start(context.Background(), "parser.a2a.parse",
		trace.WithAttributes(
			attribute.String("agentlens.parser.type", "a2a"),
			attribute.Int64("agentlens.parser.input_size", int64(len(raw))),
		),
	)
	defer span.End()

	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		err = fmt.Errorf("parsing a2a card: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "a2a", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}
	if card.Name == "" {
		err := fmt.Errorf("a2a card missing required field: name")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "a2a", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}

	endpoint, err := resolveEndpoint(&card)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "a2a", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}

	capabilities := buildSkills(&card)
	capabilities = append(capabilities, buildA2AMetaCaps(&card)...)

	securityCaps, err := buildSecurityCaps(&card)
	if err != nil {
		slog.Warn("Failed to parse security schemes", "error", err)
	} else {
		capabilities = append(capabilities, securityCaps...)
	}

	capabilities = append(capabilities, buildSecurityRequirements(&card)...)

	provider := buildProvider(&card)
	specVersion := detectSpecVersion(&card)
	agentKey := model.ComputeAgentKey(model.ProtocolA2A, endpoint)

	skillCount := len(card.Skills)
	extensionCount := 0
	if card.Capabilities != nil {
		extensionCount = len(card.Capabilities.Extensions)
	}
	securitySchemeCount := countSecuritySchemes(card.SecuritySchemes)

	span.SetAttributes(
		attribute.String("agentlens.parser.spec_version", specVersion),
		attribute.Int("agentlens.parser.skill_count", skillCount),
		attribute.Int("agentlens.parser.extension_count", extensionCount),
		attribute.Int("agentlens.parser.security_scheme_count", securitySchemeCount),
	)
	p.metrics.RecordInvocation(ctx, "a2a", "success", specVersion, time.Since(start).Seconds()*1000)

	return &model.AgentType{
		AgentKey:     agentKey,
		Protocol:     model.ProtocolA2A,
		Endpoint:     endpoint,
		Version:      card.Version,
		SpecVersion:  specVersion,
		Provider:     &provider,
		Capabilities: capabilities,
		RawBytes:     raw,
	}, nil
}

// countSecuritySchemes returns the number of security schemes in the raw JSON,
// supporting both v1.0 named-map and v0.3 array formats.
func countSecuritySchemes(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var mapSchemes map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mapSchemes); err == nil {
		return len(mapSchemes)
	}
	var arrSchemes []json.RawMessage
	if err := json.Unmarshal(raw, &arrSchemes); err == nil {
		return len(arrSchemes)
	}
	return 0
}

// resolveEndpoint determines the agent endpoint from the card, preferring
// the first supportedInterfaces URL over the top-level url field.
func resolveEndpoint(card *fullCard) (string, error) {
	endpoint := card.URL
	if len(card.SupportedInterfaces) > 0 && card.SupportedInterfaces[0].URL != "" {
		endpoint = card.SupportedInterfaces[0].URL
	}
	if endpoint == "" {
		return "", fmt.Errorf("a2a card missing required field: url or supportedInterfaces")
	}
	return endpoint, nil
}

// buildSkills converts raw card skills into []model.Capability with *model.A2ASkill entries
// and includes per-skill A2ASecurityRequirement capabilities when present.
func buildSkills(card *fullCard) []model.Capability {
	caps := make([]model.Capability, 0, len(card.Skills))
	for _, s := range card.Skills {
		caps = append(caps, &model.A2ASkill{
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			InputModes:  s.InputModes,
			OutputModes: s.OutputModes,
		})
		if len(s.SecurityRequirements) > 0 {
			skillSecCaps := parseSkillSecurityRequirements(s.Name, &s)
			caps = append(caps, skillSecCaps...)
		}
	}
	return caps
}

// buildProvider constructs a model.Provider from the card's provider field.
func buildProvider(card *fullCard) model.Provider {
	var provider model.Provider
	if card.Provider != nil {
		provider.Organization = card.Provider.Organization
		provider.Team = card.Provider.Organization
		provider.URL = card.Provider.URL
	}
	return provider
}

// buildA2AMetaCaps assembles typed capabilities from extensions,
// interfaces, and signatures declared in the card.
// Security schemes are handled separately by buildSecurityCaps.
func buildA2AMetaCaps(card *fullCard) []model.Capability {
	var caps []model.Capability

	if card.Capabilities != nil {
		for _, ext := range card.Capabilities.Extensions {
			caps = append(caps, &model.A2AExtension{
				URI:      ext.URI,
				Required: ext.Required,
			})
		}
	}

	for _, iface := range card.SupportedInterfaces {
		caps = append(caps, &model.A2AInterface{
			URL:     iface.URL,
			Binding: iface.Binding,
		})
	}

	for _, sig := range card.Signatures {
		caps = append(caps, &model.A2ASignature{
			Algorithm: sig.Algorithm,
			KeyID:     sig.KeyID,
		})
	}

	return caps
}

// buildSecurityCaps parses security schemes from the card into capabilities.
// Supports both v0.3 array format and v1.0 named map format.
func buildSecurityCaps(card *fullCard) ([]model.Capability, error) {
	var caps []model.Capability

	if len(card.SecuritySchemes) == 0 {
		return caps, nil
	}

	// Try v1.0 map format first.
	var v10Schemes map[string]json.RawMessage
	if err := json.Unmarshal(card.SecuritySchemes, &v10Schemes); err == nil {
		// Sort scheme names for deterministic capability ordering.
		schemeNames := make([]string, 0, len(v10Schemes))
		for schemeName := range v10Schemes {
			schemeNames = append(schemeNames, schemeName)
		}
		sort.Strings(schemeNames)
		for _, schemeName := range schemeNames {
			scheme, err := parseSecurityScheme(schemeName, v10Schemes[schemeName])
			if err != nil {
				slog.Warn("Failed to parse security scheme", "scheme", schemeName, "error", err)
				continue
			}
			caps = append(caps, scheme)
		}
		return caps, nil
	}

	// Try v0.3 array format.
	var v03Schemes []json.RawMessage
	if err := json.Unmarshal(card.SecuritySchemes, &v03Schemes); err == nil {
		for i, schemeData := range v03Schemes {
			var typeHolder struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(schemeData, &typeHolder); err != nil {
				slog.Warn("Failed to extract type from v0.3 scheme", "index", i, "error", err)
				continue
			}
			// Include the index to prevent name collisions when multiple schemes
			// share the same type (e.g., two "http" variants in the same card).
			schemeName := fmt.Sprintf("%sAuth%d", typeHolder.Type, i)
			scheme, err := parseSecuritySchemeV03(schemeName, schemeData)
			if err != nil {
				slog.Warn("Failed to parse v0.3 security scheme", "index", i, "error", err)
				continue
			}
			caps = append(caps, scheme)
		}
		return caps, nil
	}

	return caps, fmt.Errorf("securitySchemes is neither v1.0 map nor v0.3 array")
}

// parseSecurityScheme parses a single v1.0 security scheme entry from a named map.
func parseSecurityScheme(schemeName string, data json.RawMessage) (*model.A2ASecurityScheme, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	schemeType, ok := raw["type"].(string)
	if !ok || schemeType == "" {
		return nil, fmt.Errorf("missing or invalid type")
	}

	scheme := &model.A2ASecurityScheme{
		SchemeName: schemeName,
		Name:       schemeName,
		Type:       schemeType,
	}

	if desc, ok := raw["description"].(string); ok {
		scheme.Description = desc
	}

	switch schemeType {
	case "apiKey":
		if in, ok := raw["in"].(string); ok {
			scheme.APIKeyLocation = in
		}
		if name, ok := raw["name"].(string); ok {
			scheme.APIKeyName = name
		}
	case "http":
		if httpScheme, ok := raw["scheme"].(string); ok {
			scheme.HTTPScheme = httpScheme
		}
		if bearerFormat, ok := raw["bearerFormat"].(string); ok {
			scheme.BearerFormat = bearerFormat
		}
	case "oauth2":
		applyOAuthFlows(schemeName, raw, scheme)
		if metadataURL, ok := raw["oauth2MetadataUrl"].(string); ok {
			scheme.OAuth2MetadataURL = metadataURL
		}
	case "openIdConnect":
		if oidcURL, ok := raw["openIdConnectUrl"].(string); ok {
			scheme.OpenIDConnectURL = oidcURL
		}
	case "mutualTls":
		// No additional fields.
	default:
		slog.Warn("Unknown security scheme type", "type", schemeType, "scheme", schemeName)
	}

	return scheme, nil
}

// applyOAuthFlows populates oauth flow details on a scheme from the raw map.
func applyOAuthFlows(schemeName string, raw map[string]interface{}, scheme *model.A2ASecurityScheme) {
	flowsRaw, ok := raw["flows"].(map[string]interface{})
	if !ok {
		return
	}
	// Sort flow type names for deterministic ordering.
	flowTypes := make([]string, 0, len(flowsRaw))
	for ft := range flowsRaw {
		flowTypes = append(flowTypes, ft)
	}
	sort.Strings(flowTypes)
	for _, flowType := range flowTypes {
		flowData := flowsRaw[flowType]
		flowMap, ok := flowData.(map[string]interface{})
		if !ok {
			continue
		}
		flow := model.A2AOAuthFlow{FlowType: flowType}
		if flowType == "implicit" || flowType == "password" {
			flow.Deprecated = true
			slog.Warn("Deprecated OAuth flow detected", "flow", flowType, "scheme", schemeName)
		}
		if authURL, ok := flowMap["authorizationUrl"].(string); ok {
			flow.AuthorizationURL = authURL
		}
		if tokenURL, ok := flowMap["tokenUrl"].(string); ok {
			flow.TokenURL = tokenURL
		}
		if refreshURL, ok := flowMap["refreshUrl"].(string); ok {
			flow.RefreshURL = refreshURL
		}
		if deviceAuthURL, ok := flowMap["deviceAuthorizationUrl"].(string); ok {
			flow.DeviceAuthURL = deviceAuthURL
		}
		if scopesRaw, ok := flowMap["scopes"].(map[string]interface{}); ok {
			scopes := make(map[string]string)
			for k, v := range scopesRaw {
				if vStr, ok := v.(string); ok {
					scopes[k] = vStr
				}
			}
			flow.Scopes = scopes
		}
		scheme.OAuthFlows = append(scheme.OAuthFlows, flow)
	}
}

// parseSecuritySchemeV03 parses a single v0.3 security scheme entry from an array.
func parseSecuritySchemeV03(schemeName string, data json.RawMessage) (*model.A2ASecurityScheme, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	schemeType, ok := raw["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing type in v0.3 scheme")
	}

	scheme := &model.A2ASecurityScheme{
		SchemeName: schemeName,
		Name:       schemeName,
		Type:       schemeType,
	}

	if desc, ok := raw["description"].(string); ok {
		scheme.Description = desc
	}

	if method, ok := raw["method"].(string); ok {
		scheme.Method = method
		if schemeType == "http" {
			scheme.HTTPScheme = method
		}
	}

	if name, ok := raw["name"].(string); ok {
		// Keep SchemeName as the generated index-based name for DB uniqueness.
		// Store the raw v0.3 "name" field only for display/apiKey purposes.
		if schemeType == "apiKey" {
			scheme.APIKeyName = name
		}
	}

	return scheme, nil
}

// buildSecurityRequirements parses the top-level securityRequirements array
// from the A2A card. Each entry is an OR'd alternative.
func buildSecurityRequirements(card *fullCard) []model.Capability {
	var caps []model.Capability

	for _, reqData := range card.SecurityRequirements {
		var schemes map[string][]string
		if err := json.Unmarshal(reqData, &schemes); err != nil {
			slog.Warn("Failed to parse security requirement", "error", err)
			continue
		}

		req := &model.A2ASecurityRequirement{
			Schemes:  schemes,
			SkillRef: "",
		}

		caps = append(caps, req)
	}

	return caps
}

// parseSkillSecurityRequirements creates A2ASecurityRequirement capabilities
// with SkillRef set for per-skill auth requirements.
func parseSkillSecurityRequirements(skillName string, skill *fullSkill) []model.Capability {
	var caps []model.Capability

	for _, reqData := range skill.SecurityRequirements {
		var schemes map[string][]string
		if err := json.Unmarshal(reqData, &schemes); err != nil {
			slog.Warn("Failed to parse skill security requirement", "skill", skillName, "error", err)
			continue
		}

		req := &model.A2ASecurityRequirement{
			Schemes:  schemes,
			SkillRef: skillName,
		}

		caps = append(caps, req)
	}

	return caps
}
