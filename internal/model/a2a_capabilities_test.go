package model

import (
	"encoding/json"
	"testing"
)

func TestA2ASecurityScheme_OAuthFlows(t *testing.T) {
	scheme := &A2ASecurityScheme{
		SchemeName:  "oauth2Auth",
		Type:        "oauth2",
		Description: "OAuth 2.0 authentication",
		OAuthFlows: []A2AOAuthFlow{
			{
				FlowType:         "authorizationCode",
				AuthorizationURL: "https://auth.example.com/authorize",
				TokenURL:         "https://auth.example.com/token",
				Scopes: map[string]string{
					"read":  "Read access",
					"write": "Write access",
				},
				Deprecated: false,
			},
		},
	}

	if scheme.Kind() != "a2a.security_scheme" {
		t.Errorf("Expected kind 'a2a.security_scheme', got '%s'", scheme.Kind())
	}
	if err := scheme.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}

	data, err := json.Marshal(scheme)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var decoded A2ASecurityScheme
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if decoded.SchemeName != scheme.SchemeName {
		t.Errorf("Expected SchemeName '%s', got '%s'", scheme.SchemeName, decoded.SchemeName)
	}
	if len(decoded.OAuthFlows) != 1 {
		t.Errorf("Expected 1 OAuth flow, got %d", len(decoded.OAuthFlows))
	}
	if decoded.OAuthFlows[0].FlowType != "authorizationCode" {
		t.Errorf("Expected flowType 'authorizationCode', got '%s'", decoded.OAuthFlows[0].FlowType)
	}
}

func TestA2ASecurityScheme_APIKey(t *testing.T) {
	scheme := &A2ASecurityScheme{
		SchemeName:     "apiKeyAuth",
		Type:           "apiKey",
		APIKeyLocation: "header",
		APIKeyName:     "X-API-Key",
		Description:    "API Key in header",
	}
	if err := scheme.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
	data, err := json.Marshal(scheme)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var decoded A2ASecurityScheme
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if decoded.APIKeyLocation != "header" {
		t.Errorf("Expected location 'header', got '%s'", decoded.APIKeyLocation)
	}
	if decoded.APIKeyName != "X-API-Key" {
		t.Errorf("Expected name 'X-API-Key', got '%s'", decoded.APIKeyName)
	}
}

func TestA2ASecurityScheme_HTTP(t *testing.T) {
	scheme := &A2ASecurityScheme{
		SchemeName:   "httpAuth",
		Type:         "http",
		HTTPScheme:   "Bearer",
		BearerFormat: "JWT",
		Description:  "Bearer JWT",
	}
	if err := scheme.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
	data, err := json.Marshal(scheme)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var decoded A2ASecurityScheme
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if decoded.HTTPScheme != "Bearer" {
		t.Errorf("Expected scheme 'Bearer', got '%s'", decoded.HTTPScheme)
	}
	if decoded.BearerFormat != "JWT" {
		t.Errorf("Expected format 'JWT', got '%s'", decoded.BearerFormat)
	}
}

func TestA2ASecurityScheme_Validate_EmptyType(t *testing.T) {
	scheme := &A2ASecurityScheme{
		SchemeName: "test",
		Type:       "",
	}
	err := scheme.Validate()
	if err == nil {
		t.Error("Expected validation error for empty type")
	}
}

func TestA2ASecurityScheme_Validate_EmptySchemeName(t *testing.T) {
	scheme := &A2ASecurityScheme{
		SchemeName: "",
		Type:       "http",
	}
	err := scheme.Validate()
	if err == nil {
		t.Error("Expected validation error for empty scheme_name")
	}
}

func TestA2ASecurityRequirement_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *A2ASecurityRequirement
		wantErr bool
	}{
		{
			name: "valid requirement",
			req: &A2ASecurityRequirement{
				Schemes: map[string][]string{
					"oauth2Auth": {"read", "write"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid requirement with empty scopes",
			req: &A2ASecurityRequirement{
				Schemes: map[string][]string{
					"apiKeyAuth": {},
				},
			},
			wantErr: false,
		},
		{
			name: "valid per-skill requirement",
			req: &A2ASecurityRequirement{
				Schemes: map[string][]string{
					"apiKeyAuth": {},
				},
				SkillRef: "createDocument",
			},
			wantErr: false,
		},
		{
			name: "invalid empty schemes",
			req: &A2ASecurityRequirement{
				Schemes: map[string][]string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestA2ASecurityRequirement_Kind(t *testing.T) {
	req := &A2ASecurityRequirement{
		Schemes: map[string][]string{
			"oauth2Auth": {"read"},
		},
	}
	if req.Kind() != "a2a.security_requirement" {
		t.Errorf("Expected kind 'a2a.security_requirement', got '%s'", req.Kind())
	}
}

func TestA2ASecurityRequirement_JSONRoundTrip(t *testing.T) {
	req := &A2ASecurityRequirement{
		Schemes: map[string][]string{
			"oauth2Auth": {"read", "write"},
			"apiKeyAuth": {},
		},
		SkillRef: "createDocument",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded A2ASecurityRequirement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(decoded.Schemes) != 2 {
		t.Errorf("Expected 2 schemes, got %d", len(decoded.Schemes))
	}
	if decoded.SkillRef != "createDocument" {
		t.Errorf("Expected SkillRef 'createDocument', got '%s'", decoded.SkillRef)
	}
}
