package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
)

// Factory creates GitHub credentials
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new GitHub auth factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates GitHub credentials based on config
func (f *Factory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
	if config.Provider != auth.ProviderGitHub {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", auth.ProviderGitHub, config.Provider)
	}

	// Validate base URL if provided
	var baseURL string
	if config.Extra != nil {
		if url, ok := config.Extra["base_url"].(string); ok && url != "" {
			baseURL = url
			// Basic validation - ensure it's a valid URL format
			if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
				return nil, fmt.Errorf("invalid base_url: must start with http:// or https://")
			}
		}
	}

	var tokenSource oauth2.TokenSource
	var err error

	switch config.AuthType {
	case auth.GitHubPersonalAccessToken:
		tokenSource, err = f.createPersonalAccessTokenSource(config)
	case auth.GitHubApp:
		tokenSource, err = f.createGitHubAppTokenSource(ctx, config)
	case auth.GitHubOAuth:
		tokenSource, err = f.createOAuthTokenSource(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported GitHub auth type: %s", config.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub token source: %w", err)
	}

	// Create HTTP client with OAuth2 transport
	httpClient := oauth2.NewClient(ctx, tokenSource)

	return &GitHubCredentials{
		tokenSource: tokenSource,
		authType:    config.AuthType,
		httpClient:  httpClient,
		logger:      f.logger,
	}, nil
}

// SupportedAuthTypes returns supported GitHub auth types
func (f *Factory) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.GitHubPersonalAccessToken,
		auth.GitHubApp,
		auth.GitHubOAuth,
	}
}

// createPersonalAccessTokenSource creates a token source for PAT
func (f *Factory) createPersonalAccessTokenSource(config auth.Config) (oauth2.TokenSource, error) {
	patConfig := &PersonalAccessTokenConfig{}

	if config.Extra != nil {
		if pat, ok := config.Extra["personal_access_token"].(map[string]interface{}); ok {
			if token, ok := pat["token"].(string); ok {
				patConfig.Token = token
			}
		} else if token, ok := config.Extra["token"].(string); ok {
			patConfig.Token = token
		}
	}

	if patConfig.Token == "" {
		envConfig := LoadFromEnv()
		if envConfig.PersonalAccessToken != nil {
			patConfig = envConfig.PersonalAccessToken
		}
	}

	if err := patConfig.Validate(); err != nil {
		return nil, err
	}

	return newStaticTokenSource(patConfig.Token), nil
}

