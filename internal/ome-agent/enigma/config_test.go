package enigma

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	testingPkg "github.com/sgl-project/sgl-ome/pkg/testing"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmscrypto"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmsmgm"
	ocisecret "github.com/sgl-project/sgl-ome/pkg/vault/secret"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
		errorMsg    string
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
		{
			name: "config with nil logger",
			options: []Option{
				WithAnotherLog(nil),
			},
			expectError: true,
			errorMsg:    "logger cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, config)
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
		{
			name: "apply invalid option",
			options: []Option{
				WithAnotherLog(nil),
			},
			expectError: true,
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
			errorMsg:    "logger cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
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

func TestWithViper(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectError bool
	}{
		{
			name: "valid viper config for HuggingFace",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("model_name", "test-model")
				v.Set("local_path", "/test/path")
				v.Set("model_framework", "huggingface")
				v.Set("model_type", "base")
				return v
			},
			expectError: false,
		},
		{
			name: "valid viper config for TensorRTLLM",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("model_name", "test-model")
				v.Set("local_path", "/test/path")
				// Set both enum values with correct case to match the constants
				v.Set("model_framework", string(TensorRTLLM))
				v.Set("model_type", string(constants.ServingBaseModel))
				v.Set("node_shape_alias", "test-shape")
				v.Set("tensorrtllm_version", "1.0.0")
				v.Set("num_of_gpu", "4")
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

				// Check values loaded from viper
				if v.GetString("model_name") != "" {
					assert.Equal(t, v.GetString("model_name"), config.ModelName)
				}

				if v.GetString("model_framework") == string(TensorRTLLM) && v.GetString("model_type") == string(constants.ServingBaseModel) {
					assert.NotNil(t, config.TensorrtLLMConfig)
					assert.Equal(t, v.GetString("tensorrtllm_version"), config.TensorrtLLMConfig.TensorrtLlmVersion)
					assert.Equal(t, v.GetString("node_shape_alias"), config.TensorrtLLMConfig.NodeShapeAlias)
					assert.Equal(t, v.GetString("num_of_gpu"), config.TensorrtLLMConfig.NumOfGpu)
				}
			}
		})
	}
}

func TestWithAppParams(t *testing.T) {
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

	config := &Config{}
	err := WithAppParams(params)(config)

	assert.NoError(t, err)
	assert.Equal(t, mockKmsCrypto, config.KmsCryptoClient)
	assert.Equal(t, mockKmsMgm, config.KmsManagement)
	assert.Equal(t, mockSecret, config.OCISecret)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *Config
		expectError bool
	}{
		{
			name: "valid config with model decryption enabled",
			setupConfig: func() *Config {
				return &Config{
					ModelName:              "test-model",
					LocalPath:              "/test/path",
					ModelFramework:         HuggingFace,
					ModelType:              constants.ServingBaseModel,
					DisableModelDecryption: false,
					KmsCryptoClient:        &kmscrypto.KmsCrypto{},
					KmsManagement:          &kmsmgm.KmsMgm{},
					OCISecret:              &ocisecret.Secret{},
				}
			},
			expectError: false,
		},
		{
			name: "valid config with model decryption disabled",
			setupConfig: func() *Config {
				return &Config{
					ModelName:              "test-model",
					LocalPath:              "/test/path",
					ModelFramework:         HuggingFace,
					ModelType:              constants.ServingBaseModel,
					DisableModelDecryption: true,
				}
			},
			expectError: false,
		},
		{
			name: "invalid config with missing KmsCryptoClient",
			setupConfig: func() *Config {
				return &Config{
					ModelName:              "test-model",
					LocalPath:              "/test/path",
					ModelFramework:         HuggingFace,
					ModelType:              constants.ServingBaseModel,
					DisableModelDecryption: false,
					KmsManagement:          &kmsmgm.KmsMgm{},
					OCISecret:              &ocisecret.Secret{},
				}
			},
			expectError: true,
		},
		{
			name: "invalid config with missing KmsManagement",
			setupConfig: func() *Config {
				return &Config{
					ModelName:              "test-model",
					LocalPath:              "/test/path",
					ModelFramework:         HuggingFace,
					ModelType:              constants.ServingBaseModel,
					DisableModelDecryption: false,
					KmsCryptoClient:        &kmscrypto.KmsCrypto{},
					OCISecret:              &ocisecret.Secret{},
				}
			},
			expectError: true,
		},
		{
			name: "invalid config with missing OCISecret",
			setupConfig: func() *Config {
				return &Config{
					ModelName:              "test-model",
					LocalPath:              "/test/path",
					ModelFramework:         HuggingFace,
					ModelType:              constants.ServingBaseModel,
					DisableModelDecryption: false,
					KmsCryptoClient:        &kmscrypto.KmsCrypto{},
					KmsManagement:          &kmsmgm.KmsMgm{},
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			err := config.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := defaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, HuggingFace, config.ModelFramework)
	assert.Equal(t, false, config.DisableModelDecryption)
	assert.Equal(t, "/tmp/model-storage", config.TempPath)
	assert.NotNil(t, config.KeyMetadata)
	// Compare string values instead of types for OCI SDK enum types
	assert.Equal(t, "AES", string(config.KeyMetadata.Algorithm))
	assert.Equal(t, 32, config.KeyMetadata.Length)
	assert.Equal(t, "HSM", string(config.KeyMetadata.ProtectionModel))
	assert.Equal(t, "ENABLED", string(config.KeyMetadata.LifecycleState))
	assert.Equal(t, false, config.KeyMetadata.EnableDefinedTag)
}
