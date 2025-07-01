package azure

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.AzureClientSecret,
		auth.AzureClientCertificate,
		auth.AzureManagedIdentity,
		auth.AzureDeviceFlow,
		auth.AzureDefault,
		auth.AzureAccountKey,
		auth.AzurePodIdentity,
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAzure,
		AuthType: auth.AzureAccountKey,
		Extra: map[string]interface{}{
			"account_name": "mystorageaccount",
			"account_key":  "base64encodedkey==",
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
	logger := logging.ForZap(zaptest.NewLogger(t))
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

// Test factory creation
func TestNewFactory(t *testing.T) {
	tests := []struct {
		name   string
		logger logging.Interface
	}{
		{
			name:   "With logger",
			logger: logging.ForZap(zaptest.NewLogger(t)),
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

	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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
	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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
	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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

		factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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
		os.Setenv("AZURE_STORAGE_ACCOUNT_NAME", "envstorageaccount")
		os.Setenv("AZURE_STORAGE_ACCOUNT_KEY", "env-storage-key")

		factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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
	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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

	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
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

	// Should have 7 auth types now (including pod identity)
	if len(authTypes) != 7 {
		t.Errorf("Expected 7 auth types, got %d", len(authTypes))
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

// Test createPodIdentityCredential
func TestFactory_CreatePodIdentityCredential(t *testing.T) {
	// Save current env vars
	oldClientID := os.Getenv("AZURE_CLIENT_ID")
	oldTenantID := os.Getenv("AZURE_TENANT_ID")
	oldTokenFile := os.Getenv("AZURE_FEDERATED_TOKEN_FILE")
	oldIdentityEndpoint := os.Getenv("IDENTITY_ENDPOINT")
	oldIdentityHeader := os.Getenv("IDENTITY_HEADER")
	defer func() {
		os.Setenv("AZURE_CLIENT_ID", oldClientID)
		os.Setenv("AZURE_TENANT_ID", oldTenantID)
		os.Setenv("AZURE_FEDERATED_TOKEN_FILE", oldTokenFile)
		os.Setenv("IDENTITY_ENDPOINT", oldIdentityEndpoint)
		os.Setenv("IDENTITY_HEADER", oldIdentityHeader)
	}()

	factory := NewFactory(logging.ForZap(zaptest.NewLogger(t)))
	ctx := context.Background()

	tests := []struct {
		name    string
		config  auth.Config
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "Pod Identity with client ID from config",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
				Extra: map[string]interface{}{
					"client_id": "test-client-id",
				},
			},
			wantErr: false,
		},
		{
			name: "Pod Identity with resource ID from config",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
				Extra: map[string]interface{}{
					"resource_id": "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/mi",
				},
			},
			wantErr: false,
		},
		{
			name: "Pod Identity v1 with NMI endpoint",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
			},
			envVars: map[string]string{
				"IDENTITY_ENDPOINT": "http://169.254.169.254/metadata/identity/oauth2/token",
				"IDENTITY_HEADER":   "true",
				"AZURE_CLIENT_ID":   "test-client-id",
			},
			wantErr: false,
		},
		{
			name: "Workload Identity with federated token",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
			},
			envVars: map[string]string{
				"AZURE_CLIENT_ID":            "test-client-id",
				"AZURE_TENANT_ID":            "test-tenant-id",
				"AZURE_FEDERATED_TOKEN_FILE": "/var/run/secrets/tokens/azure-identity-token",
			},
			wantErr: false,
		},
		{
			name: "Pod Identity with no specific config (system-assigned)",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			os.Unsetenv("AZURE_CLIENT_ID")
			os.Unsetenv("AZURE_TENANT_ID")
			os.Unsetenv("AZURE_FEDERATED_TOKEN_FILE")
			os.Unsetenv("IDENTITY_ENDPOINT")
			os.Unsetenv("IDENTITY_HEADER")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			_, err := factory.Create(ctx, tt.config)
			// In non-AKS environment, this will fail but we're testing the factory logic
			if err != nil {
				t.Logf("Pod identity credential creation failed as expected: %v", err)
			} else {
				t.Log("Pod identity credential created successfully")
			}
		})
	}
}

