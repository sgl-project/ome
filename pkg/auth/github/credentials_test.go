package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
)

func TestGitHubCredentials_Provider(t *testing.T) {
	creds := &GitHubCredentials{
		authType: auth.GitHubPersonalAccessToken,
		logger:   logging.NewNopLogger(),
	}

	if provider := creds.Provider(); provider != auth.ProviderGitHub {
		t.Errorf("Expected provider %s, got %s", auth.ProviderGitHub, provider)
	}
}

func TestGitHubCredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Personal Access Token",
			authType: auth.GitHubPersonalAccessToken,
		},
		{
			name:     "GitHub App",
			authType: auth.GitHubApp,
		},
		{
			name:     "OAuth",
			authType: auth.GitHubOAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &GitHubCredentials{
				authType: tt.authType,
			}
			if typ := creds.Type(); typ != tt.authType {
				t.Errorf("Expected type %s, got %s", tt.authType, typ)
			}
		})
	}
}

func TestGitHubCredentials_Token(t *testing.T) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	})

	creds := &GitHubCredentials{
		tokenSource: tokenSource,
		logger:      logging.NewNopLogger(),
	}

	ctx := context.Background()
	token, err := creds.Token(ctx)
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", token)
	}
}

func TestGitHubCredentials_SignRequest(t *testing.T) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	})

	creds := &GitHubCredentials{
		tokenSource: tokenSource,
		logger:      logging.NewNopLogger(),
	}

	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	ctx := context.Background()

	err := creds.SignRequest(ctx, req)
	if err != nil {
		t.Fatalf("Failed to sign request: %v", err)
	}

	// Check Authorization header
	authHeader := req.Header.Get("Authorization")
	expectedAuth := "Bearer test-token"
	if authHeader != expectedAuth {
		t.Errorf("Expected Authorization header %s, got %s", expectedAuth, authHeader)
	}

	// Check Accept header
	acceptHeader := req.Header.Get("Accept")
	if acceptHeader != "application/vnd.github.v3+json" {
		t.Errorf("Expected Accept header 'application/vnd.github.v3+json', got %s", acceptHeader)
	}
}

func TestGitHubCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *GitHubCredentials
		expected bool
	}{
		{
			name:     "No cached token",
			creds:    &GitHubCredentials{},
			expected: true,
		},
		{
			name: "Valid token",
			creds: &GitHubCredentials{
				cachedToken: &oauth2.Token{
					AccessToken: "test-token",
					Expiry:      time.Now().Add(1 * time.Hour),
				},
			},
			expected: false,
		},
		{
			name: "Expired token",
			creds: &GitHubCredentials{
				cachedToken: &oauth2.Token{
					AccessToken: "test-token",
					Expiry:      time.Now().Add(-1 * time.Hour),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.creds.IsExpired()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPersonalAccessTokenConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    PersonalAccessTokenConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: PersonalAccessTokenConfig{
				Token: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			wantError: false,
		},
		{
			name:      "Empty token",
			config:    PersonalAccessTokenConfig{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGitHubAppConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    GitHubAppConfig
		wantError bool
	}{
		{
			name: "Valid config with private key",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
				PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----",
			},
			wantError: false,
		},
		{
			name: "Valid config with private key path",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
				PrivateKeyPath: "/path/to/key.pem",
			},
			wantError: false,
		},
		{
			name: "Missing app ID",
			config: GitHubAppConfig{
				InstallationID: 67890,
				PrivateKey:     "key",
			},
			wantError: true,
		},
		{
			name: "Missing installation ID",
			config: GitHubAppConfig{
				AppID:      12345,
				PrivateKey: "key",
			},
			wantError: true,
		},
		{
			name: "Missing private key",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestOAuthConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    OAuthConfig
		wantError bool
	}{
		{
			name: "Valid config with access token",
			config: OAuthConfig{
				AccessToken: "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			wantError: false,
		},
		{
			name: "Valid config with client credentials",
			config: OAuthConfig{
				ClientID:     "Iv1.8a61f9b3a7aba766",
				ClientSecret: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			wantError: false,
		},
		{
			name: "Valid config with all fields",
			config: OAuthConfig{
				ClientID:     "Iv1.8a61f9b3a7aba766",
				ClientSecret: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				AccessToken:  "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				RefreshToken: "ghr_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			wantError: false,
		},
		{
			name: "Missing required fields",
			config: OAuthConfig{
				ClientID: "Iv1.8a61f9b3a7aba766",
				// Missing client secret and access token
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    OAuthConfig{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestStaticTokenSource(t *testing.T) {
	tokenSource := newStaticTokenSource("test-token")

	token, err := tokenSource.Token()
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if token.AccessToken != "test-token" {
		t.Errorf("Expected access token 'test-token', got %s", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("Expected token type 'Bearer', got %s", token.TokenType)
	}
}
