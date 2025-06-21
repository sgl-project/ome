package kmsvault

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/utils"
)

type KMSVault struct {
	logger logging.Interface
	config *Config
	client *keymanagement.KmsVaultClient
}

// NewKMSVault initializes a new KMSVault instance with the provided configuration and environment.
func NewKMSVault(config *Config) (*KMSVault, error) {
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %w", err)
	}

	client, err := newKmsVaultClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize KMS Vault client: %w", err)
	}

	return &KMSVault{
		logger: config.AnotherLogger,
		config: config,
		client: client,
	}, nil
}

// getConfigProvider builds the configuration provider for OCI authentication.
func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	provider, err := principalConfig.Build(principalOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration provider: %w", err)
	}
	return provider, nil
}

// newKmsVaultClient creates a new KMS Vault client based on the configuration.
func newKmsVaultClient(configProvider common.ConfigurationProvider, config *Config) (*keymanagement.KmsVaultClient, error) {
	var client keymanagement.KmsVaultClient
	var err error

	if config.EnableOboToken {
		if config.OboToken == "" {
			return nil, fmt.Errorf("failed to create KMS Vault client: OBO token is empty")
		}
		client, err = keymanagement.NewKmsVaultClientWithOboToken(configProvider, config.OboToken)
	} else {
		client, err = keymanagement.NewKmsVaultClientWithConfigurationProvider(configProvider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create KMS Vault client: %w", err)
	}

	// Set the region if specified in the configuration
	if !utils.IsStringEmptyOrWithWhitespaces(config.region) {
		client.SetRegion(config.region)
	}

	return &client, nil
}

// GetVault retrieves the vault details for a specified vault ID.
func (k *KMSVault) GetVault(vaultID string) (*keymanagement.GetVaultResponse, error) {
	k.logger.Infof("Retrieving vault with ID: %s", vaultID)

	request := keymanagement.GetVaultRequest{
		VaultId: common.String(vaultID),
	}

	response, err := k.client.GetVault(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve vault with ID %s: %w", vaultID, err)
	}

	k.logger.Infof("Successfully retrieved vault with ID: %s", vaultID)
	return &response, nil
}

// GetCryptoEndpoint returns the crypto endpoint of a specified vault.
func (k *KMSVault) GetCryptoEndpoint(vaultID string) (string, error) {
	k.logger.Infof("Getting crypto endpoint for vault ID: %s", vaultID)

	vault, err := k.GetVault(vaultID)
	if err != nil {
		k.logger.Errorf("Failed to get crypto endpoint for vault ID %s: %v", vaultID, err)
		return "", fmt.Errorf("failed to get crypto endpoint for vault ID %s: %w", vaultID, err)
	}

	k.logger.Infof("Crypto endpoint for vault ID %s retrieved successfully", vaultID)
	return *vault.Vault.CryptoEndpoint, nil
}

// GetManagementEndpoint returns the management endpoint of a specified vault.
func (k *KMSVault) GetManagementEndpoint(vaultID string) (string, error) {
	k.logger.Infof("Getting management endpoint for vault ID: %s", vaultID)

	vault, err := k.GetVault(vaultID)
	if err != nil {
		k.logger.Errorf("Failed to get management endpoint for vault ID %s: %v", vaultID, err)
		return "", fmt.Errorf("failed to get management endpoint for vault ID %s: %w", vaultID, err)
	}

	k.logger.Infof("Management endpoint for vault ID %s retrieved successfully", vaultID)
	return *vault.Vault.ManagementEndpoint, nil
}
