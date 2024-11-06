package key_management

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
)

type KmsKeyManager struct {
	logger              logging.Interface
	KmsManagementClient *keymanagement.KmsManagementClient
	Config              *KmsConfig
}

func NewKmsKeyManager(config *KmsConfig, e *env.Environment) (*KmsKeyManager, error) {
	if config == nil {
		return nil, fmt.Errorf("KmsConfig is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("KmsConfig is invalid: %+v", err)
	}

	configProvider, err := getConfigProvider(config, e)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %+v", err)
	}

	client, err := NewKmsManagementClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS client: %v", err)
	}

	return &KmsKeyManager{
		logger:              config.AnotherLogger,
		Config:              config,
		KmsManagementClient: client,
	}, nil
}

// GetKeys retrieves keys based on the given key configuration.
func (km *KmsKeyManager) GetKeys(keyConfig secrets.KeyConfig) ([]keymanagement.KeySummary, error) {
	keys, err := km.ListKeysByAttributes(keyConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %v", err)
	}

	filteredKeys := km.FilterKeysByNameAndTag(keys, keyConfig)
	if len(filteredKeys) == 0 {
		return nil, fmt.Errorf("no key found for the given key config: %+v", keyConfig)
	}
	return filteredKeys, nil
}

// ListKeysByAttributes lists keys based on key shape and protection mode attributes.
func (km *KmsKeyManager) ListKeysByAttributes(keyConfig secrets.KeyConfig) ([]keymanagement.KeySummary, error) {
	request := keymanagement.ListKeysRequest{
		CompartmentId:  &keyConfig.CompartmentId,
		Algorithm:      keyConfig.Algorithm,
		Length:         &keyConfig.Length,
		ProtectionMode: keyConfig.ProtectMode,
	}

	response, err := km.KmsManagementClient.ListKeys(context.Background(), request)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list keys with response %+v: %v", response, err)
	}
	return response.Items, nil
}

// FilterKeysByNameAndTag filters keys by display name, lifecycle state, and optional defined tags.
func (km *KmsKeyManager) FilterKeysByNameAndTag(keys []keymanagement.KeySummary, keyConfig secrets.KeyConfig) []keymanagement.KeySummary {
	var result []keymanagement.KeySummary
	for _, key := range keys {
		if km.matchesNameAndState(key, keyConfig) && km.matchesDefinedTag(key, keyConfig) {
			result = append(result, key)
		}
	}
	return result
}

// matchesNameAndState checks if a key matches the display name and lifecycle state in the configuration.
func (km *KmsKeyManager) matchesNameAndState(key keymanagement.KeySummary, keyConfig secrets.KeyConfig) bool {
	return *key.DisplayName == keyConfig.Name && key.LifecycleState == keyConfig.LifecycleStyle
}

// matchesDefinedTag checks if a key has a matching defined tag in the configuration if tagging is enabled.
func (km *KmsKeyManager) matchesDefinedTag(key keymanagement.KeySummary, keyConfig secrets.KeyConfig) bool {
	if !keyConfig.EnableDefinedTag {
		return true
	}

	tagNamespace := keyConfig.DefinedTag.Namespace
	tagKey := keyConfig.DefinedTag.Key
	expectedValue := keyConfig.DefinedTag.Value

	if tag, exists := key.DefinedTags[tagNamespace]; exists {
		if value, ok := tag[tagKey]; ok && value == expectedValue {
			return true
		}
	}
	return false
}
