package secret_in_vault

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/vault"
)

type SecretInVault struct {
	logger      logging.Interface
	VaultClient *vault.VaultsClient
}

func NewSecretInVault(config *SecretInVaultConfig, e *env.Environment) (*SecretInVault, error) {
	if config == nil {
		return nil, fmt.Errorf("SecretInVaultConfig is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("SecretInVaultConfig is invalid: %+v", err)
	}

	configProvider, err := getConfigProvider(config, e)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %+v", err)
	}

	client, err := NewVaultClient(configProvider)
	if err != nil {
		return nil, err
	}

	return &SecretInVault{
		logger:      config.AnotherLogger,
		VaultClient: client,
	}, nil
}

func (siv *SecretInVault) CreateSecretInVault(secretConfig secrets.SecretConfig, secretPlainText string) (*vault.CreateSecretResponse, error) {
	createSecretDetails := vault.CreateSecretDetails{
		CompartmentId: secretConfig.CompartmentId,
		SecretName:    secretConfig.SecretName,
		SecretContent: vault.Base64SecretContentDetails{
			Content: common.String(secrets.B64Encode(secretPlainText)),
		},
		VaultId:     secretConfig.VaultId,
		Description: common.String(fmt.Sprintf("DEK for the model %s", *secretConfig.SecretName)),
		KeyId:       secretConfig.KeyId,
	}
	createSecretRequest := vault.CreateSecretRequest{
		CreateSecretDetails: createSecretDetails,
	}
	createSecretResponse, err := siv.VaultClient.CreateSecret(context.Background(), createSecretRequest)
	if err != nil || createSecretResponse.RawResponse == nil || createSecretResponse.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create secret %s in vault %s: %v", *secretConfig.SecretName, *secretConfig.VaultId, err)
	}
	return &createSecretResponse, nil
}
