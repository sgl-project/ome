package secret_in_vault

import (
	"testing"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretInVaultConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name:        "empty config",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "config with logger",
			options: []Option{
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewSecretInVaultConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestSecretInVaultConfig_Apply(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name:        "apply empty options",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "apply valid options",
			options: []Option{
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply nil option",
			options: []Option{
				nil,
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecretInVaultConfig{}
			err := config.Apply(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithViper(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectError bool
	}{
		{
			name: "valid viper config",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set(SecretInVaultConfigViperKeyNameKey, "test_prefix")
				v.Set("test_prefix."+NameViperKeyName, "test-secret")
				v.Set("test_prefix."+AuthTypeViperKeyName, "UserPrincipal")
				v.Set("test_prefix."+RegionViperKeyName, "us-ashburn-1")
				return v
			},
			expectError: false,
		},
		{
			name: "viper config without prefix",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set(NameViperKeyName, "test-secret")
				v.Set(AuthTypeViperKeyName, "UserPrincipal")
				v.Set(RegionViperKeyName, "us-ashburn-1")
				return v
			},
			expectError: false,
		},
		{
			name: "empty viper config",
			setupViper: func() *viper.Viper {
				return viper.New()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()
			config := &SecretInVaultConfig{}

			option := WithViper(v)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithAnotherLog(t *testing.T) {
	tests := []struct {
		name        string
		logger      interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid logger",
			logger:      testingPkg.SetupMockLogger(),
			expectError: false,
		},
		{
			name:        "nil logger",
			logger:      nil,
			expectError: true,
			errorMsg:    "nil another logger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecretInVaultConfig{}
			var option Option

			if tt.logger != nil {
				option = WithAnotherLog(tt.logger.(*testingPkg.MockLogger))
			} else {
				option = WithAnotherLog(nil)
			}

			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config.AnotherLogger)
			}
		})
	}
}

func TestSecretInVaultConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecretInVaultConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &SecretInVaultConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-secret",
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "missing auth type",
			config: &SecretInVaultConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				Name:          "test-secret",
				Region:        "us-ashburn-1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecretInVaultConfig_Constants(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, "secret_in_vault_config_viper_prefix", SecretInVaultConfigViperKeyNameKey)
	assert.Equal(t, "name", NameViperKeyName)
	assert.Equal(t, "auth_type", AuthTypeViperKeyName)
	assert.Equal(t, "region_override", RegionViperKeyName)
}

func TestSecretInVaultConfig_Integration(t *testing.T) {
	// Test complete configuration flow
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Test with viper
	v := viper.New()
	v.Set(NameViperKeyName, "test-secret")
	v.Set(AuthTypeViperKeyName, "UserPrincipal")
	v.Set(RegionViperKeyName, "us-ashburn-1")

	config, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.AnotherLogger)
	assert.Equal(t, "test-secret", config.Name)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "us-ashburn-1", config.Region)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}

func TestSecretInVaultConfig_WithPrefix(t *testing.T) {
	// Test configuration with prefix
	mockLogger := testingPkg.SetupMockLogger()

	v := viper.New()
	v.Set(SecretInVaultConfigViperKeyNameKey, "my_secret")
	v.Set("my_secret."+NameViperKeyName, "prefixed-secret")
	v.Set("my_secret."+AuthTypeViperKeyName, "InstancePrincipal")
	v.Set("my_secret."+RegionViperKeyName, "eu-frankfurt-1")

	config, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "prefixed-secret", config.Name)
	assert.Equal(t, principals.InstancePrincipal, *config.AuthType)
	assert.Equal(t, "eu-frankfurt-1", config.Region)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}
