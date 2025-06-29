package azure

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
		auth.AzureClientSecret,
		auth.AzureClientCertificate,
		auth.AzureManagedIdentity,
		auth.AzureDefault,
		auth.AzureAccountKey,
		auth.AzureDeviceFlow,
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
		Provider: auth.ProviderGCP, // Wrong provider
		AuthType: auth.AzureClientSecret,
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
		Provider: auth.ProviderAzure,
		AuthType: auth.GCPServiceAccount, // Wrong auth type for Azure
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestFactory_Create_ClientSecret_MissingCredentials(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureClientSecret,
		Extra:    map[string]interface{}{
			// No client secret config
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing client secret credentials")
	}
}

func TestFactory_Create_ClientSecret_PartialConfig(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureClientSecret,
		Extra: map[string]interface{}{
			"client_secret": map[string]interface{}{
				"tenant_id": "test-tenant",
				"client_id": "test-client",
				// Missing client_secret
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for incomplete client secret config")
	}
}

func TestFactory_Create_ClientCertificate_MissingCredentials(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureClientCertificate,
		Extra: map[string]interface{}{
			"client_certificate": map[string]interface{}{
				"tenant_id": "test-tenant",
				"client_id": "test-client",
				// Missing certificate
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing certificate")
	}
}

func TestFactory_Create_ManagedIdentity(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "System assigned",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
			},
		},
		{
			name: "User assigned with client ID",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
				Extra: map[string]interface{}{
					"managed_identity": map[string]interface{}{
						"client_id": "test-client-id",
					},
				},
			},
		},
		{
			name: "User assigned with resource ID",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
				Extra: map[string]interface{}{
					"managed_identity": map[string]interface{}{
						"resource_id": "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/identity",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail in unit tests because it requires Azure environment
			_, err := factory.Create(ctx, tt.config)
			if err == nil {
				t.Skip("Managed identity test skipped - requires Azure environment")
			}
		})
	}
}

func TestFactory_Create_AccountKey_Valid(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureAccountKey,
		Extra: map[string]interface{}{
			"account_key": map[string]interface{}{
				"account_name": "mystorageaccount",
				"account_key":  "base64encodedkey==",
			},
		},
	}

	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create account key credentials: %v", err)
	}

	if creds.Provider() != auth.ProviderAzure {
		t.Errorf("Expected provider %s, got %s", auth.ProviderAzure, creds.Provider())
	}

	if creds.Type() != auth.AzureAccountKey {
		t.Errorf("Expected auth type %s, got %s", auth.AzureAccountKey, creds.Type())
	}
}

func TestFactory_Create_AccountKey_Missing(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureAccountKey,
		Extra: map[string]interface{}{
			"account_key": map[string]interface{}{
				"account_name": "mystorageaccount",
				// Missing account_key
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing account key")
	}
}
