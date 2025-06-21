package ocisecret

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/secrets"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/utils"
)

type Secret struct {
	logger logging.Interface
	client *secrets.SecretsClient
	config *Config
}

// NewSecret initializes a new Secret instance with the provided configuration and environment.
func NewSecret(config *Config) (*Secret, error) {
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create config provider: %w", err)
	}

	client, err := newSecretClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Secrets client: %w", err)
	}

	return &Secret{
		logger: config.AnotherLogger,
		config: config,
		client: client,
	}, nil
}

// getConfigProvider sets up the configuration provider for OCI authentication.
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

// newSecretClient creates a new SecretsClient with the provided configuration and sets the region if specified.
func newSecretClient(configProvider common.ConfigurationProvider, config *Config) (*secrets.SecretsClient, error) {
	client, err := secrets.NewSecretsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create SecretsClient: %w", err)
	}

	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		client.SetRegion(config.Region)
	}

	return &client, nil
}

// GetSecretBundleContentByNameAndVaultId retrieves the content of a secret by its name and vault ID.
func (s *Secret) GetSecretBundleContentByNameAndVaultId(name string, vaultId string) (*string, error) {
	s.logger.Infof("Fetching secret content for name: %s in vault: %s", name, vaultId)

	request := secrets.GetSecretBundleByNameRequest{
		SecretName: &name,
		VaultId:    &vaultId,
	}

	response, err := s.client.GetSecretBundleByName(context.Background(), request)
	if err != nil {
		s.logger.Errorf("Failed to fetch secret content for name: %s in vault: %s, error: %v", name, vaultId, err)
		return nil, fmt.Errorf("failed to fetch secret %s in vault %s: %w", name, vaultId, err)
	}

	if !isResponseStatusOK(response.RawResponse) {
		return nil, fmt.Errorf("received non-OK response for secret %s in vault %s", name, vaultId)
	}

	return extractSecretContent(response.SecretBundle)
}

// GetSecretBundleContentBySecretId retrieves the content of a secret by its secret ID.
func (s *Secret) GetSecretBundleContentBySecretId(secretId string) (*string, error) {
	s.logger.Infof("Fetching secret content for secret ID: %s", secretId)

	request := secrets.GetSecretBundleRequest{
		SecretId: &secretId,
	}

	response, err := s.client.GetSecretBundle(context.Background(), request)
	if err != nil {
		s.logger.Errorf("Failed to fetch secret content for secret ID: %s, error: %v", secretId, err)
		return nil, fmt.Errorf("failed to fetch secret %s: %w", secretId, err)
	}

	if !isResponseStatusOK(response.RawResponse) {
		return nil, fmt.Errorf("received non-OK response for secret ID %s", secretId)
	}

	return extractSecretContent(response.SecretBundle)
}

// isResponseStatusOK checks if the HTTP response status is OK (200).
func isResponseStatusOK(response *http.Response) bool {
	return response != nil && response.StatusCode == http.StatusOK
}

// extractSecretContent extracts the content from a SecretBundle, returning an error if the format is unexpected.
func extractSecretContent(bundle secrets.SecretBundle) (*string, error) {
	content, ok := bundle.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		return nil, fmt.Errorf("unexpected secret bundle content format: expected Base64SecretBundleContentDetails")
	}
	return content.Content, nil
}
