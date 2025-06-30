package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// GCPCredentials implements auth.Credentials for GCP
type GCPCredentials struct {
	tokenSource oauth2.TokenSource
	authType    auth.AuthType
	projectID   string
	logger      logging.Interface

	// Mutex protects cached token
	mu          sync.RWMutex
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
	if c.tokenSource == nil {
		return "", fmt.Errorf("token source is not initialized")
	}

	token, err := c.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	c.mu.Lock()
	c.cachedToken = token
	c.mu.Unlock()

	return token.AccessToken, nil
}

// SignRequest signs an HTTP request with GCP credentials
func (c *GCPCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	if c.tokenSource == nil {
		return fmt.Errorf("token source is not initialized")
	}

	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// Cache the token for consistency with Token() method
	c.mu.Lock()
	c.cachedToken = token
	c.mu.Unlock()

	token.SetAuthHeader(req)
	return nil
}

// Refresh refreshes the credentials
func (c *GCPCredentials) Refresh(ctx context.Context) error {
	if c.tokenSource == nil {
		return fmt.Errorf("token source is not initialized")
	}

	// OAuth2 token sources handle refresh automatically
	// Force a new token to be fetched
	token, err := c.tokenSource.Token()
	if err != nil {
		return err
	}

	// Update cached token
	c.mu.Lock()
	c.cachedToken = token
	c.mu.Unlock()

	return nil
}

// IsExpired checks if the credentials are expired
func (c *GCPCredentials) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

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
// This is specifically for GKE Workload Identity
type WorkloadIdentityConfig struct {
	// ProjectID is the GCP project ID
	ProjectID string `json:"project_id"`

	// ServiceAccount is the GCP service account email to impersonate
	// Format: <name>@<project>.iam.gserviceaccount.com
	ServiceAccount string `json:"service_account,omitempty"`

	// KubernetesServiceAccount is the Kubernetes service account
	// Format: <namespace>/<name>
	KubernetesServiceAccount string `json:"kubernetes_service_account,omitempty"`

	// ClusterName is the GKE cluster name (optional)
	ClusterName string `json:"cluster_name,omitempty"`

	// ClusterLocation is the GKE cluster location (optional)
	ClusterLocation string `json:"cluster_location,omitempty"`
}

// Validate validates the workload identity configuration
func (c *WorkloadIdentityConfig) Validate() error {
	// For GKE Workload Identity, we just need project ID
	// The rest is handled by the metadata service
	if c.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	return nil
}

// GetClientOption returns a client option for use with Google APIs
func GetClientOption(creds *GCPCredentials) option.ClientOption {
	return option.WithTokenSource(creds.tokenSource)
}
