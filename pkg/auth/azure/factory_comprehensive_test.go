package azure

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Test factory creation
func TestNewFactory_Comprehensive(t *testing.T) {
	tests := []struct {
		name   string
		logger logging.Interface
	}{
		{
			name:   "With logger",
			logger: logging.NewNopLogger(),
		},
		{
			name:   "With nil logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.logger)
			if factory == nil {
				t.Fatal("Expected non-nil factory")
			}
			if factory.logger != tt.logger {
				t.Error("Logger not properly set")
			}
		})
	}
}

// Test createDefaultCredential
func TestFactory_CreateDefaultCredential(t *testing.T) {
	// Save current env vars
	oldTenantID := os.Getenv("AZURE_TENANT_ID")
	oldClientID := os.Getenv("AZURE_CLIENT_ID")
	defer func() {
		os.Setenv("AZURE_TENANT_ID", oldTenantID)
		os.Setenv("AZURE_CLIENT_ID", oldClientID)
	}()

	// Set test env vars
	os.Setenv("AZURE_TENANT_ID", "test-tenant-id")
	os.Setenv("AZURE_CLIENT_ID", "test-client-id")

	factory := NewFactory(logging.NewNopLogger())
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureDefault,
	}

	_, err := factory.Create(ctx, config)
	// This will fail in non-Azure environment but tests the path
	if err == nil {
		t.Log("Default credential created successfully")
	} else {
		t.Logf("Default credential creation failed as expected: %v", err)
	}
}

