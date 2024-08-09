package secret_in_vault

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/principals"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/vault"
)

func NewVaultClient(configProvider common.ConfigurationProvider) (*vault.VaultsClient, error) {
	vaultClient, err := vault.NewVaultsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VaultsClient: %v", err)
	}
	return &vaultClient, nil
}

func getConfigProvider(config *SecretInVaultConfig, e *env.Environment) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Env: e,
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	return principalConfig.Build(principalOpts)
}
