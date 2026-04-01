// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"fmt"
)

// TypedMetadata is the interface for extensible, discriminated metadata entries.
// Each implementation must return a unique Kind string and validate its own fields.
type TypedMetadata interface {
	Kind() string
	Validate() error
}

// typedMetaRegistry maps kind strings to factory functions for deserialization.
var typedMetaRegistry = map[string]func() TypedMetadata{}

// RegisterTypedMeta registers a factory for the given kind so that
// UnmarshalTypedMetaJSON can deserialize it.
func RegisterTypedMeta(kind string, factory func() TypedMetadata) {
	typedMetaRegistry[kind] = factory
}

// kindWrapper is used to inject/peek at the "kind" discriminator in JSON.
type kindWrapper struct {
	Kind string `json:"kind"`
}

// MarshalTypedMetaJSON serializes a slice of TypedMetadata to a JSON array,
// injecting a "kind" discriminator into each object.
func MarshalTypedMetaJSON(items []TypedMetadata) ([]byte, error) {
	results := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshal typed metadata: %w", err)
		}
		// Unmarshal into a generic map so we can inject "kind".
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("re-parse typed metadata: %w", err)
		}
		m["kind"] = item.Kind()
		enriched, err := json.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("marshal enriched metadata: %w", err)
		}
		results = append(results, enriched)
	}
	return json.Marshal(results)
}

// UnmarshalTypedMetaJSON deserializes a JSON array into a slice of TypedMetadata.
// Unknown kinds are silently skipped.
func UnmarshalTypedMetaJSON(data []byte) ([]TypedMetadata, error) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, fmt.Errorf("unmarshal typed metadata array: %w", err)
	}

	result := make([]TypedMetadata, 0, len(rawItems))
	for _, raw := range rawItems {
		var w kindWrapper
		if err := json.Unmarshal(raw, &w); err != nil {
			continue // skip items without parseable kind
		}
		factory, ok := typedMetaRegistry[w.Kind]
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