package generic_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"strconv"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

type GenericConfig struct {
	AnotherLogger logging.Interface

	ModelName                     string            `mapstructure:"model_name" validate:"required"`
	TempModelStorePath            string            `mapstructure:"temp_model_store_path" validate:"required"`
	EnableSizeLimitCheck          bool              `mapstructure:"enable_size_limit_check"`
	DownloadSizeLimitGB           int               `mapstructure:"download_size_limit_gb"`
	NumberOfThreadsForReplication int               `mapstructure:"number_of_threads_for_replication" validate:"gt=0"`
	SourceObjectStoreURI          *casper.ObjectURI `mapstructure:"source_object_store_uri" validate:"required"`
	TargetObjectStoreURI          *casper.ObjectURI `mapstructure:"target_object_store_uri" validate:"required"`

	// Injected client objects from DI
	SourceCasperDataStore *casper.CasperDataStore `validate:"required"`
	TargetCasperDataStore *casper.CasperDataStore `validate:"required"`
}

// Option represents a server configuration option.
type Option func(*GenericConfig) error

// Apply applies the given options to the configuration.
func (c *GenericConfig) Apply(opts ...Option) error {
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

// NewGenericConfig builds and returns a new configuration from the given options.
func NewGenericConfig(opts ...Option) (*GenericConfig, error) {
	c := &GenericConfig{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params appParams) Option {
	return func(c *GenericConfig) error {
		for _, casperDataStore := range params.CasperDataStoreList {
			if casperDataStore.Config.Name == "SOURCE" {
				c.SourceCasperDataStore = casperDataStore
			}
			if casperDataStore.Config.Name == "TARGET" {
				c.TargetCasperDataStore = casperDataStore
			}
		}
		return nil
	}
}

// WithAnotherLog specifies the csr logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *GenericConfig) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *GenericConfig) error {
		return nil
	}
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper) Option {
	return func(c *GenericConfig) error {
		// Check if the fields are required type
		if _, err := strconv.Atoi(v.GetString("download_size_limit_gb")); err != nil {
			return fmt.Errorf("error occurred when converting the download_size_limit_gb to integer: %v", err)
		}
		if _, err := strconv.Atoi(v.GetString("number_of_threads_for_replication")); err != nil {
			return fmt.Errorf("error occurred when converting the number_of_threads_for_replication to integer: %v", err)
		}

		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Append `/` to prefix
		if len(c.SourceObjectStoreURI.Prefix) > 0 && !strings.HasSuffix(c.SourceObjectStoreURI.Prefix, "/") {
			c.SourceObjectStoreURI.Prefix = c.SourceObjectStoreURI.Prefix + "/"
		}
		if len(c.TargetObjectStoreURI.Prefix) > 0 && !strings.HasSuffix(c.TargetObjectStoreURI.Prefix, "/") {
			c.TargetObjectStoreURI.Prefix = c.TargetObjectStoreURI.Prefix + "/"
		}

		return nil
	}
}

func (c *GenericConfig) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
