package training_agent

import (
	"fmt"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/sgl-project/sgl-ome/pkg/casper"
	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
)

const (
	InputCasperConfigName  = "input"
	OutputCasperConfigName = "output"
)

type Config struct {
	AnotherLogger logging.Interface

	Runtime                       constants.TrainingSidecarRuntime `mapstructure:"runtime" validate:"required"`
	TrainingName                  string                           `mapstructure:"training_name" validate:"required"`
	ModelDirectory                string                           `mapstructure:"model_directory" validate:"required"`
	ZippedModelPath               string                           `validate:"required"`
	ZippedMergedModelPath         string
	TrainingDataStoreDirectory    string                 `mapstructure:"training_data_directory" validate:"required"`
	TrainingDataObjectStoreURI    *casper.ObjectURI      `mapstructure:"training_data" validate:"required"`
	ModelObjectStoreURI           *casper.ObjectURI      `mapstructure:"model" validate:"required"`
	TrainingMetricsObjectStoreURI *casper.ObjectURI      `mapstructure:"training_metrics" validate:"required"`
	CohereFineTuneDetails         *CohereFineTuneDetails `mapstructure:"cohere_ft"`
	PeftFineTuneDetails           *PeftFineTuneDetails   `mapstructure:"peft_ft"`

	InputObjectStorageDataStore  *casper.CasperDataStore `mapstructure:"input_object_store" validate:"required"`
	OutputObjectStorageDataStore *casper.CasperDataStore `validate:"required"`
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

// NewTrainingAgentConfig builds and returns a new configuration from the given options.
func NewTrainingAgentConfig(opts ...Option) (*Config, error) {
	c := &Config{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithAppParams attempts to resolve the required client objects using injected named parameters
func WithAppParams(params trainingAgentParams) Option {
	return func(c *Config) error {
		for _, casperDataStore := range params.CasperDataStoreList {
			if casperDataStore.Config.Name == InputCasperConfigName {
				c.InputObjectStorageDataStore = casperDataStore
			}
			if casperDataStore.Config.Name == OutputCasperConfigName {
				c.OutputObjectStorageDataStore = casperDataStore
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
		if err := configutils.BindEnvsRecursive(v, c, ""); err != nil {
			return fmt.Errorf("error occurred when binding environment variables: %+v", err)
		}

		// Unmarshal the viper configuration into Config struct
		if err := v.Unmarshal(c); err != nil {
			return fmt.Errorf("error occurred when unmarshalling config: %+v", err)
		}

		// Set extra config variables
		setZippedModelPath(c)
		c.ZippedMergedModelPath = c.ZippedModelPath + constants.MergedModelWeightZippedFileSuffix

		return nil
	}
}

func setZippedModelPath(c *Config) {
	switch c.Runtime {
	case constants.CohereCommand1TrainingSidecar, constants.CohereCommandRTrainingSidecar:
		c.ZippedModelPath = filepath.Join(constants.CohereStorePathPrefix, c.TrainingName, c.TrainingName)
	case constants.PeftTrainingSidecar:
		c.ZippedModelPath = filepath.Join(constants.TrainingDataEmptyDirMountPath, constants.PeftTrainingOutputModelDirectoryName, c.TrainingName)
	default:
		c.ZippedModelPath = filepath.Join(constants.DefaultTrainingZippedModelDirectory, c.TrainingName)
	}
}

func (c *Config) Validate() error {
	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}
