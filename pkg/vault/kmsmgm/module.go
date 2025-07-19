package kmsmgm

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/vault/kmsvault"
)

type appParams struct {
	fx.In

	AnotherLogger  logging.Interface `name:"another_log"`
	KmsVaultClient *kmsvault.KMSVault
}

var Module = fx.Provide(
	func(v *viper.Viper, params appParams) (*KmsMgm, error) {
		config, err := NewConfig(
			WithViper(v, params.AnotherLogger),
			WithAppParams(params),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating kms management config: %+v", err)
		}
		return NewKmsMgm(config)
	})
