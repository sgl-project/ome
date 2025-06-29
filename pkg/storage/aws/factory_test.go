package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestNewFactory(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)

	if factory == nil {
		t.Error("Expected factory instance but got nil")
	}
	if factory.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestFactory_Create(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)

	tests := []struct {
		name        string
		config      interface{}
		credentials auth.Credentials
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid with Config struct",
			config: &Config{
				Region:      "us-east-1",
				PartSize:    10 * 1024 * 1024,
				Concurrency: 20,
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "us-east-1",
			},
			expectError: false,
		},
		{
			name: "Valid with map config",
			config: map[string]interface{}{
				"region":           "us-west-2",
				"endpoint":         "http://localhost:9000",
				"force_path_style": true,
				"disable_ssl":      true,
				"part_size":        int64(5 * 1024 * 1024),
				"concurrency":      15,
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "",
			},
			expectError: false,
		},
		{
			name:   "Valid with nil config",
			config: nil,
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "us-east-1",
			},
			expectError: false,
		},
		{
			name:   "Invalid config type",
			config: "invalid-config",
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "us-east-1",
			},
			expectError: true,
			errorMsg:    "invalid config type",
		},
		{
			name: "Invalid credentials provider",
			config: &Config{
				Region: "us-east-1",
			},
			credentials: &mockCredentials{
				provider: auth.ProviderOCI,
			},
			expectError: true,
			errorMsg:    "invalid credentials provider",
		},
		{
			name: "Invalid credentials type for New",
			config: &Config{
				Region: "us-east-1",
			},
			credentials: &mockCredentials{
				provider: auth.ProviderAWS, // Right provider but wrong type
			},
			expectError: true,
			errorMsg:    "invalid credentials type",
		},
		{
			name: "Map config with partial values",
			config: map[string]interface{}{
				"region": "eu-west-1",
				// Other values should get defaults
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "",
			},
			expectError: false,
		},
		{
			name: "Map config with wrong types ignored",
			config: map[string]interface{}{
				"region":      "us-east-1",
				"part_size":   "invalid", // Wrong type, should be ignored
				"concurrency": "invalid", // Wrong type, should be ignored
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "",
			},
			expectError: false,
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
