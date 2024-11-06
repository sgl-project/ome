package partner_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretinvault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_in_vault"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/spf13/viper"
)

type DownloadAgentMode string

const (
	ModelReplication DownloadAgentMode = "ModelReplication"
	ModelImporting   DownloadAgentMode = "ModelImporting"
)

type ModelPathConfig struct {
	ModelPath       string `json:"model_path"`
	ModelObjectName string `json:"partner_model_object_name"`
}

type Config struct {
	AnotherLogger logging.Interface

	DownloadAgentMode    *DownloadAgentMode `mapstructure:"mode" validate:"required"`
	ModelName            string             `mapstructure:"model_name" validate:"required"`
	TempModelStorePath   string             `mapstructure:"temp_model_store_path" validate:"required"`
	SourceModelEncrypted bool               `mapstructure:"source_model_encrypted"`
	ModelPathConfigs     []*ModelPathConfig //cannot decode this field directly using `mapstructure` tag

	SourceObjectStoreURI *casper.ObjectURI     `mapstructure:"source_object_store_uri" validate:"required"`
	SourceKeyConfig      *secrets.KeyConfig    `mapstructure:"source_key_config" validate:"required_if=SourceModelEncrypted true"`
	SourceSecretConfig   *secrets.SecretConfig `mapstructure:"source_secret_config" validate:"required_if=SourceModelEncrypted true"`
	TargetObjectStoreURI *casper.ObjectURI     `mapstructure:"target_object_store_uri" validate:"required"`
	TargetKeyConfig      *secrets.KeyConfig    `mapstructure:"target_key_config" validate:"required"`
	TargetSecretConfig   *secrets.SecretConfig `mapstructure:"target_secret_config" validate:"required"`

	// Injected client objects
	SourceCasperDataStore *casper.CasperDataStore          `validate:"required"`
	SourceKmsCrypto       *keymanagement.CryptoClient      `validate:"required_if=SourceModelEncrypted true"`
	SourceKmsManagement   *keymanagement.KmsKeyManager     `validate:"required_if=SourceModelEncrypted true"`
	SourceSecretRetrieval *secretretrieval.SecretRetriever `validate:"required_if=SourceModelEncrypted true"`
	TargetCasperDataStore *casper.CasperDataStore          `validate:"required"`
	TargetKmsCrypto       *keymanagement.CryptoClient      `validate:"required"`
	TargetKmsManagement   *keymanagement.KmsKeyManager     `validate:"required"`
	TargetSecretRetrieval *secretretrieval.SecretRetriever `validate:"required"`
	SecretInVault         *secretinvault.SecretInVault     `validate:"required"`
}

// Option represents a server configuration option.
type Option func(*Config) error

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

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *Config) error {
		return nil
	}
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params appParams) Option {
	return func(c *Config) error {
		for _, casperDataStore := range params.CasperDataStoreList {
			if casperDataStore.Config.Name == "SOURCE" {
				c.SourceCasperDataStore = casperDataStore
			}
			if casperDataStore.Config.Name == "TARGET" {
				c.TargetCasperDataStore = casperDataStore
			}
		}

		for _, kmsCrypto := range params.KmsCryptoList {
			if kmsCrypto.Config.Name == "SOURCE" {
				c.SourceKmsCrypto = kmsCrypto
			}
			if kmsCrypto.Config.Name == "TARGET" {
				c.TargetKmsCrypto = kmsCrypto
			}
		}

		for _, kmsManagement := range params.KmsManagementList {
			if kmsManagement.Config.Name == "SOURCE" {
				c.SourceKmsManagement = kmsManagement
			}
			if kmsManagement.Config.Name == "TARGET" {
				c.TargetKmsManagement = kmsManagement
			}
		}

		for _, secretRetrieval := range params.SecretRetrievalList {
			if secretRetrieval.Config.Name == "SOURCE" {
				c.SourceSecretRetrieval = secretRetrieval
			}
			if secretRetrieval.Config.Name == "TARGET" {
				c.TargetSecretRetrieval = secretRetrieval
			}
		}

		c.SecretInVault = params.SecretInVault
		return nil
	}
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {
		// Unmarshal the viper configuration into Config struct
		if err := viper.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Special handle for the secret name variable
		if c.SourceModelEncrypted && c.SourceSecretConfig.SecretName == nil {
			c.SourceSecretConfig.SecretName = common.String(fmt.Sprintf("%s-dek", c.SourceKeyConfig.Name))
		}
		if c.TargetSecretConfig.SecretName == nil {
			c.TargetSecretConfig.SecretName = common.String(fmt.Sprintf("%s-dek", c.TargetKeyConfig.Name))
		}

		// Set up a list of model path configs
		modelPathConfigJson := v.GetString("model_path_config_json")
		var modelPathConfigs []*ModelPathConfig
		if err := json.Unmarshal([]byte(modelPathConfigJson), &modelPathConfigs); err != nil {
			return fmt.Errorf("error occurred when unmarshalling model path config: %+v", err)
		}
		c.ModelPathConfigs = modelPathConfigs
		return nil
	}
}

// WithAnotherLog specifies the logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
