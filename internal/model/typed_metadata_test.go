package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMeta is a test-only TypedMetadata implementation.
type stubMeta struct {
	Value string `json:"value"`
}

func (s *stubMeta) Kind() string    { return "test.stub" }
func (s *stubMeta) Validate() error { return nil }

func TestMarshalTypedMetadata(t *testing.T) {
	// Register the stub type for this test.
	RegisterTypedMeta("test.stub", func() TypedMetadata { return &stubMeta{} })
	t.Cleanup(func() { delete(typedMetaRegistry, "test.stub") })

	original := []TypedMetadata{
		&stubMeta{Value: "hello"},
		&stubMeta{Value: "world"},
	}

	data, err := MarshalTypedMetaJSON(original)
	require.NoError(t, err)

	// Verify the JSON contains kind discriminator.
	var raw []json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Len(t, raw, 2)

	// Round-trip: unmarshal back.
	result, err := UnmarshalTypedMetaJSON(data)
	require.NoError(t, err)
	require.Len(t, result, 2)

	for i, item := range result {
		stub, ok := item.(*stubMeta)
		require.True(t, ok, "expected *stubMeta at index %d", i)
		assert.Equal(t, "test.stub", stub.Kind())
		assert.Equal(t, original[i].(*stubMeta).Value, stub.Value)
	}
}

func TestUnmarshalTypedMetadata_EmptyArray(t *testing.T) {
	result, err := UnmarshalTypedMetaJSON([]byte(`[]`))
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCatalogEntry_SyncTypedMeta_RoundTrip(t *testing.T) {
	entry := &CatalogEntry{
		ID:          "test-1",
		DisplayName: "Test",
		Protocol:    ProtocolA2A,
		SpecVersion: "1.0",
		TypedMeta: []TypedMetadata{
			&A2AExtension{URI: "urn:test", Required: true},
			&A2AInterface{URL: "https://example.com", Binding: "jsonrpc"},
		},
	}
	entry.SyncToDB()

	assert.NotEqual(t, "[]", entry.TypedMetaJSON)
	assert.Equal(t, "1.0", entry.SpecVersion)

	restored := &CatalogEntry{
		SpecVersion:   entry.SpecVersion,
		TypedMetaJSON: entry.TypedMetaJSON,
	}
	restored.SyncFromDB()

	require.Len(t, restored.TypedMeta, 2)
	ext, ok := restored.TypedMeta[0].(*A2AExtension)
	require.True(t, ok)
	assert.Equal(t, "urn:test", ext.URI)
}

func TestUnmarshalTypedMetadata_UnknownKind(t *testing.T) {
	input := `[{"kind":"unknown.type","foo":"bar"},{"kind":"also.unknown"}]`
	result, err := UnmarshalTypedMetaJSON([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, result)
}
