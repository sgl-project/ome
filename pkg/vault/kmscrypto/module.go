package kmscrypto

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/vault/kmsvault"
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

type kmsCryptoParamsWithListOfConfigs struct {
	fx.In

	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*Config `group:"kmsCryptoConfigs"`
}

func ProvideListOfKmsCryptoWithAppParams(params kmsCryptoParamsWithListOfConfigs) ([]*KmsCrypto, error) {
	kmsCryptoList := make([]*KmsCrypto, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		kmsCrypto, err := NewKmsCrypto(config)
		if err != nil {
			return kmsCryptoList, fmt.Errorf("error initializing KmsCrypto using config: %+v: %+v", config, err)
		}
		kmsCryptoList = append(kmsCryptoList, kmsCrypto)
	}
	return kmsCryptoList, nil
}
