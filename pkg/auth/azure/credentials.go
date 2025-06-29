package azure

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// AzureCredentials implements auth.Credentials for Azure
type AzureCredentials struct {
	credential  azcore.TokenCredential
	authType    auth.AuthType
	tenantID    string
	clientID    string
	logger      logging.Interface
	cachedToken *azcore.AccessToken
}

// Provider returns the provider type
func (c *AzureCredentials) Provider() auth.Provider {
	return auth.ProviderAzure
}

// Type returns the authentication type
func (c *AzureCredentials) Type() auth.AuthType {
	return c.authType
}

// Token retrieves the Azure access token
func (c *AzureCredentials) Token(ctx context.Context) (string, error) {
	// Get token for Azure Storage scope
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	c.cachedToken = &token
	return token.Token, nil
}

// SignRequest signs an HTTP request with Azure credentials
func (c *AzureCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	// Get token
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Token))
	return nil
}

// Refresh refreshes the credentials
func (c *AzureCredentials) Refresh(ctx context.Context) error {
	// Azure SDK handles token refresh automatically
	// Force a new token to be fetched
	_, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	return err
}

// IsExpired checks if the credentials are expired
func (c *AzureCredentials) IsExpired() bool {
	if c.cachedToken == nil {
		return true
	}
	return time.Now().After(c.cachedToken.ExpiresOn)
}

// GetCredential returns the underlying Azure credential
func (c *AzureCredentials) GetCredential() azcore.TokenCredential {
	return c.credential
}

// GetTokenCredential returns the Azure token credential (same as GetCredential)
func (c *AzureCredentials) GetTokenCredential() azcore.TokenCredential {
	return c.credential
}

// GetTenantID returns the Azure tenant ID
func (c *AzureCredentials) GetTenantID() string {
	return c.tenantID
}

// GetClientID returns the Azure client ID
func (c *AzureCredentials) GetClientID() string {
	return c.clientID
}

// ClientSecretConfig represents Azure client secret configuration
type ClientSecretConfig struct {
	TenantID     string `mapstructure:"tenant_id" json:"tenant_id"`
	ClientID     string `mapstructure:"client_id" json:"client_id"`
	ClientSecret string `mapstructure:"client_secret" json:"client_secret"`
}

// Validate validates the client secret configuration
func (c *ClientSecretConfig) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}
	return nil
}

// ClientCertificateConfig represents Azure client certificate configuration
type ClientCertificateConfig struct {
	TenantID        string `mapstructure:"tenant_id" json:"tenant_id"`
	ClientID        string `mapstructure:"client_id" json:"client_id"`
	CertificatePath string `mapstructure:"certificate_path" json:"certificate_path"`
	CertificateData []byte `mapstructure:"certificate_data" json:"certificate_data"`
	Password        string `mapstructure:"password" json:"password"`
}

// Validate validates the client certificate configuration
func (c *ClientCertificateConfig) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.CertificatePath == "" && len(c.CertificateData) == 0 {
		return fmt.Errorf("either certificate_path or certificate_data is required")
	}
	return nil
}

// ManagedIdentityConfig represents Azure managed identity configuration
type ManagedIdentityConfig struct {
	ClientID   string `mapstructure:"client_id" json:"client_id,omitempty"`
	ResourceID string `mapstructure:"resource_id" json:"resource_id,omitempty"`
}

// Validate validates the managed identity configuration
func (c *ManagedIdentityConfig) Validate() error {
	// Both ClientID and ResourceID are optional for system-assigned managed identity
	return nil
}

// AccountKeyConfig represents Azure storage account key configuration
type AccountKeyConfig struct {
	AccountName string `mapstructure:"account_name" json:"account_name"`
	AccountKey  string `mapstructure:"account_key" json:"account_key"`
}

// Validate validates the account key configuration
func (c *AccountKeyConfig) Validate() error {
	if c.AccountName == "" {
		return fmt.Errorf("account_name is required")
	}
	if c.AccountKey == "" {
		return fmt.Errorf("account_key is required")
	}
	return nil
}

// SharedKeyCredential implements azcore.TokenCredential for account key auth
type SharedKeyCredential struct {
	accountName string
	accountKey  string
}

// GetToken returns a static token for shared key auth
func (s *SharedKeyCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// For shared key, we don't use OAuth tokens
	// The actual signing happens in the storage client
	return azcore.AccessToken{
		Token:     s.accountKey,
		ExpiresOn: time.Now().Add(24 * time.Hour), // Doesn't expire
	}, nil
}

// NewSharedKeyCredential creates a new shared key credential
func NewSharedKeyCredential(accountName, accountKey string) *SharedKeyCredential {
	return &SharedKeyCredential{
		accountName: accountName,
		accountKey:  accountKey,
	}
}

// DeviceFlowConfig represents Azure device flow configuration
type DeviceFlowConfig struct {
	TenantID string `mapstructure:"tenant_id" json:"tenant_id"`
	ClientID string `mapstructure:"client_id" json:"client_id"`
}

// Validate validates the device flow configuration
func (c *DeviceFlowConfig) Validate() error {
	if c.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	return nil
}
