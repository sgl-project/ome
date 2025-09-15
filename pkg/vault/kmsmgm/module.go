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

type kmsMgmParamsWithListOfConfigs struct {
	fx.In

	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*Config `group:"kmsMgmConfigs"`
}

func ProvideListOfKmsMgmWithAppParams(params kmsMgmParamsWithListOfConfigs) ([]*KmsMgm, error) {
	kmsManagementList := make([]*KmsMgm, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		kmsManagement, err := NewKmsMgm(config)
		if err != nil {
			return kmsManagementList, fmt.Errorf("error initializing KmsManagement using config: %+v: %+v", config, err)
		}
		kmsManagementList = append(kmsManagementList, kmsManagement)
	}
	return kmsManagementList, nil
}
