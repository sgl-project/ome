package fine_tuned_adapter

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type fineTunedAdapterParams struct {
	fx.In

	AnotherLogger           logging.Interface `name:"another_log"`
	ObjectStorageDataStores *ociobjectstore.OCIOSDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, params fineTunedAdapterParams) (*FineTunedAdapter, error) {
		config, err := NewFineTunedAdapterConfig(
			WithViper(v),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating fine-tuned adapter config: %+v", err)
		}
		return NewFineTunedAdapter(config)
	})
