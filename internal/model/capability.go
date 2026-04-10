// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Capability is the interface for protocol-specific features such as A2A skills
// or MCP tools. Each implementation must return a unique Kind string and validate
// its own fields.
type Capability interface {
	Kind() string
	Validate() error
}

// CapabilityMeta holds registration metadata for a capability kind.
type CapabilityMeta struct {
	Factory      func() Capability
	Discoverable bool // true = user-facing, shown in capability discovery UI
}

var capabilityRegistry = map[string]CapabilityMeta{}

// kindWrapper is used to peek at the "kind" discriminator in JSON.
type kindWrapper struct {
	Kind string `json:"kind"`
}

// RegisterCapability registers a capability kind with its factory and discoverability flag.
// This function is not concurrency-safe and must only be called from package init() functions.
func RegisterCapability(kind string, factory func() Capability, discoverable bool) {
	capabilityRegistry[kind] = CapabilityMeta{
		Factory:      factory,
		Discoverable: discoverable,
	}
}

// GetCapabilityFactory returns the factory for a given kind.
// Returns nil, false if the kind is not registered.
// Maintains backward compatibility with existing deserialization code.
func GetCapabilityFactory(kind string) (func() Capability, bool) {
	m, ok := capabilityRegistry[kind]
	return m.Factory, ok
}

// DiscoverableKinds returns the kind strings where Discoverable == true.
// Results are sorted for deterministic behavior.
func DiscoverableKinds() []string {
	var kinds []string
	for kind, meta := range capabilityRegistry {
		if meta.Discoverable {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return kinds
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

// UnmarshalCapabilitiesJSON parses a JSON array of capability objects into typed Capability values.
// Items with unknown kinds or malformed JSON are silently skipped.
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
		factory, ok := GetCapabilityFactory(w.Kind)
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
