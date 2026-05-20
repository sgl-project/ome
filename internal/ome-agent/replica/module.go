package replica

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/xet"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"sigs.k8s.io/ome/pkg/afero"
	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

type replicaParams struct {
	fx.In

	AnotherLogger       logging.Interface                `name:"another_log"`
	OCIOSDataStoreList  []*ociobjectstore.OCIOSDataStore `optional:"true"`
	HubClient           *xet.Client                      `optional:"true"`
	SourcePVCFileSystem *afero.OsFs                      `name:"source_pvc_fs" optional:"true"`
	TargetPVCFileSystem *afero.OsFs                      `name:"target_pvc_fs" optional:"true"`
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
