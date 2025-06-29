package github

import (
	"context"
	"fmt"
	"net/http"
	"os"

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

	var tokenSource oauth2.TokenSource
	var err error

	switch config.AuthType {
	case auth.GitHubPersonalAccessToken:
		tokenSource, err = f.createPersonalAccessTokenSource(config)
	case auth.GitHubApp:
		tokenSource, err = f.createGitHubAppTokenSource(ctx, config)
	case auth.GitHubOAuth:
		tokenSource, err = f.createOAuthTokenSource(config)
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
	// Extract PAT config
	patConfig := PersonalAccessTokenConfig{}

	if config.Extra != nil {
		if pat, ok := config.Extra["personal_access_token"].(map[string]interface{}); ok {
			if token, ok := pat["token"].(string); ok {
				patConfig.Token = token
			}
		} else if token, ok := config.Extra["token"].(string); ok {
			// Also support direct token in extra
			patConfig.Token = token
		}
	}

	// Check environment variable
	if patConfig.Token == "" {
		patConfig.Token = os.Getenv("GITHUB_TOKEN")
		if patConfig.Token == "" {
			patConfig.Token = os.Getenv("GH_TOKEN")
		}
	}

	// Validate
	if err := patConfig.Validate(); err != nil {
		return nil, err
	}

	return NewStaticTokenSource(patConfig.Token), nil
}

// createGitHubAppTokenSource creates a token source for GitHub App
func (f *Factory) createGitHubAppTokenSource(ctx context.Context, config auth.Config) (oauth2.TokenSource, error) {
	// Extract GitHub App config
	appConfig := GitHubAppConfig{}

	if config.Extra != nil {
		if app, ok := config.Extra["github_app"].(map[string]interface{}); ok {
			if appID, ok := app["app_id"].(float64); ok {
				appConfig.AppID = int64(appID)
			} else if appID, ok := app["app_id"].(int64); ok {
				appConfig.AppID = appID
			}
			if installationID, ok := app["installation_id"].(float64); ok {
				appConfig.InstallationID = int64(installationID)
			} else if installationID, ok := app["installation_id"].(int64); ok {
				appConfig.InstallationID = installationID
			}
			if privateKey, ok := app["private_key"].(string); ok {
				appConfig.PrivateKey = privateKey
			}
			if privateKeyPath, ok := app["private_key_path"].(string); ok {
				appConfig.PrivateKeyPath = privateKeyPath
			}
		}
	}

	// Check environment variables
	if appConfig.AppID == 0 {
		if appIDStr := os.Getenv("GITHUB_APP_ID"); appIDStr != "" {
			var appID int64
			fmt.Sscanf(appIDStr, "%d", &appID)
			appConfig.AppID = appID
		}
	}
	if appConfig.InstallationID == 0 {
		if installIDStr := os.Getenv("GITHUB_APP_INSTALLATION_ID"); installIDStr != "" {
			var installID int64
			fmt.Sscanf(installIDStr, "%d", &installID)
			appConfig.InstallationID = installID
		}
	}
	if appConfig.PrivateKeyPath == "" {
		appConfig.PrivateKeyPath = os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	}

	// Validate
	if err := appConfig.Validate(); err != nil {
		return nil, err
	}

	// Read private key if path provided
	privateKey := appConfig.PrivateKey
	if privateKey == "" && appConfig.PrivateKeyPath != "" {
		data, err := os.ReadFile(appConfig.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		privateKey = string(data)
	}

	// Create GitHub App transport
	transport, err := NewGitHubAppTransport(appConfig.AppID, appConfig.InstallationID, []byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	return transport, nil
}

// createOAuthTokenSource creates a token source for OAuth
func (f *Factory) createOAuthTokenSource(config auth.Config) (oauth2.TokenSource, error) {
	// Extract OAuth config
	oauthConfig := OAuthConfig{}

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

	// Check environment variables
	if oauthConfig.ClientID == "" {
		oauthConfig.ClientID = os.Getenv("GITHUB_CLIENT_ID")
	}
	if oauthConfig.ClientSecret == "" {
		oauthConfig.ClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	}
	if oauthConfig.AccessToken == "" {
		oauthConfig.AccessToken = os.Getenv("GITHUB_ACCESS_TOKEN")
	}

	// Validate
	if err := oauthConfig.Validate(); err != nil {
		return nil, err
	}

	// If we have an access token, use it directly
	if oauthConfig.AccessToken != "" {
		token := &oauth2.Token{
			AccessToken:  oauthConfig.AccessToken,
			RefreshToken: oauthConfig.RefreshToken,
			TokenType:    "Bearer",
		}

		// If we have OAuth app credentials, create a refreshable token source
		if oauthConfig.ClientID != "" && oauthConfig.ClientSecret != "" {
			conf := &oauth2.Config{
				ClientID:     oauthConfig.ClientID,
				ClientSecret: oauthConfig.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://github.com/login/oauth/authorize",
					TokenURL: "https://github.com/login/oauth/access_token",
				},
			}
			return conf.TokenSource(context.Background(), token), nil
		}

		// Otherwise, just use the static token
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
	// TODO: Implement JWT signing and installation token exchange
	// This is a simplified implementation
	return &GitHubAppTransport{
		appID:          appID,
		installationID: installationID,
		privateKey:     privateKey,
		httpClient:     http.DefaultClient,
	}, nil
}

// Token returns an installation access token
func (t *GitHubAppTransport) Token() (*oauth2.Token, error) {
	// TODO: Implement proper GitHub App authentication
	// 1. Create JWT signed with private key
	// 2. Exchange JWT for installation access token
	// 3. Cache and refresh as needed

	// For now, return an error
	return nil, fmt.Errorf("GitHub App authentication not fully implemented")
}
