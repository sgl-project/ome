package key_management

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
)

func NewKmsCryptoClient(configProvider common.ConfigurationProvider, config *KmsConfig) (*keymanagement.KmsCryptoClient, error) {
	client, err := keymanagement.NewKmsCryptoClientWithConfigurationProvider(configProvider, *config.KmsCryptoEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms crypto client: %s", err.Error())
	}

	return &client, nil
}

func NewKmsManagementClient(configProvider common.ConfigurationProvider, config *KmsConfig) (*keymanagement.KmsManagementClient, error) {
	client, err := keymanagement.NewKmsManagementClientWithConfigurationProvider(configProvider, *config.KmsManagementEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms crypto client: %s", err.Error())
	}

	return &client, nil
}

func getConfigProvider(config *KmsConfig, e *env.Environment) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Env: e,
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	return principalConfig.Build(principalOpts)
}
