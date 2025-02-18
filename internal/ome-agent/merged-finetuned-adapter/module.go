package merged_finetuned_adapter

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type mergedFinetunedAdapterParams struct {
	fx.In

	AnotherLogger           logging.Interface `name:"another_log"`
	ObjectStorageDataStores *casper.CasperDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params mergedFinetunedAdapterParams) (*MergedFinetunedAdapter, error) {
		config, err := NewMergedFinetunedAdapterConfig(
			WithViper(v),
			WithEnv(e),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating serving sidecar config: %+v", err)
		}
		return NewMergedFinetunedAdapter(config)
	})
