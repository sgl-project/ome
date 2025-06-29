package gcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_Register(t *testing.T) {
	// Factory doesn't have a Provider method
	// This test is removed as it's not applicable
}

func TestFactory_Create(t *testing.T) {
	ctx := context.Background()
	factory := &Factory{
		logger: logging.NewNopLogger(),
	}

	tests := []struct {
		name        string
		config      interface{}
		credentials auth.Credentials
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: &Config{
				ProjectID:    "test-project",
				StorageClass: "STANDARD",
			},
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: false,
		},
		{
			name:   "Nil config",
			config: nil,
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: false,
		},
		{
			name: "JSON config",
			config: json.RawMessage(`{
				"project_id": "test-project",
				"storage_class": "NEARLINE"
			}`),
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: false,
		},
		{
			name: "Map config",
			config: map[string]interface{}{
				"project_id":    "test-project",
				"storage_class": "COLDLINE",
				"chunk_size":    32,
				"enable_crc32c": false,
			},
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: false,
		},
		{
			name:   "Invalid config type",
			config: "invalid",
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: true,
			errorMsg:    "invalid config type",
		},
		{
			name:   "Invalid JSON",
			config: json.RawMessage(`{invalid json`),
			credentials: &testGCPCredentials{
				tokenSource: &mockTokenSource{token: "test-token"},
			},
			expectError: true,
			errorMsg:    "failed to unmarshal",
		},
		{
			name:   "Invalid credentials",
			config: nil,
			credentials: &mockCredentials{
				provider: auth.ProviderAWS,
			},
			expectError: true,
			errorMsg:    "invalid credentials provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := factory.Create(ctx, tt.config, tt.credentials)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if storage == nil {
					t.Error("Expected storage instance but got nil")
				}
			}
		})
	}
}
