package model

import (
	"sort"
	"testing"
)

func TestDiscoverableKinds(t *testing.T) {
	kinds := DiscoverableKinds()

	// User-facing kinds that should be discoverable
	expected := []string{"a2a.skill", "mcp.prompt", "mcp.resource", "mcp.tool"}

	// Should return exactly the expected count
	if len(kinds) != len(expected) {
		t.Errorf("expected %d discoverable kinds, got %d: %v", len(expected), len(kinds), kinds)
	}

	// Should be sorted
	sortedKinds := make([]string, len(kinds))
	copy(sortedKinds, kinds)
	sort.Strings(sortedKinds)
	for i := range kinds {
		if kinds[i] != sortedKinds[i] {
			t.Errorf("kinds not sorted: got %v", kinds)
			break
		}
	}

	// Should contain all expected user-facing kinds
	for _, e := range expected {
		found := false
		for _, k := range kinds {
			if k == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected kind %s not found in %v", e, kinds)
		}
	}

	// Should NOT contain technical kinds
	technical := []string{"a2a.extension", "a2a.interface", "a2a.security_scheme", "a2a.signature"}
	for _, tk := range technical {
		for _, k := range kinds {
			if k == tk {
				t.Errorf("technical kind %s should not be discoverable", tk)
			}
		}
	}
}

func TestGetCapabilityFactoryStillWorks(t *testing.T) {
	// Backward compatibility: GetCapabilityFactory should work for all 8 kinds
	allKinds := []string{
		"a2a.skill", "a2a.interface", "a2a.security_scheme", "a2a.extension", "a2a.signature",
		"mcp.tool", "mcp.resource", "mcp.prompt",
	}

	for _, kind := range allKinds {
		factory, ok := GetCapabilityFactory(kind)
		if !ok {
			t.Errorf("GetCapabilityFactory(%s) returned not ok", kind)
		}
		if factory == nil {
			t.Errorf("GetCapabilityFactory(%s) returned nil factory", kind)
		}

		// Call factory to ensure it works
		cap := factory()
		if cap == nil {
			t.Errorf("factory for %s returned nil capability", kind)
		}
		if cap.Kind() != kind {
			t.Errorf("factory for %s returned capability with kind %s", kind, cap.Kind())
		}
	}
}
