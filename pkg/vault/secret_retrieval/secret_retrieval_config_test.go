package secret_retrieval

import (
	"testing"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretRetrievalConfig(t *testing.T) {
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
			config, err := NewSecretRetrievalConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestSecretRetrievalConfig_Apply(t *testing.T) {
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
			config := &SecretRetrievalConfig{}
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
				v.Set("auth_type", "UserPrincipal")
				v.Set("region_override", "us-ashburn-1")
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
			config := &SecretRetrievalConfig{}

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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecretRetrievalConfig{}
			var option Option

			if tt.logger != nil {
				option = WithAnotherLog(tt.logger.(*testingPkg.MockLogger))
			} else {
				option = WithAnotherLog(nil)
			}

			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.logger != nil {
					assert.NotNil(t, config.AnotherLogger)
				}
			}
		})
	}
}

func TestSecretRetrievalConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecretRetrievalConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &SecretRetrievalConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "missing auth type",
			config: &SecretRetrievalConfig{
				AnotherLogger: testingPkg.SetupMockLogger(),
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

func TestSecretRetrievalConfig_Integration(t *testing.T) {
	// Test complete configuration flow
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Test with viper
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("region_override", "us-ashburn-1")

	config, err := NewSecretRetrievalConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.AnotherLogger)
	assert.Equal(t, authType, *config.AuthType)
	assert.Equal(t, "us-ashburn-1", config.Region)

	// Test validation
	err = config.Validate()
	assert.NoError(t, err)
}