// Test createDeviceFlowCredential
func TestFactory_CreateDeviceFlowCredential(t *testing.T) {
	factory := NewFactory(logging.NewNopLogger())
	ctx := context.Background()

	tests := []struct {
		name    string
		config  auth.Config
		wantErr bool
	}{
		{
			name: "Device flow from config",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureDeviceFlow,
				Extra: map[string]interface{}{
					"device_flow": map[string]interface{}{
						"tenant_id": "test-tenant",
						"client_id": "test-client",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Device flow missing tenant ID",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureDeviceFlow,
				Extra: map[string]interface{}{
					"device_flow": map[string]interface{}{
						"client_id": "test-client",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Device flow missing client ID",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureDeviceFlow,
				Extra: map[string]interface{}{
					"device_flow": map[string]interface{}{
						"tenant_id": "test-tenant",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				// Device flow will succeed in creating but may fail to authenticate
				if err != nil {
					t.Logf("Device flow credential creation: %v", err)
				}
				if err == nil && creds != nil {
					if creds.Type() != auth.AzureDeviceFlow {
						t.Errorf("Expected auth type %s, got %s", auth.AzureDeviceFlow, creds.Type())
					}
				}
			}
		})
	}
}

// Test edge cases for factory
func TestFactory_Create_EdgeCases(t *testing.T) {
	factory := NewFactory(logging.NewNopLogger())
	ctx := context.Background()

	tests := []struct {
		name    string
		config  auth.Config
		wantErr bool
	}{
		{
			name: "Invalid client_secret type",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientSecret,
				Extra: map[string]interface{}{
					"client_secret": 123, // Not a map
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid client_certificate type",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientCertificate,
				Extra: map[string]interface{}{
					"client_certificate": "not a map",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid managed_identity type",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
				Extra: map[string]interface{}{
					"managed_identity": true, // Not a map
				},
			},
			// Managed identity has defaults, so it won't error
			wantErr: false,
		},
		{
			name: "Invalid account_key type",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureAccountKey,
				Extra: map[string]interface{}{
					"account_key": []string{"invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid device_flow type",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureDeviceFlow,
				Extra: map[string]interface{}{
					"device_flow": "not a map",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test environment variable handling
func TestFactory_EnvironmentVariables(t *testing.T) {
	// Save current env vars
	envVars := []string{
		"AZURE_TENANT_ID",
		"AZURE_CLIENT_ID",
		"AZURE_CLIENT_SECRET",
		"AZURE_STORAGE_ACCOUNT",
		"AZURE_STORAGE_KEY",
	}

	oldValues := make(map[string]string)
	for _, env := range envVars {
		oldValues[env] = os.Getenv(env)
		defer os.Setenv(env, oldValues[env])
	}

	// Test client secret with env vars
	t.Run("Client secret from environment", func(t *testing.T) {
		os.Setenv("AZURE_TENANT_ID", "env-tenant")
		os.Setenv("AZURE_CLIENT_ID", "env-client")
		os.Setenv("AZURE_CLIENT_SECRET", "env-secret")

		factory := NewFactory(logging.NewNopLogger())
		config := auth.Config{
			Provider: auth.ProviderAzure,
			AuthType: auth.AzureClientSecret,
		}

		creds, err := factory.Create(context.Background(), config)
		if err != nil {
			t.Logf("Create with env vars error: %v", err)
		}
		if creds != nil {
			azCreds := creds.(*AzureCredentials)
			if azCreds.GetTenantID() != "env-tenant" {
				t.Errorf("Expected tenant ID from env, got %s", azCreds.GetTenantID())
			}
		}
	})

	// Test account key with env vars
	t.Run("Account key from environment", func(t *testing.T) {
		os.Setenv("AZURE_STORAGE_ACCOUNT", "envstorageaccount")
		os.Setenv("AZURE_STORAGE_KEY", "env-storage-key")

		factory := NewFactory(logging.NewNopLogger())
		config := auth.Config{
			Provider: auth.ProviderAzure,
			AuthType: auth.AzureAccountKey,
		}

		creds, err := factory.Create(context.Background(), config)
		if err != nil {
			t.Errorf("Create with storage env vars error: %v", err)
		}
		if creds != nil && creds.Type() != auth.AzureAccountKey {
			t.Errorf("Expected auth type %s, got %s", auth.AzureAccountKey, creds.Type())
		}
	})
}

// Test managed identity with different configurations
func TestFactory_ManagedIdentity_Configurations(t *testing.T) {
	factory := NewFactory(logging.NewNopLogger())
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "System-assigned managed identity",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
			},
		},
		{
			name: "User-assigned with client ID",
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
			name: "User-assigned with resource ID",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureManagedIdentity,
				Extra: map[string]interface{}{
					"managed_identity": map[string]interface{}{
						"resource_id": "/subscriptions/test/resourceGroups/test/providers/Microsoft.ManagedIdentity/userAssignedIdentities/test",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)
			// This will fail in non-Azure environment
			if err == nil && creds != nil {
				if creds.Type() != auth.AzureManagedIdentity {
					t.Errorf("Expected auth type %s, got %s", auth.AzureManagedIdentity, creds.Type())
				}
			}
		})
	}
}

// Test certificate loading
func TestFactory_ClientCertificate_Loading(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a dummy certificate file (not a valid cert, just for path testing)
	certPath := filepath.Join(tmpDir, "test-cert.pfx")
	os.WriteFile(certPath, []byte("dummy certificate data"), 0600)

	factory := NewFactory(logging.NewNopLogger())
	ctx := context.Background()

	tests := []struct {
		name    string
		config  auth.Config
		wantErr bool
	}{
		{
			name: "Certificate from path",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientCertificate,
				Extra: map[string]interface{}{
					"client_certificate": map[string]interface{}{
						"tenant_id":        "test-tenant",
						"client_id":        "test-client",
						"certificate_path": certPath,
					},
				},
			},
			// Will fail because it's not a valid certificate
			wantErr: true,
		},
		{
			name: "Certificate from data",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientCertificate,
				Extra: map[string]interface{}{
					"client_certificate": map[string]interface{}{
						"tenant_id":        "test-tenant",
						"client_id":        "test-client",
						"certificate_data": []byte("dummy certificate data"),
					},
				},
			},
			// Will fail because it's not a valid certificate
			wantErr: true,
		},
		{
			name: "Certificate missing path and data",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientCertificate,
				Extra: map[string]interface{}{
					"client_certificate": map[string]interface{}{
						"tenant_id": "test-tenant",
						"client_id": "test-client",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test factory with nil logger
func TestFactory_NilLogger(t *testing.T) {
	factory := NewFactory(nil)

	// Should not panic
	authTypes := factory.SupportedAuthTypes()

	// Should have 6 auth types now (including device flow)
	if len(authTypes) != 6 {
		t.Errorf("Expected 6 auth types, got %d", len(authTypes))
	}

	// Test create with invalid provider
	config := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: auth.AzureClientSecret,
	}

	_, err := factory.Create(context.Background(), config)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}