// createGitHubAppTokenSource creates a token source for GitHub App
func (f *Factory) createGitHubAppTokenSource(ctx context.Context, config auth.Config) (oauth2.TokenSource, error) {
	appConfig := &GitHubAppConfig{}

	if config.Extra != nil {
		if app, ok := config.Extra["github_app"].(map[string]interface{}); ok {
			appConfig.AppID = getInt64Value(app["app_id"])
			appConfig.InstallationID = getInt64Value(app["installation_id"])
			if privateKey, ok := app["private_key"].(string); ok {
				appConfig.PrivateKey = privateKey
			}
			if privateKeyPath, ok := app["private_key_path"].(string); ok {
				appConfig.PrivateKeyPath = privateKeyPath
			}
		}
	}

	if appConfig.AppID == 0 || appConfig.InstallationID == 0 {
		envConfig := LoadFromEnv()
		if envConfig.GitHubApp != nil {
			if appConfig.AppID == 0 {
				appConfig.AppID = envConfig.GitHubApp.AppID
			}
			if appConfig.InstallationID == 0 {
				appConfig.InstallationID = envConfig.GitHubApp.InstallationID
			}
			if appConfig.PrivateKey == "" {
				appConfig.PrivateKey = envConfig.GitHubApp.PrivateKey
			}
			if appConfig.PrivateKeyPath == "" {
				appConfig.PrivateKeyPath = envConfig.GitHubApp.PrivateKeyPath
			}
		}
	}

	if err := appConfig.Validate(); err != nil {
		return nil, err
	}

	privateKey := appConfig.PrivateKey
	if privateKey == "" && appConfig.PrivateKeyPath != "" {
		data, err := os.ReadFile(appConfig.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		privateKey = string(data)
	}

	transport, err := NewGitHubAppTransport(appConfig.AppID, appConfig.InstallationID, []byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	return transport, nil
}

// createOAuthTokenSource creates a token source for OAuth
func (f *Factory) createOAuthTokenSource(ctx context.Context, config auth.Config) (oauth2.TokenSource, error) {
	oauthConfig := &OAuthConfig{}

	if config.Extra != nil {
		if oauth, ok := config.Extra["oauth"].(map[string]interface{}); ok {
			if clientID, ok := oauth["client_id"].(string); ok {
				oauthConfig.ClientID = clientID
			}
			if clientSecret, ok := oauth["client_secret"].(string); ok {
				oauthConfig.ClientSecret = clientSecret
			}
			if accessToken, ok := oauth["access_token"].(string); ok {
				oauthConfig.AccessToken = accessToken
			}
			if refreshToken, ok := oauth["refresh_token"].(string); ok {
				oauthConfig.RefreshToken = refreshToken
			}
		}
	}

	if oauthConfig.ClientID == "" || oauthConfig.ClientSecret == "" || oauthConfig.AccessToken == "" {
		envConfig := LoadFromEnv()
		if envConfig.OAuth != nil {
			if oauthConfig.ClientID == "" {
				oauthConfig.ClientID = envConfig.OAuth.ClientID
			}
			if oauthConfig.ClientSecret == "" {
				oauthConfig.ClientSecret = envConfig.OAuth.ClientSecret
			}
			if oauthConfig.AccessToken == "" {
				oauthConfig.AccessToken = envConfig.OAuth.AccessToken
			}
			if oauthConfig.RefreshToken == "" {
				oauthConfig.RefreshToken = envConfig.OAuth.RefreshToken
			}
		}
	}

	if err := oauthConfig.Validate(); err != nil {
		return nil, err
	}

	if oauthConfig.AccessToken != "" {
		token := &oauth2.Token{
			AccessToken:  oauthConfig.AccessToken,
			RefreshToken: oauthConfig.RefreshToken,
			TokenType:    "Bearer",
		}

		if oauthConfig.ClientID != "" && oauthConfig.ClientSecret != "" {
			conf := &oauth2.Config{
				ClientID:     oauthConfig.ClientID,
				ClientSecret: oauthConfig.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://github.com/login/oauth/authorize",
					TokenURL: "https://github.com/login/oauth/access_token",
				},
			}
			return conf.TokenSource(ctx, token), nil
		}

		return oauth2.StaticTokenSource(token), nil
	}

	return nil, fmt.Errorf("no valid OAuth token available")
}

// GitHubAppTransport implements oauth2.TokenSource for GitHub App authentication
type GitHubAppTransport struct {
	appID          int64
	installationID int64
	privateKey     []byte
	httpClient     *http.Client
}

// NewGitHubAppTransport creates a new GitHub App transport
func NewGitHubAppTransport(appID, installationID int64, privateKey []byte) (*GitHubAppTransport, error) {
	return &GitHubAppTransport{
		appID:          appID,
		installationID: installationID,
		privateKey:     privateKey,
		httpClient:     http.DefaultClient,
	}, nil
}

// Token returns an installation access token
func (t *GitHubAppTransport) Token() (*oauth2.Token, error) {
	// TODO: Implement GitHub App authentication
	// This requires:
	// 1. Creating a JWT token signed with the app's private key
	// 2. Using the JWT to request an installation access token
	// 3. Caching and refreshing the installation token as needed
	return nil, fmt.Errorf("github app authentication not yet implemented")
}

// getInt64Value extracts an int64 value from an interface{}
func getInt64Value(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	default:
		return 0
	}
}