// Test PodIdentityConfig validation
func TestPodIdentityConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    PodIdentityConfig
		wantError bool
	}{
		{
			name:      "Empty config (system-assigned identity)",
			config:    PodIdentityConfig{},
			wantError: false,
		},
		{
			name: "Config with client ID",
			config: PodIdentityConfig{
				ClientID: "test-client-id",
			},
			wantError: false,
		},
		{
			name: "Config with resource ID",
			config: PodIdentityConfig{
				ResourceID: "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/mi",
			},
			wantError: false,
		},
		{
			name: "Config with NMI endpoint",
			config: PodIdentityConfig{
				IdentityEndpoint: "http://169.254.169.254/metadata/identity/oauth2/token",
				IdentityHeader:   "true",
			},
			wantError: false,
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

func TestFactory_Create_WithScopes(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name         string
		config       auth.Config
		expectScopes []string
	}{
		{
			name: "Default storage scope when no scopes specified",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureAccountKey,
				Extra: map[string]interface{}{
					"account_key": map[string]interface{}{
						"account_name": "mystorageaccount",
						"account_key":  "base64encodedkey==",
					},
				},
			},
			expectScopes: nil, // Will default to storage scope in credentials
		},
		{
			name: "Custom scopes for Key Vault",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientSecret,
				Extra: map[string]interface{}{
					"client_secret": map[string]interface{}{
						"tenant_id":     "test-tenant",
						"client_id":     "test-client",
						"client_secret": "test-secret",
					},
					"scopes": []string{"https://vault.azure.net/.default"},
				},
			},
			expectScopes: []string{"https://vault.azure.net/.default"},
		},
		{
			name: "Multiple custom scopes",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientSecret,
				Extra: map[string]interface{}{
					"client_secret": map[string]interface{}{
						"tenant_id":     "test-tenant",
						"client_id":     "test-client",
						"client_secret": "test-secret",
					},
					"scopes": []string{
						"https://management.azure.com/.default",
						"https://graph.microsoft.com/.default",
					},
				},
			},
			expectScopes: []string{
				"https://management.azure.com/.default",
				"https://graph.microsoft.com/.default",
			},
		},
		{
			name: "Flat structure backward compatibility",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureAccountKey,
				Extra: map[string]interface{}{
					"account_name": "mystorageaccount",
					"account_key":  "base64encodedkey==",
				},
			},
			expectScopes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)
			if err != nil {
				// Some auth types may fail in test environment
				t.Logf("Credential creation failed (expected in test environment): %v", err)
				return
			}

			// Check if credentials were created
			if creds == nil {
				t.Fatal("Expected non-nil credentials")
			}

			// Verify the credentials are of the correct type
			azCreds, ok := creds.(*AzureCredentials)
			if !ok {
				t.Fatal("Expected AzureCredentials type")
			}

			// Check scopes
			if tt.expectScopes != nil {
				if len(azCreds.scopes) != len(tt.expectScopes) {
					t.Errorf("Expected %d scopes, got %d", len(tt.expectScopes), len(azCreds.scopes))
				} else {
					for i, scope := range tt.expectScopes {
						if azCreds.scopes[i] != scope {
							t.Errorf("Expected scope %s, got %s", scope, azCreds.scopes[i])
						}
					}
				}
			}
		})
	}
}

func TestFactory_Create_NestedVsFlatStructure(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "Nested structure (preferred)",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientSecret,
				Extra: map[string]interface{}{
					"client_secret": map[string]interface{}{
						"tenant_id":     "test-tenant",
						"client_id":     "test-client",
						"client_secret": "test-secret",
					},
				},
			},
		},
		{
			name: "Flat structure (backward compatibility)",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzureClientSecret,
				Extra: map[string]interface{}{
					"tenant_id":     "test-tenant",
					"client_id":     "test-client",
					"client_secret": "test-secret",
				},
			},
		},
		{
			name: "Pod Identity nested structure",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
				Extra: map[string]interface{}{
					"pod_identity": map[string]interface{}{
						"client_id": "test-client-id",
					},
				},
			},
		},
		{
			name: "Pod Identity flat structure",
			config: auth.Config{
				Provider: auth.ProviderAzure,
				AuthType: auth.AzurePodIdentity,
				Extra: map[string]interface{}{
					"client_id": "test-client-id",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(ctx, tt.config)
			// We expect these to fail in test environment but we're testing the config parsing
			if err != nil {
				t.Logf("Credential creation failed (expected in test environment): %v", err)
			}
		})
	}
}
