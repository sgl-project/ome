package ocisecret

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
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
