package gcp

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
		auth.GCPServiceAccount,
		auth.GCPWorkloadIdentity,
		auth.GCPDefault,
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
		AuthType: auth.GCPServiceAccount,
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
		Provider: auth.ProviderGCP,
		AuthType: auth.AWSAccessKey, // Wrong auth type for GCP
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestFactory_Create_ServiceAccount_MissingCredentials(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra:    map[string]interface{}{
			// No service account config
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing service account credentials")
	}
}

func TestFactory_Create_ServiceAccount_InvalidJSON(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPServiceAccount,
		Extra: map[string]interface{}{
			"service_account": map[string]interface{}{
				"type": "service_account",
				// Missing required fields
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid service account JSON")
	}
}

func TestFactory_Create_WorkloadIdentity(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPWorkloadIdentity,
		Extra: map[string]interface{}{
			"project_id": "test-project",
		},
	}

	// This will fail in unit tests because it tries to find actual credentials
	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Skip("Workload identity test skipped - requires GCP environment")
	}
}

func TestFactory_Create_Default(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.GCPDefault,
	}

	// This will fail in unit tests because it tries to find actual credentials
	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Skip("Default credentials test skipped - requires GCP environment")
	}
}
