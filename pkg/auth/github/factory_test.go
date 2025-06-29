package github

import (
	"context"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.GitHubPersonalAccessToken,
		auth.GitHubApp,
		auth.GitHubOAuth,
	}

	if len(authTypes) != len(expected) {
		t.Errorf("Expected %d auth types, got %d", len(expected), len(authTypes))
	}

	typeMap := make(map[auth.AuthType]bool)
	for _, at := range authTypes {
		typeMap[at] = true
	}

	for _, e := range expected {
		if !typeMap[e] {
			t.Errorf("Missing expected auth type: %s", e)
		}
	}
}

func TestFactory_Create_InvalidProvider(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderAWS, // Wrong provider
		AuthType: auth.GitHubPersonalAccessToken,
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}

func TestFactory_Create_UnsupportedAuthType(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGitHub,
		AuthType: auth.AWSAccessKey, // Wrong auth type for GitHub
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for unsupported auth type")
	}
}

func TestFactory_Create_PersonalAccessToken_Valid(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "Token in personal_access_token map",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubPersonalAccessToken,
				Extra: map[string]interface{}{
					"personal_access_token": map[string]interface{}{
						"token": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					},
				},
			},
		},
		{
			name: "Token directly in extra",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubPersonalAccessToken,
				Extra: map[string]interface{}{
					"token": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)
			if err != nil {
				t.Fatalf("Failed to create PAT credentials: %v", err)
			}

			if creds.Provider() != auth.ProviderGitHub {
				t.Errorf("Expected provider %s, got %s", auth.ProviderGitHub, creds.Provider())
			}

			if creds.Type() != auth.GitHubPersonalAccessToken {
				t.Errorf("Expected auth type %s, got %s", auth.GitHubPersonalAccessToken, creds.Type())
			}

			// Test token retrieval
			token, err := creds.Token(ctx)
			if err != nil {
				t.Errorf("Failed to get token: %v", err)
			}
			if token != "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {
				t.Errorf("Expected token to match input")
			}
		})
	}
}

func TestFactory_Create_PersonalAccessToken_Missing(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGitHub,
		AuthType: auth.GitHubPersonalAccessToken,
		Extra:    map[string]interface{}{
			// No token provided
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for missing PAT")
	}
}

func TestFactory_Create_GitHubApp_Missing(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "Missing app ID",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubApp,
				Extra: map[string]interface{}{
					"github_app": map[string]interface{}{
						"installation_id": int64(67890),
						"private_key":     "key",
					},
				},
			},
		},
		{
			name: "Missing installation ID",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubApp,
				Extra: map[string]interface{}{
					"github_app": map[string]interface{}{
						"app_id":      int64(12345),
						"private_key": "key",
					},
				},
			},
		},
		{
			name: "Missing private key",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubApp,
				Extra: map[string]interface{}{
					"github_app": map[string]interface{}{
						"app_id":          int64(12345),
						"installation_id": int64(67890),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(ctx, tt.config)
			if err == nil {
				t.Error("Expected error for incomplete GitHub App config")
			}
		})
	}
}

func TestFactory_Create_OAuth_Valid(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name   string
		config auth.Config
	}{
		{
			name: "Access token only",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubOAuth,
				Extra: map[string]interface{}{
					"oauth": map[string]interface{}{
						"access_token": "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					},
				},
			},
		},
		{
			name: "Full OAuth config",
			config: auth.Config{
				Provider: auth.ProviderGitHub,
				AuthType: auth.GitHubOAuth,
				Extra: map[string]interface{}{
					"oauth": map[string]interface{}{
						"client_id":     "Iv1.8a61f9b3a7aba766",
						"client_secret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
						"access_token":  "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
						"refresh_token": "ghr_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)
			if err != nil {
				t.Fatalf("Failed to create OAuth credentials: %v", err)
			}

			if creds.Provider() != auth.ProviderGitHub {
				t.Errorf("Expected provider %s, got %s", auth.ProviderGitHub, creds.Provider())
			}

			if creds.Type() != auth.GitHubOAuth {
				t.Errorf("Expected auth type %s, got %s", auth.GitHubOAuth, creds.Type())
			}
		})
	}
}

func TestFactory_Create_OAuth_Invalid(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderGitHub,
		AuthType: auth.GitHubOAuth,
		Extra: map[string]interface{}{
			"oauth": map[string]interface{}{
				// No valid token or credentials
			},
		},
	}

	_, err := factory.Create(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid OAuth config")
	}
}
