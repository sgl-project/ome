package secret_retrieval

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	omesecrets "bitbucket.oci.oraclecorp.com/gen/ome/pkg/secrets"
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"net/http"
)

type SecretRetrieval struct {
	logger                logging.Interface
	SecretsClient         *secrets.SecretsClient
	SecretRetrievalConfig *SecretRetrievalConfig
}

func NewSecretRetrieval(config *SecretRetrievalConfig, e *env.Environment) (*SecretRetrieval, error) {
	if config == nil {
		return nil, fmt.Errorf("SecretRetrievalConfig is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("SecretRetrievalConfig is invalid: %+v", err)
	}

	configProvider, err := getConfigProvider(config, e)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %+v", err)
	}

	client, err := NewSecretClient(configProvider, config)
	if err != nil {
		return nil, err
	}

	return &SecretRetrieval{
		logger:                config.AnotherLogger,
		SecretRetrievalConfig: config,
		SecretsClient:         client,
	}, nil
}

func (sc *SecretRetrieval) GetSecretBundleContentByNameAndVaultId(secretConfig omesecrets.SecretConfig) (*string, error) {
	if err := secretConfig.ValidateNameAndVaultId(); err != nil {
		return nil, fmt.Errorf("invalid secret config: %w", err)
	}

	getSecretBundleByNameRequest := secrets.GetSecretBundleByNameRequest{
		SecretName: secretConfig.SecretName,
		VaultId:    secretConfig.VaultId,
	}

	if secretConfig.SecretVersionConfig != nil {
		versionConfig := secretConfig.SecretVersionConfig
		defaultVersionNum := int64(0)
		if versionConfig.SecretVersionNumber != &defaultVersionNum {
			getSecretBundleByNameRequest.VersionNumber = versionConfig.SecretVersionNumber
		}
		if versionConfig.SecretVersionName != nil {
			getSecretBundleByNameRequest.SecretVersionName = versionConfig.SecretVersionName
		}
		if versionConfig.Stage != nil {
			if secretVersion, ok := secrets.GetMappingGetSecretBundleByNameStageEnum(string(*versionConfig.Stage)); ok {
				getSecretBundleByNameRequest.Stage = secretVersion
			}
		}
	}

	response, err := sc.SecretsClient.GetSecretBundleByName(context.Background(), getSecretBundleByNameRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get secret %s in vault %s: %v", *secretConfig.SecretName, *secretConfig.VaultId, err)
	}
	return response.SecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails).Content, nil
}

func (sc *SecretRetrieval) GetSecretBundleContentBySecretId(secretConfig omesecrets.SecretConfig) (*string, error) {
	if err := secretConfig.ValidateSecretId(); err != nil {
		return nil, fmt.Errorf("invalid secret config: %w", err)
	}

	getSecretBundleByIdRequest := secrets.GetSecretBundleRequest{
		SecretId: secretConfig.SecretId,
	}

	if secretConfig.SecretVersionConfig != nil {
		versionConfig := secretConfig.SecretVersionConfig
		defaultVersionNum := int64(0)
		if versionConfig.SecretVersionNumber != &defaultVersionNum {
			getSecretBundleByIdRequest.VersionNumber = versionConfig.SecretVersionNumber
		}
		if versionConfig.SecretVersionName != nil {
			getSecretBundleByIdRequest.SecretVersionName = versionConfig.SecretVersionName
		}
		if versionConfig.Stage != nil {
			if secretVersion, ok := secrets.GetMappingGetSecretBundleStageEnum(string(*versionConfig.Stage)); ok {
				getSecretBundleByIdRequest.Stage = secretVersion
			}
		}
	}

	response, err := sc.SecretsClient.GetSecretBundle(context.Background(), getSecretBundleByIdRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get secret %s: %v", *secretConfig.SecretId, err)
	}

	return response.SecretBundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails).Content, nil
}
