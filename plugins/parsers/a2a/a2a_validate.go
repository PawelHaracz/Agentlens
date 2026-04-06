package a2a

import "github.com/PawelHaracz/agentlens/internal/kernel"

// Validate validates a raw JSON A2A agent card and returns a structured result
// that satisfies the kernel.ParserPlugin interface.
func (p *Plugin) Validate(raw []byte) kernel.ValidationResult {
	vr := ValidateCard(raw)
	// Convert to kernel-level types.
	errs := make([]kernel.ValidationError, len(vr.Errors))
	for i, e := range vr.Errors {
		errs[i] = kernel.ValidationError{Field: e.Field, Message: e.Message}
	}
	var preview map[string]any
	if vr.Preview != nil {
		preview = map[string]any{
			"display_name":     vr.Preview.DisplayName,
			"description":      vr.Preview.Description,
			"protocol":         vr.Preview.Protocol,
			"spec_version":     vr.Preview.SpecVersion,
			"skills_count":     vr.Preview.SkillsCount,
			"extensions_count": vr.Preview.ExtensionsCount,
			"security_schemes": vr.Preview.SecuritySchemes,
			"interfaces":       vr.Preview.Interfaces,
		}
	}
	return kernel.ValidationResult{
		Valid:       vr.Valid,
		SpecVersion: vr.SpecVersion,
		Errors:      errs,
		Warnings:    vr.Warnings,
		Preview:     preview,
	}
}
