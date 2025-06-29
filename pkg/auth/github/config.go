package github

import (
	"fmt"
	"os"
)

// Config represents GitHub authentication configuration
type Config struct {
	// BaseURL allows custom GitHub Enterprise endpoints
	BaseURL string `mapstructure:"base_url,omitempty" json:"base_url,omitempty"`

	// PersonalAccessToken configuration
	PersonalAccessToken *PersonalAccessTokenConfig `mapstructure:"personal_access_token,omitempty" json:"personal_access_token,omitempty"`

	// GitHubApp configuration
	GitHubApp *GitHubAppConfig `mapstructure:"github_app,omitempty" json:"github_app,omitempty"`

	// OAuth configuration
	OAuth *OAuthConfig `mapstructure:"oauth,omitempty" json:"oauth,omitempty"`
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

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	config := &Config{}

	// Check for base URL (GitHub Enterprise)
	if baseURL := os.Getenv("GITHUB_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}

	// Check for personal access token
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token != "" {
		config.PersonalAccessToken = &PersonalAccessTokenConfig{
			Token: token,
		}
	}

	// Check for GitHub App configuration
	if appIDStr := os.Getenv("GITHUB_APP_ID"); appIDStr != "" {
		appConfig := &GitHubAppConfig{}
		if _, err := fmt.Sscanf(appIDStr, "%d", &appConfig.AppID); err != nil {
			// Invalid app ID format, skip GitHub App config
			return config
		}

		if installIDStr := os.Getenv("GITHUB_APP_INSTALLATION_ID"); installIDStr != "" {
			if _, err := fmt.Sscanf(installIDStr, "%d", &appConfig.InstallationID); err != nil {
				// Invalid installation ID format, skip GitHub App config
				return config
			}
		}

		appConfig.PrivateKey = os.Getenv("GITHUB_APP_PRIVATE_KEY")
		appConfig.PrivateKeyPath = os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")

		if appConfig.AppID > 0 && appConfig.InstallationID > 0 {
			config.GitHubApp = appConfig
		}
	}

	// Check for OAuth configuration
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")

	if clientID != "" || clientSecret != "" || accessToken != "" {
		config.OAuth = &OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			AccessToken:  accessToken,
		}
	}

	return config
}
