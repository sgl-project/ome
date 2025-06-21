package kmscrypto

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/vault/kmsvault"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type kmsCryptoParams struct {
	fx.In

	AnotherLogger  logging.Interface `name:"another_log"`
	KmsVaultClient *kmsvault.KMSVault
}

var Module = fx.Provide(
	func(v *viper.Viper, params kmsCryptoParams) (*KmsCrypto, error) {
		config, err := NewConfig(
			WithViper(v, params.AnotherLogger),
			WithAppParams(params),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating kms crypto config: %+v", err)
		}
		return NewKmsCrypto(config)
	})
