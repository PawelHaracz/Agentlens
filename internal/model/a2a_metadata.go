package model

import "errors"

// A2AExtension represents an A2A protocol extension.
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

// A2AInterface represents an A2A agent interface binding.
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

// A2ASignature represents a cryptographic signature configuration.
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

func init() {
	RegisterTypedMeta("a2a.extension", func() TypedMetadata { return &A2AExtension{} })
	RegisterTypedMeta("a2a.security_scheme", func() TypedMetadata { return &A2ASecurityScheme{} })
	RegisterTypedMeta("a2a.interface", func() TypedMetadata { return &A2AInterface{} })
	RegisterTypedMeta("a2a.signature", func() TypedMetadata { return &A2ASignature{} })
}
