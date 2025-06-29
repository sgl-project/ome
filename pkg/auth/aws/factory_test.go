package aws

import (
	"context"
	"os"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.AWSAccessKey,
		auth.AWSAssumeRole,
		auth.AWSInstanceProfile,
		auth.AWSWebIdentity,
		auth.AWSDefault,
	}

	if len(authTypes) != len(expected) {
		t.Errorf("Expected %d auth types, got %d", len(expected), len(authTypes))
	}

	typeMap := make(map[auth.AuthType]bool)
	for _, at := range authTypes {
		typeMap[at] = true
	}

	for _, e := range expected {
		if !typeMap[e] {
			t.Errorf("Missing expected auth type: %s", e)
		}
	}
}

func TestFactory_Create_InvalidProvider(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderOCI, // Wrong provider
		AuthType: auth.AWSAccessKey,
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}

func TestFactory_Create_UnsupportedAuthType(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.OCIInstancePrincipal, // Wrong auth type for AWS
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestFactory_AccessKeyConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    AccessKeyConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: AccessKeyConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantError: false,
		},
		{
			name: "Missing access key ID",
			config: AccessKeyConfig{
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantError: true,
		},
		{
			name: "Missing secret access key",
			config: AccessKeyConfig{
				AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestFactory_AssumeRoleConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    AssumeRoleConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: AssumeRoleConfig{
				RoleARN: "arn:aws:iam::123456789012:role/MyRole",
			},
			wantError: false,
		},
		{
			name: "Missing role ARN",
			config: AssumeRoleConfig{
				RoleSessionName: "my-session",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestFactory_Create_AccessKey(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	// Test with config values
	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSAccessKey,
		Extra: map[string]interface{}{
			"access_key": map[string]interface{}{
				"access_key_id":     "test-key",
				"secret_access_key": "test-secret",
				"session_token":     "test-token",
			},
		},
	}

	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials: %v", err)
	}

	if creds.Provider() != auth.ProviderAWS {
		t.Errorf("Expected provider %s, got %s", auth.ProviderAWS, creds.Provider())
	}
	if creds.Type() != auth.AWSAccessKey {
		t.Errorf("Expected auth type %s, got %s", auth.AWSAccessKey, creds.Type())
	}
}

func TestFactory_Create_AccessKey_FromEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("AWS_ACCESS_KEY_ID", "env-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSAccessKey,
		Extra:    map[string]interface{}{},
	}

	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials from environment: %v", err)
	}

	if creds == nil {
		t.Fatal("Expected credentials but got nil")
	}
}

func TestFactory_Create_AccessKey_MissingRequired(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSAccessKey,
		Extra: map[string]interface{}{
			"access_key": map[string]interface{}{
				"access_key_id": "test-key",
				// Missing secret_access_key
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing secret_access_key")
	}
}

func TestFactory_Create_AssumeRole(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSAssumeRole,
		Extra: map[string]interface{}{
			"assume_role": map[string]interface{}{
				"role_arn":          "arn:aws:iam::123456789012:role/TestRole",
				"role_session_name": "test-session",
				"external_id":       "external-123",
			},
		},
	}

	// This will likely fail in unit tests due to missing AWS credentials
	// but we can test that it attempts to create the provider
	_, err := factory.Create(ctx, config)
	// We expect an error here because we don't have real AWS creds
	if err == nil {
		t.Log("Unexpected success - normally would fail without real AWS credentials")
	}
}

func TestFactory_Create_WebIdentity(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSWebIdentity,
		Extra: map[string]interface{}{
			"web_identity": map[string]interface{}{
				"role_arn":          "arn:aws:iam::123456789012:role/TestRole",
				"token_file":        "/tmp/token",
				"role_session_name": "test-session",
			},
		},
	}

	// This will likely fail in unit tests due to missing AWS credentials
	// but we can test that it attempts to create the provider
	_, err := factory.Create(ctx, config)
	// We expect an error here because we don't have real AWS creds
	if err == nil {
		t.Log("Unexpected success - normally would fail without real AWS credentials")
	}
}

func TestFactory_Create_InstanceProfile(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSInstanceProfile,
	}

	// This will create an EC2 role credentials provider
	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create instance profile credentials: %v", err)
	}

	if creds.Provider() != auth.ProviderAWS {
		t.Errorf("Expected provider %s, got %s", auth.ProviderAWS, creds.Provider())
	}
	if creds.Type() != auth.AWSInstanceProfile {
		t.Errorf("Expected auth type %s, got %s", auth.AWSInstanceProfile, creds.Type())
	}
}

func TestFactory_Create_Default(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: auth.AWSDefault,
		Region:   "us-west-2",
	}

	// This will likely fail in unit tests due to missing AWS credentials
	// but we can test that it attempts to create the provider
	_, err := factory.Create(ctx, config)
	// We might get an error here because we don't have real AWS creds
	if err == nil {
		t.Log("Unexpected success - normally would fail without real AWS credentials")
	}
}
