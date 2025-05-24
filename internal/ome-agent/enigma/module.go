package enigma

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmscrypto"
	"github.com/sgl-project/sgl-ome/pkg/vault/kmsmgm"
	ocisecret "github.com/sgl-project/sgl-ome/pkg/vault/secret"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type enigmaParams struct {
	fx.In

	AnotherLogger   logging.Interface `name:"another_log"`
	KmsCryptoClient *kmscrypto.KmsCrypto
	KmsManagement   *kmsmgm.KmsMgm
	Secret          *ocisecret.Secret
}

var Module = fx.Provide(
	func(v *viper.Viper, params enigmaParams) (*Enigma, error) {
		config, err := NewConfig(
			WithViper(v, params.AnotherLogger),
			WithAppParams(params),
			WithAnotherLog(params.AnotherLogger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating enigma config: %+v", err)
		}
		return NewApplication(config)
	})
