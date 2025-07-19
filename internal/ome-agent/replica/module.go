package replica

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

type replicaParams struct {
	fx.In

	AnotherLogger      logging.Interface                `name:"another_log"`
	OCIOSDataStoreList []*ociobjectstore.OCIOSDataStore `optional:"true"`
	HubClient          *hub.HubClient                   `optional:"true"`
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

		if err = config.Validate(); err != nil {
			return nil, fmt.Errorf("error validating replica config: %+v", err)
		}
		return NewReplicaAgent(config)
	})
