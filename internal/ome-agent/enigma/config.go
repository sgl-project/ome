package enigma

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	utils "github.com/sgl-project/sgl-ome/pkg/utils"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmscrypto"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmsmgm"
	ocisecret "github.com/sgl-project/sgl-ome/pkg/vault/secret"
	"github.com/spf13/viper"
)

type ModelFramework string

const (
	TensorRTLLM       ModelFramework = "tensorrtllm"
	HuggingFace       ModelFramework = "huggingface"
	FasterTransformer ModelFramework = "fastertransformer"
)

type Config struct {
	AnotherLogger          logging.Interface
	ModelName              string                  `mapstructure:"model_name"`
	LocalPath              string                  `mapstructure:"local_path"`
	ModelFramework         ModelFramework          `mapstructure:"model_framework"`
	TensorrtLLMConfig      *TensorrtLLMConfig      `mapstructure:"tensorrtllm_config"`
	DisableModelDecryption bool                    `mapstructure:"disable_model_decryption"`
	TempPath               string                  `mapstructure:"model_store_directory"`
	VaultId                string                  `mapstructure:"vault_id"`
	SecretName             string                  `mapstructure:"secret_name"`
	ModelType              constants.BaseModelType `mapstructure:"model_type"`
	KeyMetadata            *kmsmgm.KeyMetadata
	KmsCryptoClient        *kmscrypto.KmsCrypto `validate:"required_if=DisableModelDecryption false"`
	KmsManagement          *kmsmgm.KmsMgm       `validate:"required_if=DisableModelDecryption false"`
	OCISecret              *ocisecret.Secret    `validate:"required_if=DisableModelDecryption false"`
}

type TensorrtLLMConfig struct {
	TensorrtLlmVersion string `mapstructure:"tensorrtllm_version"`
	NodeShapeAlias     string `mapstructure:"node_shape_alias"`
	NumOfGpu           string `mapstructure:"num_of_gpu"`
}

func defaultConfig() *Config {
	return &Config{
		ModelFramework:         HuggingFace,
		DisableModelDecryption: false,
		TempPath:               "/tmp/model-storage",
		KeyMetadata: &kmsmgm.KeyMetadata{
			Algorithm:        "AES",
			Length:           32,
			ProtectionModel:  "HSM",
			LifecycleState:   "ENABLED",
			EnableDefinedTag: false,
		},
	}

}

// NewConfig builds and returns a new configuration from the given options.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}
	return c, nil
}

// Apply applies the given options to the configuration.
func (c *Config) Apply(opts ...Option) error {
	for _, o := range opts {
		if o != nil {
			if err := o(c); err != nil {
				return err
			}
		}
	}
	return nil
}

// Option represents a configuration option for the server.
type Option func(*Config) error

// WithAnotherLog sets an alternative logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("logger cannot be nil")
		}
		c.AnotherLogger = logger
		return nil
	}
}

// WithViper loads configuration using Viper.
func WithViper(v *viper.Viper, logger logging.Interface) Option {
	return func(c *Config) error {

		*c = *defaultConfig()
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error binding envs: %w", err)
		}

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error unmarshalling config: %w", err)
		}

		if err := v.Unmarshal(c.KeyMetadata); err != nil {
			return fmt.Errorf("error unmarshalling key metadata: %w", err)
		}

		if c.ModelFramework == TensorRTLLM && c.ModelType == constants.ServingBaseModel {
			if err := configureTensorRTLLM(c, v, logger); err != nil {
				return err
			}
		}

		return nil
	}
}

// configureTensorRTLLM configures TensorRT LLM-specific settings.
func configureTensorRTLLM(c *Config, v *viper.Viper, logger logging.Interface) error {
	var nodeShapeAlias string
	if v.GetString("node_shape_alias") == "" {
		nodeShape, err := utils.GetOCINodeShape(logger)
		if err != nil {
			return fmt.Errorf("failed to get OCI node shape: %w", err)
		}

		nodeShapeAlias, err = utils.GetOCINodeShortVersionShape(nodeShape)
		if err != nil {
			return fmt.Errorf("failed to get short version shape for node: %w", err)
		}
	} else {
		nodeShapeAlias = v.GetString("node_shape_alias")
	}

	c.TensorrtLLMConfig = &TensorrtLLMConfig{
		NodeShapeAlias:     nodeShapeAlias,
		TensorrtLlmVersion: v.GetString("tensorrtllm_version"),
		NumOfGpu:           v.GetString("num_of_gpu"),
	}

	return nil
}

// WithAppParams applies configuration parameters from Enigma-specific params.
func WithAppParams(params enigmaParams) Option {
	return func(c *Config) error {
		c.OCISecret = params.Secret
		c.KmsCryptoClient = params.KmsCryptoClient
		c.KmsManagement = params.KmsManagement
		return nil
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}
