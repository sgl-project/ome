package github

import (
	"context"
	"fmt"
	"net/http"

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

	// Set common GitHub headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return nil
}

// Refresh refreshes the credentials
func (c *GitHubCredentials) Refresh(ctx context.Context) error {
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

// GetTokenSource returns the underlying token source
func (c *GitHubCredentials) GetTokenSource() oauth2.TokenSource {
	return c.tokenSource
}

// newStaticTokenSource creates a new static token source
func newStaticTokenSource(token string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
	})
}
