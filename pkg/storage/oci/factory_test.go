package oci

import (
	"context"
	"net/http"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_Create_InvalidCredentials(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	// Mock credentials with wrong provider
	mockCreds := &mockCredentials{
		provider: auth.ProviderAWS, // Wrong provider
	}

	config := &Config{
		CompartmentID: "test-compartment",
		Region:        "us-ashburn-1",
	}

	_, err := factory.Create(ctx, config, mockCreds)
	if err == nil {
		t.Error("Expected error for invalid credentials provider")
	}
}

func TestFactory_Create_ConfigTypeConversion(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	// Mock OCI credentials
	mockCreds := &mockCredentials{
		provider: auth.ProviderOCI,
	}

	// Test with map config
	mapConfig := map[string]interface{}{
		"compartment_id":   "test-compartment",
		"region":           "us-ashburn-1",
		"enable_obo_token": true,
		"obo_token":        "test-token",
		"auth": auth.Config{
			Provider: auth.ProviderOCI,
			AuthType: auth.OCIInstancePrincipal,
		},
	}

	// This will fail because we're using mock credentials instead of real OCI credentials
	// but it should fail after config conversion, not during conversion
	_, err := factory.Create(ctx, mapConfig, mockCreds)
	if err == nil {
		t.Error("Expected error with mock credentials")
	}

	// The error should be about invalid credentials type, not config conversion
	if err.Error() == "invalid config type: expected *Config or map[string]interface{}" {
		t.Error("Config conversion failed when it should have succeeded")
	}
}

// Mock credentials for testing
type mockCredentials struct {
	provider auth.Provider
}

func (m *mockCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockCredentials) Type() auth.AuthType {
	return auth.OCIInstancePrincipal
}

func (m *mockCredentials) Token(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (m *mockCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockCredentials) IsExpired() bool {
	return false
}
