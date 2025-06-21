package kmsvault

import (
	"testing"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	// Test that the module can be created without panicking
	assert.NotNil(t, Module)
}

func TestKmsVaultParams(t *testing.T) {
	// Test the kmsVault struct
	mockLogger := testingPkg.SetupMockLogger()

	params := kmsVault{
		AnotherLogger: mockLogger,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.Equal(t, mockLogger, params.AnotherLogger)
}

func TestModuleProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		setupParams func() kmsVault
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("region_override", "us-ashburn-1")
				v.Set("enable_obo_token", false)
				return v
			},
			setupParams: func() kmsVault {
				mockLogger := testingPkg.SetupMockLogger()

				return kmsVault{
					AnotherLogger: mockLogger,
				}
			},
			expectError: false,
		},
		{
			name: "invalid viper configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Missing required auth_type
				return v
			},
			setupParams: func() kmsVault {
				mockLogger := testingPkg.SetupMockLogger()

				return kmsVault{
					AnotherLogger: mockLogger,
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()
			params := tt.setupParams()

			// Test the provider function logic (without actual fx execution)
			config, err := NewConfig(
				WithViper(v),
				WithAnotherLogger(params.AnotherLogger),
			)

			if tt.expectError {
				// For missing auth_type, config creation succeeds but validation fails
				if err == nil {
					err = config.Validate()
				}
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestModuleIntegration(t *testing.T) {
	// Test module integration with valid configuration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Setup viper with valid configuration
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("region_override", "us-ashburn-1")
	v.Set("enable_obo_token", false)

	// Test configuration creation
	config, err := NewConfig(
		WithViper(v),
		WithAnotherLogger(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, authType, *config.AuthType)
	// Note: region field is private, so we can't test it directly
	assert.False(t, config.EnableOboToken)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}

func TestModuleWithOboToken(t *testing.T) {
	// Test module with OBO token configuration
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Setup viper with OBO token configuration
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("enable_obo_token", true)
	v.Set("obo_token", "test-obo-token")

	// Test configuration creation
	config, err := NewConfig(
		WithViper(v),
		WithAnotherLogger(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, authType, *config.AuthType)
	assert.True(t, config.EnableOboToken)
	assert.Equal(t, "test-obo-token", config.OboToken)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}

func TestModuleErrorHandling(t *testing.T) {
	// Test error handling in module provider
	mockLogger := testingPkg.SetupMockLogger()

	// Test with invalid OBO token configuration
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("enable_obo_token", true)
	// Missing obo_token

	config, err := NewConfig(
		WithViper(v),
		WithAnotherLogger(mockLogger),
	)

	// Should succeed in creation but fail validation
	require.NoError(t, err)
	assert.NotNil(t, config)

	// Validation should fail due to missing obo_token when enabled
	err = config.Validate()
	assert.Error(t, err)
}

func TestModuleConfigurationOptions(t *testing.T) {
	// Test various configuration options
	mockLogger := testingPkg.SetupMockLogger()

	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectValid bool
	}{
		{
			name: "minimal valid config",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				return v
			},
			expectValid: true,
		},
		{
			name: "config with region override",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "InstancePrincipal")
				v.Set("region_override", "eu-frankfurt-1")
				return v
			},
			expectValid: true,
		},
		{
			name: "config with OBO token enabled and provided",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "ResourcePrincipal")
				v.Set("enable_obo_token", true)
				v.Set("obo_token", "valid-token")
				return v
			},
			expectValid: true,
		},
		{
			name: "config with OBO token enabled but missing",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("auth_type", "UserPrincipal")
				v.Set("enable_obo_token", true)
				// obo_token not set
				return v
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()

			config, err := NewConfig(
				WithViper(v),
				WithAnotherLogger(mockLogger),
			)

			require.NoError(t, err)
			assert.NotNil(t, config)

			err = config.Validate()
			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
