// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"fmt"
)

// Capability is the interface for protocol-specific features such as A2A skills
// or MCP tools. Each implementation must return a unique Kind string and validate
// its own fields.
type Capability interface {
	Kind() string
	Validate() error
}

// capabilityRegistry maps kind strings to factory functions for deserialization.
var capabilityRegistry = map[string]func() Capability{}

// kindWrapper is used to peek at the "kind" discriminator in JSON.
type kindWrapper struct {
	Kind string `json:"kind"`
}

// RegisterCapability registers a factory for the given kind so that
// UnmarshalCapabilitiesJSON can deserialize it.
func RegisterCapability(kind string, factory func() Capability) {
	capabilityRegistry[kind] = factory
}

// GetCapabilityFactory returns the factory function for the given kind.
// The second return value is false if the kind is not registered.
func GetCapabilityFactory(kind string) (func() Capability, bool) {
	f, ok := capabilityRegistry[kind]
	return f, ok
}

// MarshalCapabilitiesJSON serializes a slice of Capability to a JSON array,
// injecting a "kind" discriminator into each object.
// Empty slices marshal as "[]" rather than null.
func MarshalCapabilitiesJSON(items []Capability) ([]byte, error) {
	results := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshal capability: %w", err)
		}
		// Unmarshal into a generic map so we can inject "kind".
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("re-parse capability: %w", err)
		}
		m["kind"] = item.Kind()
		enriched, err := json.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("marshal enriched capability: %w", err)
		}
		results = append(results, enriched)
	}
	return json.Marshal(results)
}

// UnmarshalCapabilitiesJSON deserializes a JSON array into a slice of Capability.
// Unknown kinds are silently skipped.
func UnmarshalCapabilitiesJSON(data []byte) ([]Capability, error) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, fmt.Errorf("unmarshal capabilities array: %w", err)
	}

	result := make([]Capability, 0, len(rawItems))
	for _, raw := range rawItems {
		var w kindWrapper
		if err := json.Unmarshal(raw, &w); err != nil {
			continue // skip items without parseable kind
		}
		factory, ok := capabilityRegistry[w.Kind]
		if !ok {
			continue // silently skip unknown kinds
		}
		item := factory()
		if err := json.Unmarshal(raw, item); err != nil {
			return nil, fmt.Errorf("unmarshal kind %q: %w", w.Kind, err)
		}
		result = append(result, item)
	}
	return result, nil
}
