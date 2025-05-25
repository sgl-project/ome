package replica

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/casper"
	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
)

type Config struct {
	AnotherLogger logging.Interface

	LocalPath              string                  `mapstructure:"local_path" validate:"required"`
	DownloadSizeLimitGB    int                     `mapstructure:"download_size_limit_gb"`
	EnableSizeLimitCheck   bool                    `mapstructure:"enable_size_limit_check"`
	NumConnections         int                     `mapstructure:"num_connections"`
	SourceObjectStoreURI   casper.ObjectURI        `mapstructure:"source" validate:"required"`
	TargetObjectStoreURI   casper.ObjectURI        `mapstructure:"target" validate:"required"`
	ObjectStorageDataStore *casper.CasperDataStore `validate:"required"`
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
		c.ObjectStorageDataStore = params.ObjectStorageDataStores
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
