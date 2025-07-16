package ocisecret

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/ome/pkg/principals"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
)

func TestNewConfig(t *testing.T) {
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
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestConfig_Apply(t *testing.T) {
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
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply nil option",
			options: []Option{
				nil,
				WithAnotherLogger(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			err := config.Apply(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithAnotherLogger(t *testing.T) {
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
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			var option Option

			if tt.logger != nil {
				option = WithAnotherLogger(tt.logger.(*testingPkg.MockLogger))
			} else {
				option = WithAnotherLogger(nil)
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
			mockLogger := testingPkg.SetupMockLogger()
			config := &Config{}

			option := WithViper(v, mockLogger)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithParams(t *testing.T) {
	tests := []struct {
		name        string
		setupParams func() appParams
		expectError bool
	}{
		{
			name: "valid params",
			setupParams: func() appParams {
				mockLogger := testingPkg.SetupMockLogger()

				return appParams{
					AnotherLogger: mockLogger,
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.setupParams()
			config := &Config{}

			option := WithParams(params)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "missing auth type",
			config: &Config{
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

func TestConfig_Integration(t *testing.T) {
	// Test complete configuration flow
	mockLogger := testingPkg.SetupMockLogger()
	authType := principals.UserPrincipal

	// Test with viper
	v := viper.New()
	v.Set("auth_type", "UserPrincipal")
	v.Set("region_override", "us-ashburn-1")

	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAnotherLogger(mockLogger),
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
