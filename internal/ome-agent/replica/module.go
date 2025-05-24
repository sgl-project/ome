package replica

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/casper"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type replicaParams struct {
	fx.In

	AnotherLogger           logging.Interface `name:"another_log"`
	ObjectStorageDataStores *casper.CasperDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, params replicaParams) (*ReplicaAgent, error) {
		config, err := NewReplicaConfig(
			WithViper(v),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating replica config: %+v", err)
		}
		return NewReplicaAgent(config)
	})
