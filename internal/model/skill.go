package model

// Skill represents a capability exposed by an agent.
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"input_modes,omitempty"`
	OutputModes []string `json:"output_modes,omitempty"`
}
