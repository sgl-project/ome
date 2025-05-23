package download

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type downloadAgentParams struct {
	fx.In

	// this is an example on how to inject logger
	AnotherLogger logging.Interface `name:"another_log"`
}

var Module = fx.Provide(
	func(v *viper.Viper, params downloadAgentParams) (*HFDownloadAgent, error) {
		config, err := NewConfig(
			WithViper(v),
			WithLogger(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating hf_download agent config: %+v", err)
		}
		return NewHFDownloadAgent(config)
	})
