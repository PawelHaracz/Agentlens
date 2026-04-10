package model

import (
	"errors"
	"fmt"
)

func init() {
	RegisterCapability("a2a.skill", func() Capability { return &A2ASkill{} }, true)
	RegisterCapability("a2a.interface", func() Capability { return &A2AInterface{} }, false)
	RegisterCapability("a2a.security_scheme", func() Capability { return &A2ASecurityScheme{} }, false)
	RegisterCapability("a2a.extension", func() Capability { return &A2AExtension{} }, false)
	RegisterCapability("a2a.signature", func() Capability { return &A2ASignature{} }, false)
}

// A2ASkill represents an A2A protocol skill capability.
// kind: "a2a.skill"
type A2ASkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// Kind returns the capability kind identifier.
func (s *A2ASkill) Kind() string { return "a2a.skill" }

// Validate checks that required fields are present.
func (s *A2ASkill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("a2a.skill: name is required")
	}
	return nil
}

// A2AExtension represents an A2A protocol extension capability.
// kind: "a2a.extension"
type A2AExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

func (a *A2AExtension) Kind() string { return "a2a.extension" }
func (a *A2AExtension) Validate() error {
	if a.URI == "" {
		return errors.New("a2a.extension: uri is required")
	}
	return nil
}

// A2ASecurityScheme represents a security scheme for A2A communication.
// kind: "a2a.security_scheme"
type A2ASecurityScheme struct {
	Type   string `json:"type"`
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
}

func (a *A2ASecurityScheme) Kind() string { return "a2a.security_scheme" }
func (a *A2ASecurityScheme) Validate() error {
	if a.Type == "" {
		return errors.New("a2a.security_scheme: type is required")
	}
	return nil
}

// A2AInterface represents an A2A agent interface binding capability.
// kind: "a2a.interface"
type A2AInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

func (a *A2AInterface) Kind() string { return "a2a.interface" }
func (a *A2AInterface) Validate() error {
	if a.URL == "" {
		return errors.New("a2a.interface: url is required")
	}
	return nil
}

// A2ASignature represents a cryptographic signature configuration capability.
// kind: "a2a.signature"
type A2ASignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

func (a *A2ASignature) Kind() string { return "a2a.signature" }
func (a *A2ASignature) Validate() error {
	if a.Algorithm == "" {
		return errors.New("a2a.signature: algorithm is required")
	}
	return nil
}
