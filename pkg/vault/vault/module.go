package oci_vault

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type vaultParams struct {
	fx.In

	AnotherLogger logging.Interface
}

var Module = fx.Provide(
	func(v *viper.Viper, params vaultParams) (*VaultClient, error) {
		config, err := NewSecretInVaultConfig(
			WithViper(v),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating secret in vault agent config: %+v", err)
		}
		return NewVaultClient(config)
	})
