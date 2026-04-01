package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestA2AMetadataKinds(t *testing.T) {
	tests := []struct {
		name     string
		meta     TypedMetadata
		wantKind string
	}{
		{"A2AExtension", &A2AExtension{}, "a2a.extension"},
		{"A2ASecurityScheme", &A2ASecurityScheme{}, "a2a.security_scheme"},
		{"A2AInterface", &A2AInterface{}, "a2a.interface"},
		{"A2ASignature", &A2ASignature{}, "a2a.signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantKind, tt.meta.Kind())
		})
	}
}

func TestA2AMetadataValidate(t *testing.T) {
	tests := []struct {
		name    string
		meta    TypedMetadata
		wantErr bool
	}{
		// A2AExtension
		{"Extension valid", &A2AExtension{URI: "urn:example:ext"}, false},
		{"Extension missing URI", &A2AExtension{}, true},

		// A2ASecurityScheme
		{"SecurityScheme valid", &A2ASecurityScheme{Type: "oauth2"}, false},
		{"SecurityScheme missing type", &A2ASecurityScheme{}, true},

		// A2AInterface
		{"Interface valid", &A2AInterface{URL: "https://example.com"}, false},
		{"Interface missing URL", &A2AInterface{}, true},

		// A2ASignature
		{"Signature valid", &A2ASignature{Algorithm: "Ed25519"}, false},
		{"Signature missing algorithm", &A2ASignature{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meta.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
