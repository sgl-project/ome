package ociobjectstore

import (
	"fmt"
	"net/http"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	"github.com/sgl-project/ome/pkg/principals"
	"github.com/sgl-project/ome/pkg/utils"
)

func NewObjectStorageClient(configurationProvider common.ConfigurationProvider, config *Config) (*objectstorage.ObjectStorageClient, error) {
	common.EnableInstanceMetadataServiceLookup()
	var client objectstorage.ObjectStorageClient
	var err error
	if config.EnableOboToken {
		if config.OboToken == "" {
			return nil, fmt.Errorf("failed to get object storage client: oboToken is empty")
		}
		client, err = objectstorage.NewObjectStorageClientWithOboToken(configurationProvider, config.OboToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create ObjectStorageClient: %s", err.Error())
		}
	} else {
		client, err = objectstorage.NewObjectStorageClientWithConfigurationProvider(configurationProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to create objectStorageClient: %s", err.Error())
		}
	}
	client.BaseClient.HTTPClient = &http.Client{
		Timeout: 20 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
		},
	}

	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		client.SetRegion(config.Region)
	}

	return &client, nil
}

func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	configProvider, err := principalConfig.Build(principalOpts)
	return configProvider, err
}
