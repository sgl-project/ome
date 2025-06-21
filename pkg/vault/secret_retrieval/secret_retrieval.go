package secret_retrieval

import (
	"context"
	"fmt"
	"net/http"

	omesecrets "github.com/sgl-project/ome/pkg/vault"

	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/sgl-project/ome/pkg/logging"
)

type SecretRetriever struct {
	logger        logging.Interface
	SecretsClient *secrets.SecretsClient
	Config        *SecretRetrievalConfig
}

func NewSecretRetriever(config *SecretRetrievalConfig) (*SecretRetriever, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %v", err)
	}

	client, err := NewSecretClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret client: %v", err)
	}

	return &SecretRetriever{
		logger:        config.AnotherLogger,
		Config:        config,
		SecretsClient: client,
	}, nil
}

func (sr *SecretRetriever) GetSecretBundleContentByNameAndVaultId(secretConfig omesecrets.SecretConfig) (*string, error) {
	if err := secretConfig.ValidateNameAndVaultId(); err != nil {
		return nil, fmt.Errorf("invalid secret config: %w", err)
	}

	request := secrets.GetSecretBundleByNameRequest{
		SecretName: secretConfig.SecretName,
		VaultId:    secretConfig.VaultId,
	}
	setSecretVersionConfig(secretConfig.SecretVersionConfig, &request)

	response, err := sr.SecretsClient.GetSecretBundleByName(context.Background(), request)
	if err != nil || !isResponseStatusOK(response.RawResponse) {
		return nil, fmt.Errorf("failed to get secret %s in vault %s: %v", *secretConfig.SecretName, *secretConfig.VaultId, err)
	}

	return extractSecretContent(response.SecretBundle)
}

func (sr *SecretRetriever) GetSecretBundleContentBySecretId(secretConfig omesecrets.SecretConfig) (*string, error) {
	if err := secretConfig.ValidateSecretId(); err != nil {
		return nil, fmt.Errorf("invalid secret config: %w", err)
	}

	request := secrets.GetSecretBundleRequest{
		SecretId: secretConfig.SecretId,
	}
	setSecretVersionConfig(secretConfig.SecretVersionConfig, &request)

	response, err := sr.SecretsClient.GetSecretBundle(context.Background(), request)
	if err != nil || !isResponseStatusOK(response.RawResponse) {
		return nil, fmt.Errorf("failed to get secret %s: %v", *secretConfig.SecretId, err)
	}

	return extractSecretContent(response.SecretBundle)
}

// setSecretVersionConfig applies the SecretVersionConfig settings to a request.
func setSecretVersionConfig(versionConfig *omesecrets.SecretVersionConfig, request interface{}) {
	if versionConfig == nil {
		return
	}

	defaultVersionNum := int64(0)
	if versionConfig.SecretVersionNumber != &defaultVersionNum {
		switch req := request.(type) {
		case *secrets.GetSecretBundleByNameRequest:
			req.VersionNumber = versionConfig.SecretVersionNumber
		case *secrets.GetSecretBundleRequest:
			req.VersionNumber = versionConfig.SecretVersionNumber
		}
	}

	if versionConfig.SecretVersionName != nil {
		switch req := request.(type) {
		case *secrets.GetSecretBundleByNameRequest:
			req.SecretVersionName = versionConfig.SecretVersionName
		case *secrets.GetSecretBundleRequest:
			req.SecretVersionName = versionConfig.SecretVersionName
		}
	}

	if versionConfig.Stage != nil {
		if secretVersion, ok := secrets.GetMappingGetSecretBundleStageEnum(string(*versionConfig.Stage)); ok {
			switch req := request.(type) {
			case *secrets.GetSecretBundleByNameRequest:
				req.Stage = secrets.GetSecretBundleByNameStageEnum(secretVersion)
			case *secrets.GetSecretBundleRequest:
				req.Stage = secretVersion
			}
		}
	}
}

// isResponseStatusOK checks if the response status is HTTP 200 OK.
func isResponseStatusOK(response *http.Response) bool {
	return response != nil && response.StatusCode == http.StatusOK
}

// extractSecretContent extracts the content from a secret bundle.
func extractSecretContent(bundle secrets.SecretBundle) (*string, error) {
	content, ok := bundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		return nil, fmt.Errorf("unexpected secret bundle content format")
	}
	return content.Content, nil
}
