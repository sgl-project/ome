package secret_in_vault

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func ProvideSecretInVaultConfig(v *viper.Viper, logger logging.Interface) (*SecretInVaultConfig, error) {
	secretInVaultConfig, err := NewSecretInVaultConfig(
		WithViper(v),
		WithAnotherLog(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVaultConfig: %+v", err)
	}
	return secretInVaultConfig, nil
}

func ProvideSecretInVault(v *viper.Viper, logger logging.Interface) (*SecretInVault, error) {
	secretInVaultConfig, err := ProvideSecretInVaultConfig(v, logger)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVaultConfig: %+v", err)
	}

	secretInVault, err := NewSecretInVault(secretInVaultConfig)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVault: %+v", err)
	}
	return secretInVault, nil
}

var SecretInVaultModule = fx.Provide(
	ProvideSecretInVault,
)

/*
 * Below is a way to inject a list of SecretInVault using a list of Configs leveraging fx Value Groups feature
 */
type appParamsWithConfigs struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	/*
	 * Use Value Groups feature from fx to inject a list of Configs
	 * https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
	 */
	Configs []*SecretInVaultConfig `group:"secretInVaultConfigs"`
}

func ProvideListOfSecretInVaultWithAppParams(params appParamsWithConfigs) ([]*SecretInVault, error) {
	secretInVaultList := make([]*SecretInVault, 0)
	for _, config := range params.Configs {
		if config == nil {
			continue
		}
		secretInVault, err := NewSecretInVault(config)
		if err != nil {
			return secretInVaultList, fmt.Errorf("error initializing a list of SecretInVault using Config: %+v: %+v", config, err)
		}
		secretInVaultList = append(secretInVaultList, secretInVault)
	}
	return secretInVaultList, nil
}
