package oci

import (
	"context"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.OCIUserPrincipal,
		auth.OCIInstancePrincipal,
		auth.OCIResourcePrincipal,
		auth.OCIOkeWorkloadIdentity,
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
		Provider: auth.ProviderAWS, // Wrong provider
		AuthType: auth.OCIInstancePrincipal,
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
		Provider: auth.ProviderOCI,
		AuthType: auth.AWSAccessKey, // Wrong auth type for OCI
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestUserPrincipalConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    UserPrincipalConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "DEFAULT",
			},
			wantError: false,
		},
		{
			name: "Missing config path",
			config: UserPrincipalConfig{
				Profile: "DEFAULT",
			},
			wantError: true,
		},
		{
			name: "Missing profile",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
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
