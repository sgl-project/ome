package oci

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// mockConfigProvider is a mock implementation of common.ConfigurationProvider
type mockConfigProvider struct {
	region string
}

func (m *mockConfigProvider) TenancyOCID() (string, error) {
	return "ocid1.tenancy.oc1..example", nil
}

func (m *mockConfigProvider) UserOCID() (string, error) {
	return "ocid1.user.oc1..example", nil
}

func (m *mockConfigProvider) KeyFingerprint() (string, error) {
	return "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99", nil
}

func (m *mockConfigProvider) Region() (string, error) {
	if m.region != "" {
		return m.region, nil
	}
	return "us-ashburn-1", nil
}

func (m *mockConfigProvider) KeyID() (string, error) {
	return "ocid1.tenancy.oc1..example/ocid1.user.oc1..example/aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99", nil
}

func (m *mockConfigProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	// For testing, we would need to generate a proper RSA key
	// This is a simplified mock
	return nil, nil
}

func (m *mockConfigProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{
		AuthType:         common.UnknownAuthenticationType,
		IsFromConfigFile: false,
		OboToken:         nil,
	}, nil
}

func TestOCICredentials_Provider(t *testing.T) {
	creds := &OCICredentials{
		configProvider: &mockConfigProvider{},
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	if provider := creds.Provider(); provider != auth.ProviderOCI {
		t.Errorf("Expected provider %s, got %s", auth.ProviderOCI, provider)
	}
}

func TestOCICredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Instance Principal",
			authType: auth.OCIInstancePrincipal,
		},
		{
			name:     "User Principal",
			authType: auth.OCIUserPrincipal,
		},
		{
			name:     "Resource Principal",
			authType: auth.OCIResourcePrincipal,
		},
		{
			name:     "OKE Workload Identity",
			authType: auth.OCIOkeWorkloadIdentity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: &mockConfigProvider{},
				authType:       tt.authType,
				logger:         logging.NewNopLogger(),
			}

			if authType := creds.Type(); authType != tt.authType {
				t.Errorf("Expected auth type %s, got %s", tt.authType, authType)
			}
		})
	}
}

func TestOCICredentials_Token(t *testing.T) {
	creds := &OCICredentials{
		configProvider: &mockConfigProvider{},
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	ctx := context.Background()
	token, err := creds.Token(ctx)

	// OCI uses request signing, not bearer tokens, so this should return empty
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if token != "" {
		t.Errorf("Expected empty token for OCI, got %s", token)
	}
}

func TestOCICredentials_IsExpired(t *testing.T) {
	creds := &OCICredentials{
		configProvider: &mockConfigProvider{},
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	// OCI SDK handles expiration internally, so this should always return false
	if creds.IsExpired() {
		t.Error("Expected IsExpired to return false")
	}
}

func TestOCICredentials_GetRegion(t *testing.T) {
	tests := []struct {
		name           string
		configRegion   string
		credsRegion    string
		expectedRegion string
	}{
		{
			name:           "Region from credentials",
			configRegion:   "us-phoenix-1",
			credsRegion:    "us-ashburn-1",
			expectedRegion: "us-ashburn-1",
		},
		{
			name:           "Region from config provider",
			configRegion:   "us-phoenix-1",
			credsRegion:    "",
			expectedRegion: "us-phoenix-1",
		},
		{
			name:           "No region",
			configRegion:   "",
			credsRegion:    "",
			expectedRegion: "us-ashburn-1", // Mock returns default region
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: &mockConfigProvider{region: tt.configRegion},
				authType:       auth.OCIInstancePrincipal,
				region:         tt.credsRegion,
				logger:         logging.NewNopLogger(),
			}

			region := creds.GetRegion()
			if region != tt.expectedRegion {
				t.Errorf("Expected region %s, got %s", tt.expectedRegion, region)
			}
		})
	}
}

func TestOCICredentials_GetConfigurationProvider(t *testing.T) {
	mockProvider := &mockConfigProvider{}
	creds := &OCICredentials{
		configProvider: mockProvider,
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	provider := creds.GetConfigurationProvider()
	// Can't directly compare interfaces, so check a method
	region1, _ := provider.Region()
	region2, _ := mockProvider.Region()
	if region1 != region2 {
		t.Error("Expected same configuration provider")
	}
}

func TestNewOCIHTTPClient(t *testing.T) {
	creds := &OCICredentials{
		configProvider: &mockConfigProvider{},
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	client := NewOCIHTTPClient(creds)

	if client == nil {
		t.Fatal("Expected non-nil HTTP client")
	}

	if client.credentials != creds {
		t.Error("Expected same credentials in HTTP client")
	}

	if client.client == nil {
		t.Error("Expected non-nil underlying HTTP client")
	}
}
