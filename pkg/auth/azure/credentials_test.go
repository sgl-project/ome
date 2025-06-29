package azure

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestAzureCredentials_Provider(t *testing.T) {
	creds := &AzureCredentials{
		authType: auth.AzureClientSecret,
		tenantID: "test-tenant",
		clientID: "test-client",
		logger:   logging.NewNopLogger(),
	}

	if provider := creds.Provider(); provider != auth.ProviderAzure {
		t.Errorf("Expected provider %s, got %s", auth.ProviderAzure, provider)
	}
}

func TestAzureCredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Client Secret",
			authType: auth.AzureClientSecret,
		},
		{
			name:     "Client Certificate",
			authType: auth.AzureClientCertificate,
		},
		{
			name:     "Managed Identity",
			authType: auth.AzureManagedIdentity,
		},
		{
			name:     "Default",
			authType: auth.AzureDefault,
		},
		{
			name:     "Account Key",
			authType: auth.AzureAccountKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &AzureCredentials{
				authType: tt.authType,
			}
			if typ := creds.Type(); typ != tt.authType {
				t.Errorf("Expected type %s, got %s", tt.authType, typ)
			}
		})
	}
}

func TestAzureCredentials_GetTenantID(t *testing.T) {
	creds := &AzureCredentials{
		tenantID: "my-tenant-id",
	}

	if tenantID := creds.GetTenantID(); tenantID != "my-tenant-id" {
		t.Errorf("Expected tenant ID my-tenant-id, got %s", tenantID)
	}
}

func TestAzureCredentials_GetClientID(t *testing.T) {
	creds := &AzureCredentials{
		clientID: "my-client-id",
	}

	if clientID := creds.GetClientID(); clientID != "my-client-id" {
		t.Errorf("Expected client ID my-client-id, got %s", clientID)
	}
}

func TestAzureCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *AzureCredentials
		expected bool
	}{
		{
			name:     "No cached token",
			creds:    &AzureCredentials{},
			expected: true,
		},
		{
			name: "Valid token",
			creds: &AzureCredentials{
				cachedToken: &azcore.AccessToken{
					Token:     "test-token",
					ExpiresOn: time.Now().Add(1 * time.Hour),
				},
			},
			expected: false,
		},
		{
			name: "Expired token",
			creds: &AzureCredentials{
				cachedToken: &azcore.AccessToken{
					Token:     "test-token",
					ExpiresOn: time.Now().Add(-1 * time.Hour),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.creds.IsExpired()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestClientSecretConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientSecretConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: ClientSecretConfig{
				TenantID:     "test-tenant",
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantError: false,
		},
		{
			name: "Missing tenant ID",
			config: ClientSecretConfig{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			wantError: true,
		},
		{
			name: "Missing client ID",
			config: ClientSecretConfig{
				TenantID:     "test-tenant",
				ClientSecret: "test-secret",
			},
			wantError: true,
		},
		{
			name: "Missing client secret",
			config: ClientSecretConfig{
				TenantID: "test-tenant",
				ClientID: "test-client",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    ClientSecretConfig{},
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

func TestClientCertificateConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientCertificateConfig
		wantError bool
	}{
		{
			name: "Valid config with path",
			config: ClientCertificateConfig{
				TenantID:        "test-tenant",
				ClientID:        "test-client",
				CertificatePath: "/path/to/cert.pfx",
			},
			wantError: false,
		},
		{
			name: "Valid config with data",
			config: ClientCertificateConfig{
				TenantID:        "test-tenant",
				ClientID:        "test-client",
				CertificateData: []byte("cert-data"),
			},
			wantError: false,
		},
		{
			name: "Missing tenant ID",
			config: ClientCertificateConfig{
				ClientID:        "test-client",
				CertificatePath: "/path/to/cert.pfx",
			},
			wantError: true,
		},
		{
			name: "Missing client ID",
			config: ClientCertificateConfig{
				TenantID:        "test-tenant",
				CertificatePath: "/path/to/cert.pfx",
			},
			wantError: true,
		},
		{
			name: "Missing certificate",
			config: ClientCertificateConfig{
				TenantID: "test-tenant",
				ClientID: "test-client",
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

func TestManagedIdentityConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ManagedIdentityConfig
		wantError bool
	}{
		{
			name:      "Valid empty config (system-assigned)",
			config:    ManagedIdentityConfig{},
			wantError: false,
		},
		{
			name: "Valid config with client ID",
			config: ManagedIdentityConfig{
				ClientID: "test-client-id",
			},
			wantError: false,
		},
		{
			name: "Valid config with resource ID",
			config: ManagedIdentityConfig{
				ResourceID: "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/identity",
			},
			wantError: false,
		},
		{
			name: "Valid config with both IDs",
			config: ManagedIdentityConfig{
				ClientID:   "test-client-id",
				ResourceID: "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/identity",
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

func TestAccountKeyConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    AccountKeyConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: AccountKeyConfig{
				AccountName: "mystorageaccount",
				AccountKey:  "base64encodedkey==",
			},
			wantError: false,
		},
		{
			name: "Missing account name",
			config: AccountKeyConfig{
				AccountKey: "base64encodedkey==",
			},
			wantError: true,
		},
		{
			name: "Missing account key",
			config: AccountKeyConfig{
				AccountName: "mystorageaccount",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    AccountKeyConfig{},
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

func TestSharedKeyCredential_GetToken(t *testing.T) {
	cred := NewSharedKeyCredential("myaccount", "mykey")

	ctx := context.Background()
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{})

	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token.Token != "mykey" {
		t.Errorf("Expected token 'mykey', got %s", token.Token)
	}

	if time.Until(token.ExpiresOn) < 23*time.Hour {
		t.Error("Expected token to expire in ~24 hours")
	}
}
