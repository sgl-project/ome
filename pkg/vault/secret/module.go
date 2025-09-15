package ocisecret

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
)

type appParams struct {
	fx.In

	AnotherLogger logging.Interface `name:"another_log"`
}

var Module = fx.Provide(
	func(v *viper.Viper, params appParams) (*Secret, error) {
		config, err := NewConfig(
			WithViper(v, params.AnotherLogger),
			WithParams(params),
			WithAnotherLogger(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating secret config: %+v", err)
		}
		return NewSecret(config)
	})

type secretClientParamsWithListOfConfigs struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*Config `group:"secretClientConfigs"`
}

func ProvideListOfSecretRetrievalWithAppParams(params secretClientParamsWithListOfConfigs) ([]*Secret, error) {
	secretList := make([]*Secret, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		secretRetrieval, err := NewSecret(config)
		if err != nil {
			return secretList, fmt.Errorf("error initializing Secret using Config: %+v: %+v", config, err)
		}
		secretList = append(secretList, secretRetrieval)
	}
	return secretList, nil
}
