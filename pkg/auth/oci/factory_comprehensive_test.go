package oci

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Test Factory creation
func TestNewFactory_Comprehensive(t *testing.T) {
	tests := []struct {
		name   string
		logger logging.Interface
	}{
		{
			name:   "With logger",
			logger: logging.NewNopLogger(),
		},
		{
			name:   "With nil logger",
			logger: nil,
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

// Test Create method comprehensively
func TestFactory_Create_Comprehensive(t *testing.T) {
	// Create temporary OCI config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[DEFAULT]
user=ocid1.user.oc1..example
fingerprint=aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99
tenancy=ocid1.tenancy.oc1..example
region=us-ashburn-1
key_file=/path/to/key.pem

[PROFILE1]
user=ocid1.user.oc1..profile1
fingerprint=11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00
tenancy=ocid1.tenancy.oc1..profile1
region=us-phoenix-1
key_file=/path/to/key2.pem
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name      string
		config    auth.Config
		wantErr   bool
		checkType auth.AuthType
	}{
		{
			name: "User Principal - success",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Region:   "us-ashburn-1",
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": configPath,
						"profile":     "DEFAULT",
					},
				},
			},
			wantErr:   false,
			checkType: auth.OCIUserPrincipal,
		},
		{
			name: "User Principal - with profile",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Region:   "us-phoenix-1",
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": configPath,
						"profile":     "PROFILE1",
					},
				},
			},
			wantErr:   false,
			checkType: auth.OCIUserPrincipal,
		},
		{
			name: "User Principal - with session token",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Region:   "us-ashburn-1",
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path":       configPath,
						"profile":           "DEFAULT",
						"use_session_token": true,
					},
				},
			},
			wantErr:   false,
			checkType: auth.OCIUserPrincipal,
		},
		{
			name: "User Principal - missing config",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"profile": "DEFAULT",
					},
				},
			},
			wantErr: true,
		},
		// Skip Instance Principal test as it hangs trying to reach metadata service
		// {
		// 	name: "Instance Principal",
		// 	config: auth.Config{
		// 		Provider: auth.ProviderOCI,
		// 		AuthType: auth.OCIInstancePrincipal,
		// 		Region:   "us-ashburn-1",
		// 	},
		// 	// This will fail in non-OCI environment but we can test the attempt
		// 	wantErr:   true,
		// 	checkType: auth.OCIInstancePrincipal,
		// },
		{
			name: "Resource Principal",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIResourcePrincipal,
				Region:   "us-ashburn-1",
			},
			// This will fail in non-OCI environment but we can test the attempt
			wantErr:   true,
			checkType: auth.OCIResourcePrincipal,
		},
		{
			name: "OKE Workload Identity",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIOkeWorkloadIdentity,
				Region:   "us-ashburn-1",
			},
			// This will fail in non-OKE environment but we can test the attempt
			wantErr:   true,
			checkType: auth.OCIOkeWorkloadIdentity,
		},
		{
			name: "Invalid provider",
			config: auth.Config{
				Provider: auth.ProviderAWS,
				AuthType: auth.OCIInstancePrincipal,
			},
			wantErr: true,
		},
		{
			name: "Unsupported auth type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.AWSAccessKey,
			},
			wantErr: true,
		},
		{
			name: "Empty config",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a short timeout for tests that might try to reach metadata service
			testCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			creds, err := factory.Create(testCtx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && creds != nil {
				// Check provider
				if creds.Provider() != auth.ProviderOCI {
					t.Errorf("Expected provider %s, got %s", auth.ProviderOCI, creds.Provider())
				}

				// Check auth type
				if creds.Type() != tt.checkType {
					t.Errorf("Expected auth type %s, got %s", tt.checkType, creds.Type())
				}

				// Check it's OCICredentials
				ociCreds, ok := creds.(*OCICredentials)
				if !ok {
					t.Error("Expected OCICredentials type")
				} else {
					// Check region
					if tt.config.Region != "" && ociCreds.region != tt.config.Region {
						t.Errorf("Expected region %s, got %s", tt.config.Region, ociCreds.region)
					}
				}
			}
		})
	}
}

// Test UserPrincipalConfig environment variable handling
func TestUserPrincipalConfig_ApplyEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		initial  UserPrincipalConfig
		expected UserPrincipalConfig
	}{
		{
			name: "Apply OCI_CONFIG_PATH",
			envVars: map[string]string{
				"OCI_CONFIG_PATH": "/custom/path/config",
			},
			initial: UserPrincipalConfig{},
			expected: UserPrincipalConfig{
				ConfigPath: "/custom/path/config",
			},
		},
		{
			name: "Apply OCI_PROFILE",
			envVars: map[string]string{
				"OCI_PROFILE": "MYPROFILE",
			},
			initial: UserPrincipalConfig{},
			expected: UserPrincipalConfig{
				Profile: "MYPROFILE",
			},
		},
		{
			name: "Apply both env vars",
			envVars: map[string]string{
				"OCI_CONFIG_PATH": "/custom/path/config",
				"OCI_PROFILE":     "MYPROFILE",
			},
			initial: UserPrincipalConfig{},
			expected: UserPrincipalConfig{
				ConfigPath: "/custom/path/config",
				Profile:    "MYPROFILE",
			},
		},
		{
			name: "Don't override existing values",
			envVars: map[string]string{
				"OCI_CONFIG_PATH": "/env/path/config",
				"OCI_PROFILE":     "ENVPROFILE",
			},
			initial: UserPrincipalConfig{
				ConfigPath: "/existing/path/config",
				Profile:    "EXISTING",
			},
			expected: UserPrincipalConfig{
				ConfigPath: "/existing/path/config",
				Profile:    "EXISTING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				oldValue := os.Getenv(key)
				os.Setenv(key, value)
				defer os.Setenv(key, oldValue)
			}

			config := tt.initial
			config.ApplyEnvironment()

			if config.ConfigPath != tt.expected.ConfigPath {
				t.Errorf("Expected ConfigPath %s, got %s", tt.expected.ConfigPath, config.ConfigPath)
			}
			if config.Profile != tt.expected.Profile {
				t.Errorf("Expected Profile %s, got %s", tt.expected.Profile, config.Profile)
			}
		})
	}
}

