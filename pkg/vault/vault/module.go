package oci_vault

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type vaultParams struct {
	fx.In

	AnotherLogger logging.Interface
}

var Module = fx.Provide(
	func(v *viper.Viper, e *env.Environment, params vaultParams) (*VaultClient, error) {
		config, err := NewSecretInVaultConfig(
			WithViper(v),
			WithEnv(e),
			WithAnotherLog(params.AnotherLogger),
			WithAppParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating secret in vault agent config: %+v", err)
		}
		return NewVaultClient(config, e)
	})
