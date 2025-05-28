package training_agent

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociobjectstore"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type trainingAgentParams struct {
	fx.In

	AnotherLogger       logging.Interface `name:"another_log"`
	CasperDataStoreList []*ociobjectstore.OCIOSDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, params trainingAgentParams) (*TrainingAgent, error) {
		config, err := NewTrainingAgentConfig(
			WithViper(v),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating training agent config: %+v", err)
		}
		return NewTrainingAgent(config)
	})
