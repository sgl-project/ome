package kmsvault

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type kmsVault struct {
	fx.In

	AnotherLogger logging.Interface `name:"another_log"`
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params kmsVault) (*KMSVault, error) {
		config, err := NewConfig(
			WithViper(v),
			WithEnv(e),
			WithAnotherLogger(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating kms vault agent config: %+v", err)
		}
		return NewKMSVault(config, e)
	})