// Test UserPrincipalConfig validation comprehensively
func TestUserPrincipalConfig_Validate_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		config  UserPrincipalConfig
		wantErr bool
	}{
		{
			name: "Valid config",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "DEFAULT",
			},
			wantErr: false,
		},
		{
			name: "Valid config with session token",
			config: UserPrincipalConfig{
				ConfigPath:      "~/.oci/config",
				Profile:         "DEFAULT",
				UseSessionToken: true,
			},
			wantErr: false,
		},
		{
			name: "Empty config path",
			config: UserPrincipalConfig{
				ConfigPath: "",
				Profile:    "DEFAULT",
			},
			wantErr: true,
		},
		{
			name: "Empty profile",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "",
			},
			wantErr: true,
		},
		{
			name: "Both empty",
			config: UserPrincipalConfig{
				ConfigPath: "",
				Profile:    "",
			},
			wantErr: true,
		},
		{
			name: "Whitespace only config path",
			config: UserPrincipalConfig{
				ConfigPath: "   ",
				Profile:    "DEFAULT",
			},
			wantErr: true,
		},
		{
			name: "Whitespace only profile",
			config: UserPrincipalConfig{
				ConfigPath: "~/.oci/config",
				Profile:    "   ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test edge cases for Create method
func TestFactory_Create_EdgeCases(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	tests := []struct {
		name    string
		config  auth.Config
		wantErr bool
	}{
		{
			name: "Nil Extra map",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra:    nil,
			},
			wantErr: true,
		},
		{
			name: "Empty Extra map",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra:    map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "Invalid user_principal type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": "not a map",
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid config_path type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": 123, // Not a string
						"profile":     "DEFAULT",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid profile type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": "~/.oci/config",
						"profile":     true, // Not a string
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid use_session_token type",
			config: auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path":       "~/.oci/config",
						"profile":           "DEFAULT",
						"use_session_token": "yes", // Not a bool
					},
				},
			},
			wantErr: false, // Type assertion fails, defaults to false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(ctx, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test concurrent Create calls
func TestFactory_Create_Concurrent(t *testing.T) {
	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	// Create temporary config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[DEFAULT]
user=ocid1.user.oc1..example
fingerprint=aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99
tenancy=ocid1.tenancy.oc1..example
region=us-ashburn-1
key_file=/path/to/key.pem
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: auth.OCIUserPrincipal,
		Region:   "us-ashburn-1",
		Extra: map[string]interface{}{
			"user_principal": map[string]interface{}{
				"config_path": configPath,
				"profile":     "DEFAULT",
			},
		},
	}

	// Run concurrent creates
	numGoroutines := 10
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := factory.Create(ctx, config)
			errors <- err
		}()
	}

	// Check results
	for i := 0; i < numGoroutines; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent create failed: %v", err)
		}
	}
}

// Test factory with different loggers
func TestFactory_WithDifferentLoggers(t *testing.T) {
	tests := []struct {
		name   string
		logger logging.Interface
	}{
		{
			name:   "Nop logger",
			logger: logging.NewNopLogger(),
		},
		{
			name:   "Nil logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.logger)

			// Test supported auth types
			authTypes := factory.SupportedAuthTypes()
			if len(authTypes) != 4 {
				t.Errorf("Expected 4 auth types, got %d", len(authTypes))
			}

			// Test create with invalid provider (should handle nil logger gracefully)
			ctx := context.Background()
			config := auth.Config{
				Provider: auth.ProviderAWS,
				AuthType: auth.OCIInstancePrincipal,
			}
			_, err := factory.Create(ctx, config)
			if err == nil {
				t.Error("Expected error for invalid provider")
			}
		})
	}
}

// Benchmark factory creation
func BenchmarkFactory_Create(b *testing.B) {
	// Create temporary config
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[DEFAULT]
user=ocid1.user.oc1..example
fingerprint=aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99
tenancy=ocid1.tenancy.oc1..example
region=us-ashburn-1
key_file=/path/to/key.pem
`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		b.Fatalf("Failed to create test config file: %v", err)
	}

	logger := logging.NewNopLogger()
	factory := NewFactory(logger)
	ctx := context.Background()

	config := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: auth.OCIUserPrincipal,
		Region:   "us-ashburn-1",
		Extra: map[string]interface{}{
			"user_principal": map[string]interface{}{
				"config_path": configPath,
				"profile":     "DEFAULT",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = factory.Create(ctx, config)
	}
}
