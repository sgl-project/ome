package key_management

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"context"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/keymanagement"
	"net/http"
)

type KmsManagement struct {
	logger              logging.Interface
	KmsManagementClient *keymanagement.KmsManagementClient
	Config              *KmsConfig
}

func NewKmsManagement(config *KmsConfig, e *env.Environment) (*KmsManagement, error) {
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
		return nil, err
	}

	return &KmsManagement{
		logger:              config.AnotherLogger,
		Config:              config,
		KmsManagementClient: client,
	}, nil
}

func (km *KmsManagement) GetKeys(keyConfig secrets.KeyConfig) ([]keymanagement.KeySummary, error) {
	var res []keymanagement.KeySummary = nil
	if keys, err := km.ListKeysByKeyShapeAndProtectMode(keyConfig); err == nil {
		keys = km.SearchKeysByNameAndTag(keys, &keyConfig)
		res = keys
	} else {
		return nil, fmt.Errorf("failed to list keys: %v", err)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no key found given key config: %+v", keyConfig)
	}
	return res, nil
}

func (km *KmsManagement) ListKeysByKeyShapeAndProtectMode(keyConfig secrets.KeyConfig) ([]keymanagement.KeySummary, error) {
	listKeysRequest := keymanagement.ListKeysRequest{
		CompartmentId:  &keyConfig.CompartmentId,
		Algorithm:      keyConfig.Algorithm,
		Length:         &keyConfig.Length,
		ProtectionMode: keyConfig.ProtectMode,
	}
	response, err := km.KmsManagementClient.ListKeys(context.Background(), listKeysRequest)

	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to list keys with response %+v: %+v",
			response,
			err)
	}
	return response.Items, nil
}

func (km *KmsManagement) SearchKeysByNameAndTag(keys []keymanagement.KeySummary, keyConfig *secrets.KeyConfig) []keymanagement.KeySummary {
	var res []keymanagement.KeySummary
	for _, key := range keys {
		if *key.DisplayName == keyConfig.Name && key.LifecycleState == keyConfig.LifecycleStyle {
			if keyConfig.EnableDefinedTag {
				if tag, ok := key.DefinedTags[keyConfig.DefinedTag.Namespace]; ok {
					if value, ok := tag[keyConfig.DefinedTag.Key]; ok {
						if value == keyConfig.DefinedTag.Value {
							res = append(res, key)
						}
					}
				}
			} else {
				res = append(res, key)
			}
		}
	}
	return res
}
