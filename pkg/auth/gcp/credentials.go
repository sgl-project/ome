package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// GCPCredentials implements auth.Credentials for GCP
type GCPCredentials struct {
	tokenSource oauth2.TokenSource
	authType    auth.AuthType
	projectID   string
	logger      logging.Interface
	cachedToken *oauth2.Token
}

// Provider returns the provider type
func (c *GCPCredentials) Provider() auth.Provider {
	return auth.ProviderGCP
}

// Type returns the authentication type
func (c *GCPCredentials) Type() auth.AuthType {
	return c.authType
}

// Token retrieves the GCP access token
func (c *GCPCredentials) Token(ctx context.Context) (string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	c.cachedToken = token
	return token.AccessToken, nil
}

// SignRequest signs an HTTP request with GCP credentials
func (c *GCPCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	token.SetAuthHeader(req)
	return nil
}

// Refresh refreshes the credentials
func (c *GCPCredentials) Refresh(ctx context.Context) error {
	// OAuth2 token sources handle refresh automatically
	// Force a new token to be fetched
	_, err := c.tokenSource.Token()
	return err
}

// IsExpired checks if the credentials are expired
func (c *GCPCredentials) IsExpired() bool {
	if c.cachedToken == nil {
		return true
	}
	return !c.cachedToken.Valid()
}

// GetTokenSource returns the underlying token source
func (c *GCPCredentials) GetTokenSource() oauth2.TokenSource {
	return c.tokenSource
}

// GetProjectID returns the GCP project ID
func (c *GCPCredentials) GetProjectID() string {
	return c.projectID
}

// ServiceAccountConfig represents GCP service account configuration
type ServiceAccountConfig struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// Validate validates the service account configuration
func (c *ServiceAccountConfig) Validate() error {
	if c.Type != "service_account" {
		return fmt.Errorf("invalid service account type: %s", c.Type)
	}
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("private_key is required")
	}
	if c.ClientEmail == "" {
		return fmt.Errorf("client_email is required")
	}
	return nil
}

// ToJSON converts the service account config to JSON
func (c *ServiceAccountConfig) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// WorkloadIdentityConfig represents GCP workload identity configuration
type WorkloadIdentityConfig struct {
	ProjectID        string `json:"project_id"`
	PoolID           string `json:"pool_id"`
	ProviderID       string `json:"provider_id"`
	ServiceAccount   string `json:"service_account"`
	CredentialSource string `json:"credential_source"`
}

// Validate validates the workload identity configuration
func (c *WorkloadIdentityConfig) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if c.PoolID == "" {
		return fmt.Errorf("pool_id is required")
	}
	if c.ProviderID == "" {
		return fmt.Errorf("provider_id is required")
	}
	return nil
}

// createTokenSource creates an OAuth2 token source from credentials
func createTokenSource(ctx context.Context, creds *google.Credentials, scopes []string) oauth2.TokenSource {
	if len(scopes) == 0 {
		scopes = []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/devstorage.full_control",
		}
	}
	return creds.TokenSource
}

// GetClientOption returns a client option for use with Google APIs
func GetClientOption(creds *GCPCredentials) option.ClientOption {
	return option.WithTokenSource(creds.tokenSource)
}
