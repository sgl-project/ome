package enigma

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/vault/kmscrypto"
	"sigs.k8s.io/ome/pkg/vault/kmsmgm"
	ocisecret "sigs.k8s.io/ome/pkg/vault/secret"
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
