package kmsvault

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
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
