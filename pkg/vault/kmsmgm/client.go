package kmsmgm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
)

type KmsMgm struct {
	logger logging.Interface
	config *Config
	Client *keymanagement.KmsManagementClient
}

type KeyMetadata struct {
	Name             string                                     `mapstructure:"key_name"`
	CompartmentId    string                                     `mapstructure:"compartment_id"`
	Algorithm        keymanagement.ListKeysAlgorithmEnum        `mapstructure:"algorithm"`
	Length           int                                        `mapstructure:"length"`
	LifecycleState   keymanagement.KeySummaryLifecycleStateEnum `mapstructure:"life_cycle_state"`
	ProtectionModel  keymanagement.ListKeysProtectionModeEnum   `mapstructure:"protection_model"`
	EnableDefinedTag bool                                       `mapstructure:"enable_defined_tag"`
	DefinedTags      DefinedTags                                `mapstructure:"defined_tags"`
}

type DefinedTags struct {
	Namespace string
	Key       string
	Value     string
}

// NewKmsMgm initializes a new KmsMgm instance with the given configuration and environment.
func NewKmsMgm(config *Config) (*KmsMgm, error) {
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %w", err)
	}

	client, err := newKmsManagementClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS management client: %w", err)
	}

	return &KmsMgm{
		logger: config.AnotherLogger,
		config: config,
		Client: client,
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

// newKmsManagementClient creates a new KMS Management client with the specified configuration provider.
func newKmsManagementClient(configProvider common.ConfigurationProvider, config *Config) (*keymanagement.KmsManagementClient, error) {
	client, err := keymanagement.NewKmsManagementClientWithConfigurationProvider(configProvider, config.KmsManagementEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS management client: %w", err)
	}
	return &client, nil
}

// GetKeys retrieves keys based on the provided metadata, applying filtering for name and tags.
func (km *KmsMgm) GetKeys(metadata KeyMetadata) ([]keymanagement.KeySummary, error) {
	km.logger.Infof("Retrieving keys for compartment: %s, name: %s", metadata.CompartmentId, metadata.Name)

	keys, err := km.listKeysByAttributes(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	filteredKeys := km.filterKeysByNameAndTag(keys, metadata)
	if len(filteredKeys) == 0 {
		return nil, fmt.Errorf("no keys found matching metadata: %+v", metadata)
	}

	km.logger.Infof("Found %d keys matching criteria", len(filteredKeys))
	return filteredKeys, nil
}

// listKeysByAttributes lists keys in the specified compartment based on algorithm, length, and protection model.
func (km *KmsMgm) listKeysByAttributes(metadata KeyMetadata) ([]keymanagement.KeySummary, error) {
	request := keymanagement.ListKeysRequest{
		CompartmentId:  &metadata.CompartmentId,
		Algorithm:      metadata.Algorithm,
		Length:         &metadata.Length,
		ProtectionMode: metadata.ProtectionModel,
	}

	km.logger.Debugf("Listing keys with attributes: Compartment=%s, Algorithm=%s, Length=%d, ProtectionMode=%s",
		metadata.CompartmentId, metadata.Algorithm, metadata.Length, metadata.ProtectionModel)

	response, err := km.Client.ListKeys(context.Background(), request)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return response.Items, nil
}

// filterKeysByNameAndTag filters the retrieved keys by matching name and defined tags, if enabled.
func (km *KmsMgm) filterKeysByNameAndTag(keys []keymanagement.KeySummary, metadata KeyMetadata) []keymanagement.KeySummary {
	var filteredKeys []keymanagement.KeySummary
	for _, key := range keys {
		if km.matchesNameAndState(key, metadata) && km.matchesDefinedTag(key, metadata) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys
}

// matchesNameAndState checks if a key's display name and lifecycle state match the metadata.
func (km *KmsMgm) matchesNameAndState(key keymanagement.KeySummary, metadata KeyMetadata) bool {
	match := *key.DisplayName == metadata.Name && key.LifecycleState == metadata.LifecycleState
	if !match {
		km.logger.Debugf("Key %s did not match name or lifecycle state", *key.DisplayName)
	}
	return match
}

// matchesDefinedTag checks if a key has a defined tag that matches the specified metadata.
func (km *KmsMgm) matchesDefinedTag(key keymanagement.KeySummary, metadata KeyMetadata) bool {
	if !metadata.EnableDefinedTag {
		return true
	}

	tagNamespace := metadata.DefinedTags.Namespace
	tagKey := metadata.DefinedTags.Key
	expectedValue := metadata.DefinedTags.Value

	if tag, exists := key.DefinedTags[tagNamespace]; exists {
		if value, ok := tag[tagKey]; ok && value == expectedValue {
			return true
		}
	}

	km.logger.Debugf("Key %s did not match defined tag %s:%s=%s", *key.DisplayName, tagNamespace, tagKey, expectedValue)
	return false
}
