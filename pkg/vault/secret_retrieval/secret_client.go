package secret_retrieval

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/secrets"
)

func NewSecretClient(configProvider common.ConfigurationProvider, config *SecretRetrievalConfig) (*secrets.SecretsClient, error) {
	secretsClient, err := secrets.NewSecretsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SecretsClient: %s", err.Error())
	}

	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		secretsClient.SetRegion(config.Region)
	}
	return &secretsClient, nil
}

func getConfigProvider(config *SecretRetrievalConfig) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	return principalConfig.Build(principalOpts)
}
