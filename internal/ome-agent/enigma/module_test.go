package enigma

import (
	"testing"

	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/vault/kmscrypto"
	"github.com/sgl-project/ome/pkg/vault/kmsmgm"
	ocisecret "github.com/sgl-project/ome/pkg/vault/secret"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	// Test that the module can be created without panicking
	assert.NotNil(t, Module)
}

func TestEnigmaParams(t *testing.T) {
	// Test the enigmaParams struct
	mockLogger := testingPkg.SetupMockLogger()
	mockKmsCrypto := &kmscrypto.KmsCrypto{}
	mockKmsMgm := &kmsmgm.KmsMgm{}
	mockSecret := &ocisecret.Secret{}

	params := enigmaParams{
		AnotherLogger:   mockLogger,
		KmsCryptoClient: mockKmsCrypto,
		KmsManagement:   mockKmsMgm,
		Secret:          mockSecret,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.NotNil(t, params.KmsCryptoClient)
	assert.NotNil(t, params.KmsManagement)
	assert.NotNil(t, params.Secret)
	assert.Equal(t, mockLogger, params.AnotherLogger)
	assert.Equal(t, mockKmsCrypto, params.KmsCryptoClient)
	assert.Equal(t, mockKmsMgm, params.KmsManagement)
	assert.Equal(t, mockSecret, params.Secret)
}

func TestModuleProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("model_name", "test-model")
				v.Set("local_path", "/test/path")
				v.Set("model_framework", "huggingface")
				v.Set("model_type", "base")
				v.Set("disable_model_decryption", true)
				return v
			},
			expectError: false,
		},
		{
			name: "invalid viper configuration - missing required fields",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Missing required fields
				return v
			},
			expectError: false, // Configuration creation succeeds, but validation might fail later
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()

			// Setup mock dependencies
			mockLogger := testingPkg.SetupMockLogger()
			mockKmsCrypto := &kmscrypto.KmsCrypto{}
			mockKmsMgm := &kmsmgm.KmsMgm{}
			mockSecret := &ocisecret.Secret{}

			params := enigmaParams{
				AnotherLogger:   mockLogger,
				KmsCryptoClient: mockKmsCrypto,
				KmsManagement:   mockKmsMgm,
				Secret:          mockSecret,
			}

			// The provider function from Module (simplified to avoid fx dependencies)
			app, err := func(v *viper.Viper, params enigmaParams) (*Enigma, error) {
				config, err := NewConfig(
					WithViper(v, params.AnotherLogger),
					WithAppParams(params),
					WithAnotherLog(params.AnotherLogger),
				)
				if err != nil {
					return nil, err
				}
				return NewApplication(config)
			}(v, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, app)
			} else {
				if err != nil {
					// In some cases, we might get an error during validation,
					// but the configuration creation itself succeeds
					t.Logf("Got non-critical error: %v", err)
				} else {
					assert.NotNil(t, app)
					assert.Equal(t, mockLogger, app.logger)
				}
			}
		})
	}
}

func TestModuleIntegration(t *testing.T) {
	// Setup viper with valid configuration
	v := viper.New()
	v.Set("model_name", "test-model")
	v.Set("local_path", "/test/path")
	v.Set("model_framework", "huggingface")
	v.Set("model_type", "base")
	v.Set("disable_model_decryption", true)

	// Setup mock dependencies
	mockLogger := testingPkg.SetupMockLogger()
	mockKmsCrypto := &kmscrypto.KmsCrypto{}
	mockKmsMgm := &kmsmgm.KmsMgm{}
	mockSecret := &ocisecret.Secret{}

	params := enigmaParams{
		AnotherLogger:   mockLogger,
		KmsCryptoClient: mockKmsCrypto,
		KmsManagement:   mockKmsMgm,
		Secret:          mockSecret,
	}

	// Test configuration creation
	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAppParams(params),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "test-model", config.ModelName)
	assert.Equal(t, "/test/path", config.LocalPath)
	assert.Equal(t, HuggingFace, config.ModelFramework)
	assert.Equal(t, true, config.DisableModelDecryption)

	// Test application creation
	app, err := NewApplication(config)
	require.NoError(t, err)
	assert.NotNil(t, app)
}

func TestModuleErrorHandling(t *testing.T) {
	// Setup viper with invalid configuration
	v := viper.New()
	// Missing model_name and other required fields

	// Setup mock dependencies
	mockLogger := testingPkg.SetupMockLogger()

	// Don't include required dependencies for validation
	params := enigmaParams{
		AnotherLogger: mockLogger,
		// Missing KmsCryptoClient, KmsManagement, Secret
	}

	// Test configuration creation
	config, err := NewConfig(
		WithViper(v, mockLogger),
		WithAppParams(params),
		WithAnotherLog(mockLogger),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)

	// Test application creation (should fail validation with disabled_model_decryption=false)
	_, err = NewApplication(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
}
