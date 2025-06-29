package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
)

// GitHubCredentials implements auth.Credentials for GitHub
type GitHubCredentials struct {
	tokenSource oauth2.TokenSource
	authType    auth.AuthType
	httpClient  *http.Client
	logger      logging.Interface
	cachedToken *oauth2.Token
}

// Provider returns the provider type
func (c *GitHubCredentials) Provider() auth.Provider {
	return auth.ProviderGitHub
}

// Type returns the authentication type
func (c *GitHubCredentials) Type() auth.AuthType {
	return c.authType
}

// Token retrieves the GitHub access token
func (c *GitHubCredentials) Token(ctx context.Context) (string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	c.cachedToken = token
	return token.AccessToken, nil
}

// SignRequest signs an HTTP request with GitHub credentials
func (c *GitHubCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// GitHub uses Bearer token authentication
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	// Also set token header for Git LFS
	req.Header.Set("X-GitHub-Token", token.AccessToken)

	return nil
}

// Refresh refreshes the credentials
func (c *GitHubCredentials) Refresh(ctx context.Context) error {
	// OAuth2 token sources handle refresh automatically
	// Force a new token to be fetched
	_, err := c.tokenSource.Token()
	return err
}

// IsExpired checks if the credentials are expired
func (c *GitHubCredentials) IsExpired() bool {
	if c.cachedToken == nil {
		return true
	}
	return !c.cachedToken.Valid()
}

// GetHTTPClient returns an HTTP client configured with GitHub auth
func (c *GitHubCredentials) GetHTTPClient() *http.Client {
	return c.httpClient
}

// PersonalAccessTokenConfig represents GitHub personal access token configuration
type PersonalAccessTokenConfig struct {
	Token string `mapstructure:"token" json:"token"`
}

// Validate validates the personal access token configuration
func (c *PersonalAccessTokenConfig) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}
	return nil
}

// GitHubAppConfig represents GitHub App configuration
type GitHubAppConfig struct {
	AppID          int64  `mapstructure:"app_id" json:"app_id"`
	InstallationID int64  `mapstructure:"installation_id" json:"installation_id"`
	PrivateKey     string `mapstructure:"private_key" json:"private_key"`
	PrivateKeyPath string `mapstructure:"private_key_path" json:"private_key_path"`
}

// Validate validates the GitHub App configuration
func (c *GitHubAppConfig) Validate() error {
	if c.AppID == 0 {
		return fmt.Errorf("app_id is required")
	}
	if c.InstallationID == 0 {
		return fmt.Errorf("installation_id is required")
	}
	if c.PrivateKey == "" && c.PrivateKeyPath == "" {
		return fmt.Errorf("either private_key or private_key_path is required")
	}
	return nil
}

// OAuthConfig represents GitHub OAuth configuration
type OAuthConfig struct {
	ClientID     string `mapstructure:"client_id" json:"client_id"`
	ClientSecret string `mapstructure:"client_secret" json:"client_secret"`
	AccessToken  string `mapstructure:"access_token" json:"access_token,omitempty"`
	RefreshToken string `mapstructure:"refresh_token" json:"refresh_token,omitempty"`
}

// Validate validates the OAuth configuration
func (c *OAuthConfig) Validate() error {
	if c.AccessToken == "" {
		if c.ClientID == "" || c.ClientSecret == "" {
			return fmt.Errorf("either access_token or (client_id and client_secret) is required")
		}
	}
	return nil
}

// StaticTokenSource implements oauth2.TokenSource for static tokens
type StaticTokenSource struct {
	token *oauth2.Token
}

// Token returns the static token
func (s *StaticTokenSource) Token() (*oauth2.Token, error) {
	return s.token, nil
}

// NewStaticTokenSource creates a new static token source
func NewStaticTokenSource(token string) oauth2.TokenSource {
	return &StaticTokenSource{
		token: &oauth2.Token{
			AccessToken: token,
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(365 * 24 * time.Hour), // PATs don't expire through OAuth
		},
	}
}
