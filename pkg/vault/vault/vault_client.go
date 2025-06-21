package oci_vault

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/vault"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	vaultUtils "github.com/sgl-project/ome/pkg/vault"
)

type VaultClient struct {
	logger      logging.Interface
	vaultClient *vault.VaultsClient
}

// NewVaultClient initializes a new VaultClient with the provided configuration and environment.
func NewVaultClient(config *Config) (*VaultClient, error) {
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create config provider: %w", err)
	}

	client, err := newOCIVaultClient(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OCI Vault client: %w", err)
	}

	return &VaultClient{
		logger:      config.AnotherLogger,
		vaultClient: client,
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

// newOCIVaultClient creates a new VaultsClient using the provided configuration provider.
func newOCIVaultClient(configProvider common.ConfigurationProvider) (*vault.VaultsClient, error) {
	client, err := vault.NewVaultsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VaultsClient: %w", err)
	}
	return &client, nil
}

// CreateSecretInVault creates a new secret in the specified vault using the provided secret configuration and plaintext.
func (v *VaultClient) CreateSecretInVault(secretConfig vaultUtils.SecretConfig, secretPlainText string) (*vault.CreateSecretResponse, error) {
	v.logger.Infof("Creating secret %s in vault %s", *secretConfig.SecretName, *secretConfig.VaultId)

	// Encode the plaintext secret content to Base64
	base64Content := vaultUtils.B64Encode(secretPlainText)

	createSecretDetails := vault.CreateSecretDetails{
		CompartmentId: secretConfig.CompartmentId,
		SecretName:    secretConfig.SecretName,
		SecretContent: vault.Base64SecretContentDetails{
			Content: common.String(base64Content),
		},
		VaultId:     secretConfig.VaultId,
		Description: common.String(fmt.Sprintf("DEK for the model %s", *secretConfig.SecretName)),
		KeyId:       secretConfig.KeyId,
	}
	createSecretRequest := vault.CreateSecretRequest{
		CreateSecretDetails: createSecretDetails,
	}

	response, err := v.vaultClient.CreateSecret(context.Background(), createSecretRequest)
	if err != nil {
		v.logger.Errorf("Failed to create secret %s in vault %s: %v", *secretConfig.SecretName, *secretConfig.VaultId, err)
		return nil, fmt.Errorf("failed to create secret %s in vault %s: %w", *secretConfig.SecretName, *secretConfig.VaultId, err)
	}

	if response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response for creating secret %s in vault %s", *secretConfig.SecretName, *secretConfig.VaultId)
	}

	v.logger.Infof("Secret %s successfully created in vault %s", *secretConfig.SecretName, *secretConfig.VaultId)
	return &response, nil
}
