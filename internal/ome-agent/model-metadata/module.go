package modelmetadata

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/logging"
)

type metadataParams struct {
	fx.In

	Logger logging.Interface
	Fs     afero.Fs
	Client client.Client
	Viper  *viper.Viper
}

// Module provides the model metadata extractor via fx
var Module = fx.Provide(
	func(params metadataParams) (*MetadataExtractor, error) {
		config, err := NewConfig(
			WithViper(params.Viper),
			WithLogger(params.Logger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating model metadata config: %w", err)
		}

		// Validate configuration
		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid model metadata config: %w", err)
		}

		return NewMetadataExtractor(config, params.Fs, params.Client)
	})
