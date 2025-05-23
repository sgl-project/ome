package kmsvault

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type kmsVault struct {
	fx.In

	AnotherLogger logging.Interface `name:"another_log"`
}

var Module = fx.Provide(
	func(v *viper.Viper, params kmsVault) (*KMSVault, error) {
		config, err := NewConfig(
			WithViper(v),
			WithAnotherLogger(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating kms vault agent config: %+v", err)
		}
		return NewKMSVault(config)
	})
