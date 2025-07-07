package replica

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/ome/pkg/configutils"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/spf13/viper"
)

type Config struct {
	AnotherLogger logging.Interface

	LocalPath                    string                         `mapstructure:"local_path" validate:"required"`
	DownloadSizeLimitGB          int                            `mapstructure:"download_size_limit_gb"`
	EnableSizeLimitCheck         bool                           `mapstructure:"enable_size_limit_check"`
	NumConnections               int                            `mapstructure:"num_connections"`
	SourceObjectStoreURI         ociobjectstore.ObjectURI       `mapstructure:"source" validate:"required"`
	TargetObjectStoreURI         ociobjectstore.ObjectURI       `mapstructure:"target" validate:"required"`
	SourceObjectStorageDataStore *ociobjectstore.OCIOSDataStore `validate:"required"`
	TargetObjectStorageDataStore *ociobjectstore.OCIOSDataStore `validate:"required"`
}

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

// defaultConfig returns a new configuration with default values.
func defaultConfig() *Config {
	return &Config{
		NumConnections:       10,
		DownloadSizeLimitGB:  650,
		EnableSizeLimitCheck: true,
	}
}

// NewReplicaConfig builds and returns a new configuration from the given options.
func NewReplicaConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params replicaParams) Option {
	return func(c *Config) error {
		for _, casperDataStore := range params.ObjectStorageDataStores {
			if casperDataStore == nil || casperDataStore.Config == nil {
				continue
			}
			switch casperDataStore.Config.Name {
			case ociobjectstore.SourceOsConfigName:
				if c.SourceObjectStorageDataStore != nil {
					return fmt.Errorf("duplicate source object storage data store provided")
				}
				c.SourceObjectStorageDataStore = casperDataStore
			case ociobjectstore.TargetOsConfigName:
				if c.TargetObjectStorageDataStore != nil {
					return fmt.Errorf("duplicate target object storage data store provided")
				}
				c.TargetObjectStorageDataStore = casperDataStore
			}
		}
		return nil
	}
}

// WithAnotherLog sets the logger for the configuration.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *Config) error {
		c.AnotherLogger = logger
		return nil
	}
}

// WithViper sets the viper for the configuration.
func WithViper(v *viper.Viper) Option {
	return func(c *Config) error {

		*c = *defaultConfig()
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error occurred when binding environment variables: %+v", err)
		}

		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Ensure that the prefix of the object store URIs end with a slash
		if len(c.SourceObjectStoreURI.Prefix) > 0 && !strings.HasSuffix(c.SourceObjectStoreURI.Prefix, "/") {
			c.SourceObjectStoreURI.Prefix = c.SourceObjectStoreURI.Prefix + "/"
		}
		if len(c.TargetObjectStoreURI.Prefix) > 0 && !strings.HasSuffix(c.TargetObjectStoreURI.Prefix, "/") {
			c.TargetObjectStoreURI.Prefix = c.TargetObjectStoreURI.Prefix + "/"
		}

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
