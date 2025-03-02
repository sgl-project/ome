package casper

import (
	"fmt"
	"regexp"

	"github.com/oracle/oci-go-sdk/v65/common"
	cas "github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// GetObjectStorageClient returns an initialized ObjectStorageClient with the given configuration provider
func GetObjectStorageClient(configurationProvider common.ConfigurationProvider, regionOverride string) (*cas.ObjectStorageClient, error) {
	client, err := cas.NewObjectStorageClientWithConfigurationProvider(configurationProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create ObjectStorageClient: %s", err.Error())
	}

	if _, err = canStringBeRegion(regionOverride); err == nil {
		client.SetRegion(regionOverride)
	}

	return &client, nil
}

// GetObjectStorageClientWithOboToken returns an initialized ObjectStorageClient with the given configuration provider and obo token
func GetObjectStorageClientWithOboToken(configurationProvider common.ConfigurationProvider, oboToken string, regionOverride string) (*cas.ObjectStorageClient, error) {
	if oboToken == "" {
		return nil, fmt.Errorf("failed to get object storage client: oboToken is empty")
	}
	client, err := cas.NewObjectStorageClientWithOboToken(configurationProvider, oboToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create ObjectStorageClient: %s", err.Error())
	}

	if _, err = canStringBeRegion(regionOverride); err == nil {
		client.SetRegion(regionOverride)
	}

	return &client, nil
}

// canStringBeRegion test if the string can be a region, if it can, returns the string as is, otherwise it
// returns an error
var blankRegex = regexp.MustCompile(`\s`)

func canStringBeRegion(stringRegion string) (region string, err error) {
	if blankRegex.MatchString(stringRegion) || stringRegion == "" {
		return "", fmt.Errorf("region can not be empty or have spaces")
	}
	return stringRegion, nil
}
