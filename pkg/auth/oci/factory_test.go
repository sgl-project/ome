package oci

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

func TestNewFactory(t *testing.T) {
	tests := []struct {
		name   string
		logger logging.Interface
	}{
		{
			name:   "With zap test logger",
			logger: logging.ForZap(zaptest.NewLogger(t)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.logger)
			if factory == nil {
				t.Fatal("Expected non-nil factory")
			}
			if factory.logger != tt.logger {
				t.Error("Logger not properly set")
			}
		})
	}
}

func TestFactory_SupportedAuthTypes(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)

	authTypes := factory.SupportedAuthTypes()
	expected := []auth.AuthType{
		auth.OCIUserPrincipal,
		auth.OCIInstancePrincipal,
		auth.OCIResourcePrincipal,
		auth.OCIOkeWorkloadIdentity,
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

func TestFactory_Create(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create temporary OCI config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[DEFAULT]
user=ocid1.user.oc1..example
fingerprint=aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99
tenancy=ocid1.tenancy.oc1..example
region=us-ashburn-1
key_file=` + filepath.Join(tmpDir, "key.pem") + `

[PROFILE1]
user=ocid1.user.oc1..profile1
fingerprint=11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00
tenancy=ocid1.tenancy.oc1..profile1
region=us-phoenix-1
key_file=` + filepath.Join(tmpDir, "key.pem") + `
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create a fake key file
	keyContent := `-----BEGIN RSA PRIVATE KEY-----
FAKE KEY FOR TESTING
-----END RSA PRIVATE KEY-----`
	if err := os.WriteFile(filepath.Join(tmpDir, "key.pem"), []byte(keyContent), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	tests := []struct {
		name    string
		config  auth.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Invalid provider",
			config: auth.Config{
				Provider: auth.ProviderAWS,
				AuthType: auth.OCIInstancePrincipal,
			},
			wantErr: true,
			errMsg:  "invalid provider",
		},
		{
			name: "Unsupported auth type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: "invalid-type",
			},
			wantErr: true,
			errMsg:  "unsupported OCI auth type",
		},
		{
			name: "User principal without config",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						// Explicitly empty to test validation
					},
				},
			},
			wantErr: false, // Will use defaults from ApplyEnvironment
		},
		{
			name: "User principal with config",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": configPath,
						"profile":     "DEFAULT",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "User principal with session token",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path":       configPath,
						"profile":           "DEFAULT",
						"use_session_token": true,
					},
				},
			},
			wantErr: false,
		},
		// Note: Instance Principal, Resource Principal, and OKE Workload Identity
		// would fail in non-OCI environments, so we just test they don't panic
		{
			name: "Instance principal",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIInstancePrincipal,
			},
			wantErr: true, // Expected to fail in test environment
		},
		{
			name: "Resource principal",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIResourcePrincipal,
			},
			wantErr: true, // Expected to fail in test environment
		},
		{
			name: "OKE workload identity",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIOkeWorkloadIdentity,
			},
			wantErr: true, // Expected to fail in test environment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := factory.Create(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Create() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if !tt.wantErr && creds == nil {
				t.Error("Expected non-nil credentials")
			}
		})
	}
}

func TestFactory_CreateWithEnvironment(t *testing.T) {
	logger := logging.ForZap(zaptest.NewLogger(t))
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create temporary OCI config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[TEST]
user=ocid1.user.oc1..test
fingerprint=00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff
tenancy=ocid1.tenancy.oc1..test
region=eu-frankfurt-1
key_file=` + filepath.Join(tmpDir, "key.pem") + `
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create a fake key file
	keyContent := `-----BEGIN RSA PRIVATE KEY-----
FAKE KEY FOR TESTING
-----END RSA PRIVATE KEY-----`
	if err := os.WriteFile(filepath.Join(tmpDir, "key.pem"), []byte(keyContent), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}

	// Set environment variables
	t.Setenv("OCI_CONFIG_PATH", configPath)
	t.Setenv("OCI_PROFILE", "TEST")
	t.Setenv("OCI_USE_SESSION_TOKEN", "true")

	config := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: auth.OCIUserPrincipal,
		Extra: map[string]interface{}{
			"user_principal": map[string]interface{}{},
		},
	}

	creds, err := factory.Create(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create credentials with environment: %v", err)
	}

	if creds == nil {
		t.Fatal("Expected non-nil credentials")
	}

	ociCreds, ok := creds.(*OCICredentials)
	if !ok {
		t.Fatal("Expected OCICredentials type")
	}

	if ociCreds.authType != auth.OCIUserPrincipal {
		t.Errorf("Expected auth type %s, got %s", auth.OCIUserPrincipal, ociCreds.authType)
	}
}

func TestUserPrincipalConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    UserPrincipalConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "DEFAULT",
			},
			wantError: false,
		},
		{
			name: "Missing config path",
			config: UserPrincipalConfig{
				Profile: "DEFAULT",
			},
			wantError: true,
		},
		{
			name: "Missing profile",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
			},
			wantError: true,
		},
		{
			name: "Empty config path",
			config: UserPrincipalConfig{
				ConfigPath: "   ",
				Profile:    "DEFAULT",
			},
			wantError: true,
		},
		{
			name: "Empty profile",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "   ",
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

func TestUserPrincipalConfig_ApplyEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		initial  UserPrincipalConfig
		envVars  map[string]string
		expected UserPrincipalConfig
	}{
		{
			name:    "Apply all from environment",
			initial: UserPrincipalConfig{},
			envVars: map[string]string{
				"OCI_CONFIG_PATH":       "/custom/path/config",
				"OCI_PROFILE":           "CUSTOM",
				"OCI_USE_SESSION_TOKEN": "true",
			},
			expected: UserPrincipalConfig{
				ConfigPath:      "/custom/path/config",
				Profile:         "CUSTOM",
				UseSessionToken: true,
			},
		},
		{
			name:    "Boolean parsing flexibility",
			initial: UserPrincipalConfig{},
			envVars: map[string]string{
				"OCI_USE_SESSION_TOKEN": "1",
			},
			expected: UserPrincipalConfig{
				ConfigPath:      expandPath("~/.oci/config"),
				Profile:         "DEFAULT",
				UseSessionToken: true,
			},
		},
		{
			name:    "Use defaults when no environment",
			initial: UserPrincipalConfig{},
			envVars: map[string]string{},
			expected: UserPrincipalConfig{
				ConfigPath: expandPath("~/.oci/config"),
				Profile:    "DEFAULT",
			},
		},
		{
			name: "Don't override existing values",
			initial: UserPrincipalConfig{
				ConfigPath: "/existing/config",
				Profile:    "EXISTING",
			},
			envVars: map[string]string{
				"OCI_CONFIG_PATH": "/env/config",
				"OCI_PROFILE":     "ENV",
			},
			expected: UserPrincipalConfig{
				ConfigPath: "/existing/config",
				Profile:    "EXISTING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config := tt.initial
			config.ApplyEnvironment()

			if config.ConfigPath != tt.expected.ConfigPath {
				t.Errorf("ConfigPath = %v, want %v", config.ConfigPath, tt.expected.ConfigPath)
			}
			if config.Profile != tt.expected.Profile {
				t.Errorf("Profile = %v, want %v", config.Profile, tt.expected.Profile)
			}
			if config.UseSessionToken != tt.expected.UseSessionToken {
				t.Errorf("UseSessionToken = %v, want %v", config.UseSessionToken, tt.expected.UseSessionToken)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}
