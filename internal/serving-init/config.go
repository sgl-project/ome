package serving_init

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/spf13/viper"
)

// Config represents a struct to hold all configs for serving init
type Config struct {
	AnotherLogger logging.Interface

	ModelName              string                `mapstructure:"model_name" validate:"required"`
	ModelStoreDirectory    string                `mapstructure:"model_store_directory" validate:"required"`
	LocalStoreDirectory    string                `mapstructure:"local_store_directory" validate:"required"`
	NodeShapeMappingPath   string                `mapstructure:"node_shape_mapping_path"`
	DisableModelDecryption bool                  `mapstructure:"disable_model_decryption"`
	IsTensorrtModel        bool                  `mapstructure:"is_tensorrt_model"`
	TensorrtConfig         *TensorrtConfig       `mapstructure:"tensorrt_config" validate:"required_if=IsTensorrtModel true"`
	KeyConfig              *secrets.KeyConfig    `mapstructure:"key_config" validate:"required_if=DisableModelDecryption false"`
	SecretConfig           *secrets.SecretConfig `mapstructure:"secret_config" validate:"required_if=DisableModelDecryption false"`

	// Injected client objects from DI
	KmsCrypto       *keymanagement.KmsCrypto         `validate:"required_if=DisableModelDecryption false"`
	KmsManagement   *keymanagement.KmsManagement     `validate:"required_if=DisableModelDecryption false"`
	SecretRetrieval *secretretrieval.SecretRetrieval `validate:"required_if=DisableModelDecryption false"`
}

type TensorrtConfig struct {
	TensorrtLlmVersion string `mapstructure:"tensorrt_llm_version" json:"tensorrt_llm_version"`
	NodeShapeShort     string `mapstructure:"node_shape_short" json:"node_shape_short"`
	NumOfGpu           string `mapstructure:"num_of_gpu" json:"num_of_gpu"`
}

// Apply applies the given options to the configuration.
func (c *Config) Apply(opts ...Option) error {
	for _, o := range opts {
		if o == nil {
			continue
		}

		if err := o(c); err != nil {
			return err
		}
	}
	return nil
}

// NewConfig builds and returns a new configuration from the given options.
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// Option represents a server configuration option.
type Option func(*Config) error

// WithAnotherLog specifies the csr logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *Config) error {
		return nil
	}
}

func WithAppParams(params appParams) Option {
	return func(c *Config) error {
		c.KmsCrypto = params.KmsCrypto
		c.KmsManagement = params.KmsManagement
		c.SecretRetrieval = params.SecretInVault
		return nil
	}
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper, logger logging.Interface) Option {
	return func(c *Config) error {
		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Special set up for secret name
		if !c.DisableModelDecryption &&
			(c.SecretConfig.SecretName == nil || len(*c.SecretConfig.SecretName) == 0) {
			c.SecretConfig.SecretName = common.String(fmt.Sprintf("%s-dek", c.KeyConfig.Name))
		}

		// Special set up for node shape short
		if c.IsTensorrtModel {
			mappingPath := c.NodeShapeMappingPath
			nodeShape, err := GetCurrentNodeShape(logger)
			if err != nil {
				return err
			}
			nodeShapeShort, err := GetCurrentNodeShortVersionShape(nodeShape, mappingPath)
			if err != nil {
				return err
			}

			c.TensorrtConfig.NodeShapeShort = nodeShapeShort
		}

		return nil
	}
}

// Validate ensures the app Config is valid.
func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
