package a2a

import "encoding/json"

// ValidationError represents a single field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationPreview summarises a valid card for display.
type ValidationPreview struct {
	DisplayName     string   `json:"display_name"`
	Description     string   `json:"description"`
	Protocol        string   `json:"protocol"`
	SpecVersion     string   `json:"spec_version"`
	SkillsCount     int      `json:"skills_count"`
	ExtensionsCount int      `json:"extensions_count"`
	SecuritySchemes []string `json:"security_schemes"`
	Interfaces      []string `json:"interfaces"`
}

// ValidationResult is the output of ValidateCard.
type ValidationResult struct {
	Valid       bool               `json:"valid"`
	SpecVersion string             `json:"spec_version"`
	Errors      []ValidationError  `json:"errors"`
	Warnings    []string           `json:"warnings"`
	Preview     *ValidationPreview `json:"preview,omitempty"`
}

// fullCard represents the complete A2A agent card structure across spec versions.
type fullCard struct {
	Name                      string            `json:"name"`
	Description               string            `json:"description"`
	URL                       string            `json:"url"`
	Version                   string            `json:"version"`
	Provider                  *a2aProvider      `json:"provider,omitempty"`
	Skills                    []fullSkill       `json:"skills,omitempty"`
	SupportedInterfaces       []fullInterface   `json:"supportedInterfaces,omitempty"`
	SecuritySchemes           json.RawMessage   `json:"securitySchemes,omitempty"` // array (v0.3) OR map (v1.0)
	SecurityRequirements      []json.RawMessage `json:"securityRequirements,omitempty"`
	Signatures                []fullSignature   `json:"signatures,omitempty"`
	SupportsExtendedAgentCard *bool             `json:"supportsExtendedAgentCard,omitempty"`
	Capabilities              *capabilities     `json:"capabilities,omitempty"`
	StateTransitionHistory    *bool             `json:"stateTransitionHistory,omitempty"`
}

type fullSkill struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Tags                 []string          `json:"tags,omitempty"`
	InputModes           []string          `json:"inputModes,omitempty"`
	OutputModes          []string          `json:"outputModes,omitempty"`
	SecurityRequirements []json.RawMessage `json:"securityRequirements,omitempty"`
}

type fullInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

type fullSignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

type capabilities struct {
	Extensions                []capExtension `json:"extensions,omitempty"`
	SupportsExtendedAgentCard *bool          `json:"supportsExtendedAgentCard,omitempty"`
}

type capExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

// detectSpecVersion determines the A2A spec version from the card structure.
func detectSpecVersion(card *fullCard) string {
	// Root-level flag indicates v0.3
	if card.SupportsExtendedAgentCard != nil {
		return "0.3"
	}
	// Flag nested inside capabilities indicates v1.0
	if card.Capabilities != nil && card.Capabilities.SupportsExtendedAgentCard != nil {
		return "1.0"
	}
	return ""
}

// ValidateCard validates a raw JSON A2A agent card and returns structured results.
func ValidateCard(raw []byte) ValidationResult {
	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return ValidationResult{
			Valid:    false,
			Errors:   []ValidationError{{Field: "json", Message: "invalid JSON: " + err.Error()}},
			Warnings: []string{},
		}
	}

	specVersion := detectSpecVersion(&card)
	errs := validateRequiredFields(&card)
	errs = append(errs, validateSkills(&card)...)
	warnings := collectWarnings(&card, specVersion)

	if len(errs) > 0 {
		return ValidationResult{
			Valid:       false,
			SpecVersion: specVersion,
			Errors:      errs,
			Warnings:    warnings,
		}
	}

	preview := buildPreview(&card, specVersion)

	return ValidationResult{
		Valid:       true,
		SpecVersion: specVersion,
		Errors:      []ValidationError{},
		Warnings:    warnings,
		Preview:     preview,
	}
}

// validateRequiredFields checks that all mandatory top-level card fields are
// present and returns any field-level errors found.
func validateRequiredFields(card *fullCard) []ValidationError {
	var errs []ValidationError

	if card.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "name is required"})
	}
	if card.Description == "" {
		errs = append(errs, ValidationError{Field: "description", Message: "description is required"})
	}
	if card.Version == "" {
		errs = append(errs, ValidationError{Field: "version", Message: "version is required"})
	}
	if len(card.SupportedInterfaces) == 0 && card.URL == "" {
		errs = append(errs, ValidationError{Field: "url", Message: "url or supportedInterfaces is required"})
	}

	// Validate extensions (inside capabilities).
	if card.Capabilities != nil {
		for i, ext := range card.Capabilities.Extensions {
			prefix := "capabilities.extensions[" + itoa(i) + "]"
			if ext.URI == "" {
				errs = append(errs, ValidationError{Field: prefix + ".uri", Message: "extension uri is required"})
			}
		}
	}

	return errs
}

// validateSkills checks each skill entry for required fields and returns any
// field-level errors found.
func validateSkills(card *fullCard) []ValidationError {
	var errs []ValidationError

	for i, s := range card.Skills {
		prefix := "skills[" + itoa(i) + "]"
		if s.ID == "" {
			errs = append(errs, ValidationError{Field: prefix + ".id", Message: "skill id is required"})
		}
		if s.Name == "" {
			errs = append(errs, ValidationError{Field: prefix + ".name", Message: "skill name is required"})
		}
		if s.Description == "" {
			errs = append(errs, ValidationError{Field: prefix + ".description", Message: "skill description is required"})
		}
	}

	return errs
}

// collectWarnings returns deprecation and informational warnings for the card.
func collectWarnings(card *fullCard, specVersion string) []string {
	var warnings []string

	if card.StateTransitionHistory != nil && (specVersion == "1.0" || specVersion == "") {
		warnings = append(warnings, "stateTransitionHistory is deprecated in v1.0")
	}

	return warnings
}

// buildPreview constructs a ValidationPreview summary from a valid card.
func buildPreview(card *fullCard, specVersion string) *ValidationPreview {
	schemeNames := buildSecurityPreviewNames(card.SecuritySchemes)

	ifaceBindings := make([]string, 0, len(card.SupportedInterfaces))
	for _, iface := range card.SupportedInterfaces {
		if iface.Binding != "" {
			ifaceBindings = append(ifaceBindings, iface.Binding)
		} else {
			ifaceBindings = append(ifaceBindings, iface.URL)
		}
	}

	extCount := 0
	if card.Capabilities != nil {
		extCount = len(card.Capabilities.Extensions)
	}

	return &ValidationPreview{
		DisplayName:     card.Name,
		Description:     card.Description,
		Protocol:        "a2a",
		SpecVersion:     specVersion,
		SkillsCount:     len(card.Skills),
		ExtensionsCount: extCount,
		SecuritySchemes: schemeNames,
		Interfaces:      ifaceBindings,
	}
}

// buildSecurityPreviewNames extracts scheme names/types from the raw
// securitySchemes value, which may be a v1.0 named map or a v0.3 array.
func buildSecurityPreviewNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try v1.0 map format first.
	var v10 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v10); err == nil {
		names := make([]string, 0, len(v10))
		for name := range v10 {
			names = append(names, name)
		}
		return names
	}

	// Try v0.3 array format.
	var v03 []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &v03); err == nil {
		names := make([]string, 0, len(v03))
		for _, s := range v03 {
			names = append(names, s.Type)
		}
		return names
	}

	return nil
}

// itoa is a simple int-to-string helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
