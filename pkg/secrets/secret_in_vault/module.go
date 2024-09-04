package secret_in_vault

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type appParams struct {
	fx.In

	// this is an example on how to inject logger
	// with a specified name (in case you have many).
	// See https://uber-go.github.io/fx/get-started/another-handler.html
	AnotherLogger logging.Interface `name:"another_log"`

	ViperKeyNames []string `optional:"true"`
}

func ProvideSecretInVaultConfig(v *viper.Viper, e *env.Environment, params appParams) (*SecretInVaultConfig, error) {
	secretInVaultConfig, err := NewSecretInVaultConfig(
		WithViper(v),
		WithEnv(e),
		WithAnotherLog(params.AnotherLogger),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVaultConfig: %+v", err)
	}
	return secretInVaultConfig, nil
}

func ProvideSecretInVault(v *viper.Viper, e *env.Environment, params appParams) (*SecretInVault, error) {
	secretInVaultConfig, err := ProvideSecretInVaultConfig(v, e, params)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVaultConfig: %+v", err)
	}

	secretInVault, err := NewSecretInVault(secretInVaultConfig, e)
	if err != nil {
		return nil, fmt.Errorf("error initializing SecretInVault: %+v", err)
	}
	return secretInVault, nil
}

var SecretInVaultModule = fx.Provide(
	ProvideSecretInVault,
)
