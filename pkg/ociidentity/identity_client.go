package ociidentity

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type OciIdentityClient struct {
	logger logging.Interface
	config *Config
	client *identity.IdentityClient
}

func NewIdentityClient(config *Config) (*OciIdentityClient, error) {
	common.EnableInstanceMetadataServiceLookup()

	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %w", err)
	}

	client, err := newIdentityClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Identity client: %w", err)
	}

	return &OciIdentityClient{
		logger: config.AnotherLogger,
		config: config,
		client: client,
	}, nil
}

func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{
		Log: config.AnotherLogger,
	}
	principalConfig := principals.Config{
		AuthType: *config.AuthType,
	}
	return principalConfig.Build(principalOpts)
}

// newIdentityClient creates a new Identity client based on the configuration.
func newIdentityClient(configProvider common.ConfigurationProvider, config *Config) (*identity.IdentityClient, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Identity client: %w", err)
	}

	// Set the region if specified in the configuration.
	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		client.SetRegion(config.Region)
	}

	return &client, nil
}

// ListAvailabilityDomains proxies the call to the underlying Identity client with logging.
func (c *OciIdentityClient) ListAvailabilityDomains(
	ctx context.Context,
	request identity.ListAvailabilityDomainsRequest,
) (identity.ListAvailabilityDomainsResponse, error) {
	compartmentID := ""
	if request.CompartmentId != nil {
		compartmentID = *request.CompartmentId
	}

	c.logger.Infof("Listing availability domains for compartment: %s", compartmentID)

	resp, err := c.client.ListAvailabilityDomains(ctx, request)
	if err != nil {
		c.logger.Errorf("Failed to list availability domains for compartment %s: %v",
			compartmentID, err)
		return resp, fmt.Errorf("list availability domains for compartment %s: %w",
			compartmentID, err)
	}

	c.logger.Infof("Successfully listed %d availability domains for compartment %s",
		len(resp.Items), compartmentID)
	return resp, nil
}
