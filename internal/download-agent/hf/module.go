package hf_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	CasperDataStore *casper.CasperDataStore
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params appParams) (*HFDownloadAgent, error) {
		config, err := NewHFConfig(
			WithAppParams(params),
			WithViper(v),
			WithEnv(e),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating HF download agent config: %+v", err)
		}
		return NewHFDownloadAgent(config)
	})
