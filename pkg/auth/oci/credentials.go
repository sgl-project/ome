package oci

import (
	"context"
	"net/http"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// OCICredentials implements auth.Credentials for OCI
type OCICredentials struct {
	configProvider common.ConfigurationProvider
	authType       auth.AuthType
	region         string
	logger         logging.Interface
}

// Provider returns the provider type
func (c *OCICredentials) Provider() auth.Provider {
	return auth.ProviderOCI
}

// Type returns the authentication type
func (c *OCICredentials) Type() auth.AuthType {
	return c.authType
}

// Token returns an access token if applicable
func (c *OCICredentials) Token(ctx context.Context) (string, error) {
	// OCI uses request signing, not bearer tokens
	return "", nil
}

// SignRequest signs an HTTP request with OCI auth headers
func (c *OCICredentials) SignRequest(ctx context.Context, req *http.Request) error {
	// Create a default request signer
	signer := common.DefaultRequestSigner(c.configProvider)

	// Sign the request
	return signer.Sign(req)
}

// Refresh refreshes the credentials if needed
func (c *OCICredentials) Refresh(ctx context.Context) error {
	// OCI SDK handles credential refresh internally
	return nil
}

// IsExpired checks if credentials are expired
func (c *OCICredentials) IsExpired() bool {
	// OCI SDK handles expiration internally
	return false
}

// GetConfigurationProvider returns the underlying OCI configuration provider
func (c *OCICredentials) GetConfigurationProvider() common.ConfigurationProvider {
	return c.configProvider
}

// GetRegion returns the configured region
func (c *OCICredentials) GetRegion() string {
	if c.region != "" {
		return c.region
	}

	// Try to get region from config provider
	region, err := c.configProvider.Region()
	if err != nil {
		// Log the error for visibility instead of silently ignoring it
		c.logger.WithError(err).Warn("Failed to get region from configuration provider")
		return ""
	}

	return region
}

// OCIHTTPClient creates an HTTP client with OCI authentication
type OCIHTTPClient struct {
	client      *http.Client
	credentials *OCICredentials
}

// NewOCIHTTPClient creates a new HTTP client with OCI auth
func NewOCIHTTPClient(credentials *OCICredentials) *OCIHTTPClient {
	return &OCIHTTPClient{
		client: &http.Client{
			Timeout: 2 * time.Minute, // Reduced from 20 minutes to prevent resource exhaustion
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 200,
				MaxConnsPerHost:     200,
			},
		},
		credentials: credentials,
	}
}

// Do executes an HTTP request with OCI authentication
func (c *OCIHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Sign the request
	if err := c.credentials.SignRequest(req.Context(), req); err != nil {
		return nil, err
	}

	return c.client.Do(req)
}
