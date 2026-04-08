package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchetype_AgentTypeHasNoCatalogFields(t *testing.T) {
	catalogOnlyFields := []string{
		"DisplayName", "Description", "Categories",
		"Status", "Source", "Metadata", "Validity",
	}
	agentTypeT := reflect.TypeOf(AgentType{})
	for _, name := range catalogOnlyFields {
		_, found := agentTypeT.FieldByName(name)
		assert.False(t, found, "AgentType must not have catalog field %q — it belongs on CatalogEntry", name)
	}
}

func TestArchetype_CatalogEntryHasNoProductFields(t *testing.T) {
	productOnlyFields := []string{
		"Protocol", "Endpoint",
		"Capabilities", "Skills", "TypedMeta", "SpecVersion",
	}
	entryT := reflect.TypeOf(CatalogEntry{})
	for _, name := range productOnlyFields {
		_, found := entryT.FieldByName(name)
		assert.False(t, found, "CatalogEntry must not have product field %q — it belongs on AgentType", name)
	}
}

func TestArchetype_CatalogEntryReferencesAgentType(t *testing.T) {
	entryT := reflect.TypeOf(CatalogEntry{})
	field, found := entryT.FieldByName("AgentTypeID")
	assert.True(t, found, "CatalogEntry must have AgentTypeID field")
	if found {
		assert.Equal(t, reflect.String, field.Type.Kind(), "AgentTypeID must be a string")
	}
	atField, found := entryT.FieldByName("AgentType")
	assert.True(t, found, "CatalogEntry must have AgentType field")
	if found {
		assert.Equal(t, reflect.Ptr, atField.Type.Kind(), "AgentType must be a pointer")
	}
}

func TestArchetype_AgentTypeHasRequiredFields(t *testing.T) {
	required := []string{"ID", "AgentKey", "Protocol", "Endpoint", "CreatedOn"}
	agentTypeT := reflect.TypeOf(AgentType{})
	for _, name := range required {
		_, found := agentTypeT.FieldByName(name)
		assert.True(t, found, "AgentType must have required field %q", name)
	}
}

func TestArchetype_AllCapabilityTypesImplementInterface(t *testing.T) {
	capType := reflect.TypeOf((*Capability)(nil)).Elem()
	types := []any{
		&A2ASkill{}, &A2AInterface{}, &A2ASecurityScheme{},
		&A2AExtension{}, &A2ASignature{},
		&MCPTool{}, &MCPResource{}, &MCPPrompt{},
	}
	for _, impl := range types {
		implType := reflect.TypeOf(impl)
		assert.True(t, implType.Implements(capType),
			"%s must implement Capability interface", implType.Elem().Name())
	}
}

func TestArchetype_CapabilityKindsAreRegistered(t *testing.T) {
	expectedKinds := []string{
		"a2a.skill", "a2a.interface", "a2a.security_scheme",
		"a2a.extension", "a2a.signature",
		"mcp.tool", "mcp.resource", "mcp.prompt",
	}
	for _, kind := range expectedKinds {
		_, ok := capabilityRegistry[kind]
		assert.True(t, ok, "capability kind %q must be registered", kind)
	}
}
