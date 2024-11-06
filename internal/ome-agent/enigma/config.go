package enigma

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"strings"
)

type ModelFramework string

const (
	TensorRTLLM       ModelFramework = "tensorrtllm"
	HuggingFace       ModelFramework = "huggingface"
	FasterTransformer ModelFramework = "fastertransformer"
)

type Config struct {
	AnotherLogger          logging.Interface
	ModelName              string                           `mapstructure:"model_name" validate:"required"`
	ModelStoreDirectory    string                           `mapstructure:"model_store_directory" validate:"required"`
	ModelFramework         ModelFramework                   `mapstructure:"model_framework" validate:"required"`
	TensorrtLLMConfig      *TensorrtLLMConfig               `mapstructure:"tensorrtllm_config"`
	DisableModelDecryption bool                             `mapstructure:"disable_model_decryption"`
	KeyConfig              *secrets.KeyConfig               `mapstructure:"key_config" validate:"required_if=DisableModelDecryption false"`
	SecretConfig           *secrets.SecretConfig            `mapstructure:"secret_config" validate:"required_if=DisableModelDecryption false"`
	CryptoClient           *keymanagement.CryptoClient      `validate:"required_if=DisableModelDecryption false"`
	KmsKeyManager          *keymanagement.KmsKeyManager     `validate:"required_if=DisableModelDecryption false"`
	SecretRetriever        *secretretrieval.SecretRetriever `validate:"required_if=DisableModelDecryption false"`
}

type TensorrtLLMConfig struct {
	TensorrtLlmVersion string `mapstructure:"tensorrt_llm_version"`
	NodeShapeAlias     string `mapstructure:"node_shape_alias"`
	NumOfGpu           string `mapstructure:"num_of_gpu"`
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
		v.AutomaticEnv()
		v.SetEnvPrefix("OME_AGENT")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error unmarshalling config: %w", err)
		}

		if err := populateConfigFields(v, c, logger); err != nil {
			return err
		}

		if c.ModelFramework == TensorRTLLM {
			if err := configureTensorRTLLM(c, v, logger); err != nil {
				return err
			}
		}

		return nil
	}
}

// populateConfigFields populates configuration fields directly from environment variables.
func populateConfigFields(v *viper.Viper, c *Config, logger logging.Interface) error {
	c.ModelName = v.GetString("model_name")
	c.ModelStoreDirectory = v.GetString("model_store_directory")
	c.ModelFramework = ModelFramework(v.GetString("model_framework"))
	c.DisableModelDecryption = v.GetBool("disable_model_decryption")

	if c.ModelName == "" || c.ModelStoreDirectory == "" || c.ModelFramework == "" {
		return errors.New("missing required configuration values")
	}
	return nil
}

// configureTensorRTLLM configures TensorRT LLM-specific settings.
func configureTensorRTLLM(c *Config, v *viper.Viper, logger logging.Interface) error {
	nodeShape, err := GetOCINodeShape(logger)
	if err != nil {
		return fmt.Errorf("failed to get OCI node shape: %w", err)
	}

	nodeShapeAlias, err := GetOCINodeShortVersionShape(nodeShape)
	if err != nil {
		return fmt.Errorf("failed to get short version shape for node: %w", err)
	}

	c.TensorrtLLMConfig = &TensorrtLLMConfig{
		NodeShapeAlias:     nodeShapeAlias,
		TensorrtLlmVersion: v.GetString("tensorrt_llm_version"),
		NumOfGpu:           v.GetString("num_of_gpu"),
	}

	return nil
}

// WithAppParams applies configuration parameters from Enigma-specific params.
func WithAppParams(params enigmaParams) Option {
	return func(c *Config) error {
		c.CryptoClient = params.CryptoClient
		c.KmsKeyManager = params.KmsKeyManager
		c.SecretRetriever = params.SecretRetriever
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
