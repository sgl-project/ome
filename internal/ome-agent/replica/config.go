package replica

import (
	"fmt"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"

	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/configutils"
	hf "github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

type Config struct {
	AnotherLogger logging.Interface

	LocalPath            string `mapstructure:"local_path" validate:"required"`
	DownloadSizeLimitGB  int    `mapstructure:"download_size_limit_gb"`
	EnableSizeLimitCheck bool   `mapstructure:"enable_size_limit_check"`
	NumConnections       int    `mapstructure:"num_connections"`

	Source struct {
		StorageURIStr  string `mapstructure:"storage_uri" validate:"required"`
		OCIOSDataStore *ociobjectstore.OCIOSDataStore
		HubClient      *hf.HubClient
		PVCFileSystem  *afero.OsFs
	} `mapstructure:"source"`

	Target struct {
		StorageURIStr  string `mapstructure:"storage_uri" validate:"required"`
		OCIOSDataStore *ociobjectstore.OCIOSDataStore
		PVCFileSystem  *afero.OsFs
	} `mapstructure:"target"`
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
		for _, dataStore := range params.OCIOSDataStoreList {
			if dataStore.Config.Name == SourceStorageConfigKeyName {
				c.Source.OCIOSDataStore = dataStore
			}
			if dataStore.Config.Name == TargetStorageConfigKeyName {
				c.Target.OCIOSDataStore = dataStore
			}
		}

		c.Source.HubClient = params.HubClient
		c.Source.PVCFileSystem = params.SourcePVCFileSystem
		c.Target.PVCFileSystem = params.TargetPVCFileSystem
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

		return nil
	}
}

func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}

	if err := storage.ValidateStorageURI(c.Source.StorageURIStr); err != nil {
		return fmt.Errorf("invalid source storage URI %s - %w ", c.Source.StorageURIStr, err)
	}
	if err := storage.ValidateStorageURI(c.Target.StorageURIStr); err != nil {
		return fmt.Errorf("invalid target storage URI %s - %w", c.Target.StorageURIStr, err)
	}
	return nil
}

func (c *Config) ValidateRequiredDependencies(sourceStorageType storage.StorageType, targetStorageType storage.StorageType) error {
	// Validate source dependencies
	switch sourceStorageType {
	case storage.StorageTypeOCI:
		if err := common.RequireNonNil("Source.OCIOSDataStore", c.Source.OCIOSDataStore); err != nil {
			return err
		}
	case storage.StorageTypeHuggingFace:
		if err := common.RequireNonNil("Source.HubClient", c.Source.HubClient); err != nil {
			return err
		}
	case storage.StorageTypePVC:
		if err := common.RequireNonNil("Source.PVCFileSystem", c.Source.PVCFileSystem); err != nil {
			return err
		}
	}

	// Validate target dependencies
	switch targetStorageType {
	case storage.StorageTypeOCI:
		if err := common.RequireNonNil("Target.OCIOSDataStore", c.Target.OCIOSDataStore); err != nil {
			return err
		}
	case storage.StorageTypePVC:
		if err := common.RequireNonNil("Target.PVCFileSystem", c.Target.PVCFileSystem); err != nil {
			return err
		}
	}
	return nil
}
