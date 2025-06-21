package serving_agent

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type servingSidecarParams struct {
	fx.In

	AnotherLogger           logging.Interface `name:"another_log"`
	ObjectStorageDataStores *ociobjectstore.OCIOSDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, params servingSidecarParams) (*ServingSidecar, error) {
		config, err := NewServingSidecarConfig(
			WithViper(v),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating serving sidecar config: %+v", err)
		}
		return NewServingSidecar(config)
	})
