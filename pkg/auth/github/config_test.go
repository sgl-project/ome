package github

import (
	"os"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(t *testing.T, config *Config)
	}{
		{
			name: "Personal Access Token from GITHUB_TOKEN",
			envVars: map[string]string{
				"GITHUB_TOKEN": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			validate: func(t *testing.T, config *Config) {
				if config.PersonalAccessToken == nil {
					t.Fatal("Expected PersonalAccessToken to be set")
				}
				if config.PersonalAccessToken.Token != "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {
					t.Errorf("Expected token to be ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx, got %s", config.PersonalAccessToken.Token)
				}
			},
		},
		{
			name: "Personal Access Token from GH_TOKEN",
			envVars: map[string]string{
				"GH_TOKEN": "ghp_yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
			},
			validate: func(t *testing.T, config *Config) {
				if config.PersonalAccessToken == nil {
					t.Fatal("Expected PersonalAccessToken to be set")
				}
				if config.PersonalAccessToken.Token != "ghp_yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy" {
					t.Errorf("Expected token to be ghp_yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy, got %s", config.PersonalAccessToken.Token)
				}
			},
		},
		{
			name: "GitHub App configuration",
			envVars: map[string]string{
				"GITHUB_APP_ID":               "12345",
				"GITHUB_APP_INSTALLATION_ID":  "67890",
				"GITHUB_APP_PRIVATE_KEY_PATH": "/path/to/key.pem",
			},
			validate: func(t *testing.T, config *Config) {
				if config.GitHubApp == nil {
					t.Fatal("Expected GitHubApp to be set")
				}
				if config.GitHubApp.AppID != 12345 {
					t.Errorf("Expected AppID to be 12345, got %d", config.GitHubApp.AppID)
				}
				if config.GitHubApp.InstallationID != 67890 {
					t.Errorf("Expected InstallationID to be 67890, got %d", config.GitHubApp.InstallationID)
				}
				if config.GitHubApp.PrivateKeyPath != "/path/to/key.pem" {
					t.Errorf("Expected PrivateKeyPath to be /path/to/key.pem, got %s", config.GitHubApp.PrivateKeyPath)
				}
			},
		},
		{
			name: "OAuth configuration",
			envVars: map[string]string{
				"GITHUB_CLIENT_ID":     "Iv1.8a61f9b3a7aba766",
				"GITHUB_CLIENT_SECRET": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
				"GITHUB_ACCESS_TOKEN":  "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			validate: func(t *testing.T, config *Config) {
				if config.OAuth == nil {
					t.Fatal("Expected OAuth to be set")
				}
				if config.OAuth.ClientID != "Iv1.8a61f9b3a7aba766" {
					t.Errorf("Expected ClientID to be Iv1.8a61f9b3a7aba766, got %s", config.OAuth.ClientID)
				}
				if config.OAuth.ClientSecret != "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {
					t.Errorf("Expected ClientSecret to be xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx, got %s", config.OAuth.ClientSecret)
				}
				if config.OAuth.AccessToken != "gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {
					t.Errorf("Expected AccessToken to be gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx, got %s", config.OAuth.AccessToken)
				}
			},
		},
		{
			name: "GitHub Enterprise base URL",
			envVars: map[string]string{
				"GITHUB_BASE_URL": "https://github.enterprise.com",
				"GITHUB_TOKEN":    "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			},
			validate: func(t *testing.T, config *Config) {
				if config.BaseURL != "https://github.enterprise.com" {
					t.Errorf("Expected BaseURL to be https://github.enterprise.com, got %s", config.BaseURL)
				}
			},
		},
		{
			name:    "Empty configuration",
			envVars: map[string]string{},
			validate: func(t *testing.T, config *Config) {
				if config.PersonalAccessToken != nil {
					t.Error("Expected PersonalAccessToken to be nil")
				}
				if config.GitHubApp != nil {
					t.Error("Expected GitHubApp to be nil")
				}
				if config.OAuth != nil {
					t.Error("Expected OAuth to be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars
			originalEnv := make(map[string]string)
			for k := range tt.envVars {
				originalEnv[k] = os.Getenv(k)
			}

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Test
			config := LoadFromEnv()
			tt.validate(t, config)

			// Restore original env vars
			for k, v := range originalEnv {
				if v == "" {
					os.Unsetenv(k)
				} else {
					os.Setenv(k, v)
				}
			}
		})
	}
}

func TestGitHubAppConfig_ValidateWithEnv(t *testing.T) {
	tests := []struct {
		name      string
		config    GitHubAppConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid with all fields",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
				PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\n...",
			},
			wantError: false,
		},
		{
			name: "Valid with private key path",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
				PrivateKeyPath: "/path/to/key.pem",
			},
			wantError: false,
		},
		{
			name: "Missing AppID",
			config: GitHubAppConfig{
				InstallationID: 67890,
				PrivateKey:     "key",
			},
			wantError: true,
			errorMsg:  "app_id is required",
		},
		{
			name: "Missing InstallationID",
			config: GitHubAppConfig{
				AppID:      12345,
				PrivateKey: "key",
			},
			wantError: true,
			errorMsg:  "installation_id is required",
		},
		{
			name: "Missing private key and path",
			config: GitHubAppConfig{
				AppID:          12345,
				InstallationID: 67890,
			},
			wantError: true,
			errorMsg:  "either private_key or private_key_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" && err.Error() != tt.errorMsg {
				t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}
