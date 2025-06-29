package auth

import (
	"context"
	"net/http"
)

// Provider represents an authentication provider type
type Provider string

const (
	ProviderOCI    Provider = "oci"
	ProviderAWS    Provider = "aws"
	ProviderGCP    Provider = "gcp"
	ProviderAzure  Provider = "azure"
	ProviderGitHub Provider = "github"
)

// AuthType represents the type of authentication mechanism
type AuthType string

// Common auth types across providers
const (
	// OCI auth types
	OCIUserPrincipal       AuthType = "OCIUserPrincipal"
	OCIInstancePrincipal   AuthType = "OCIInstancePrincipal"
	OCIResourcePrincipal   AuthType = "OCIResourcePrincipal"
	OCIOkeWorkloadIdentity AuthType = "OCIOkeWorkloadIdentity"

	// AWS auth types
	AWSAccessKey       AuthType = "AWSAccessKey"
	AWSInstanceProfile AuthType = "AWSInstanceProfile"
	AWSAssumeRole      AuthType = "AWSAssumeRole"
	AWSWebIdentity     AuthType = "AWSWebIdentity"
	AWSDefault         AuthType = "AWSDefault"

	// GCP auth types
	GCPServiceAccount     AuthType = "GCPServiceAccount"
	GCPApplicationDefault AuthType = "GCPApplicationDefault"
	GCPWorkloadIdentity   AuthType = "GCPWorkloadIdentity"
	GCPDefault            AuthType = "GCPDefault"

	// Azure auth types
	AzureServicePrincipal  AuthType = "AzureServicePrincipal"
	AzureManagedIdentity   AuthType = "AzureManagedIdentity"
	AzureDeviceFlow        AuthType = "AzureDeviceFlow"
	AzureClientSecret      AuthType = "AzureClientSecret"
	AzureClientCertificate AuthType = "AzureClientCertificate"
	AzureDefault           AuthType = "AzureDefault"
	AzureAccountKey        AuthType = "AzureAccountKey"

	// GitHub auth types
	GitHubToken               AuthType = "GitHubToken"
	GitHubApp                 AuthType = "GitHubApp"
	GitHubPersonalAccessToken AuthType = "GitHubPersonalAccessToken"
	GitHubOAuth               AuthType = "GitHubOAuth"
)

// Credentials represents authentication credentials
type Credentials interface {
	// Provider returns the provider type
	Provider() Provider

	// Type returns the authentication type
	Type() AuthType

	// Token returns an access token if applicable
	Token(ctx context.Context) (string, error)

	// SignRequest signs an HTTP request with appropriate auth headers
	SignRequest(ctx context.Context, req *http.Request) error

	// Refresh refreshes the credentials if needed
	Refresh(ctx context.Context) error

	// IsExpired checks if credentials are expired
	IsExpired() bool
}

// Config represents a base configuration for authentication
type Config struct {
	Provider Provider               `json:"provider" validate:"required"`
	AuthType AuthType               `json:"auth_type" validate:"required"`
	Region   string                 `json:"region,omitempty"`
	Extra    map[string]interface{} `json:"extra,omitempty"`
	// Fallback configuration to use if primary fails
	Fallback *Config `json:"fallback,omitempty"`
}

// Factory creates authentication providers
type Factory interface {
	// Create creates credentials for the given provider and config
	Create(ctx context.Context, config Config) (Credentials, error)

	// SupportedProviders returns list of supported providers
	SupportedProviders() []Provider

	// SupportedAuthTypes returns supported auth types for a provider
	SupportedAuthTypes(provider Provider) []AuthType
}

// TokenProvider provides tokens for authentication
type TokenProvider interface {
	// GetToken returns a valid token
	GetToken(ctx context.Context) (string, error)

	// RefreshToken refreshes and returns a new token
	RefreshToken(ctx context.Context) (string, error)
}

// CredentialsProvider provides credentials for a specific use case
type CredentialsProvider interface {
	// GetCredentials returns credentials for the given context
	GetCredentials(ctx context.Context) (Credentials, error)
}

// ChainProvider tries multiple credential providers in sequence
type ChainProvider struct {
	Providers []CredentialsProvider
}

// GetCredentials tries each provider until one succeeds
func (c *ChainProvider) GetCredentials(ctx context.Context) (Credentials, error) {
	var lastErr error
	for _, provider := range c.Providers {
		creds, err := provider.GetCredentials(ctx)
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// HTTPTransport wraps http.RoundTripper with authentication
type HTTPTransport struct {
	Base        http.RoundTripper
	Credentials Credentials
}

// RoundTrip implements http.RoundTripper
func (t *HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Base == nil {
		t.Base = http.DefaultTransport
	}

	// Clone the request to avoid modifying the original
	r := req.Clone(req.Context())

	// Sign the request
	if err := t.Credentials.SignRequest(req.Context(), r); err != nil {
		return nil, err
	}

	return t.Base.RoundTrip(r)
}
