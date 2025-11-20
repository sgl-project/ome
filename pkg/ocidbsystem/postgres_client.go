package ocidbsystem

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"
)

type OciPostgresClient struct {
	logger logging.Interface
	config *Config
	client *psql.PostgresqlClient
}

func NewOciPostgreSQLClient(config *Config) (*OciPostgresClient, error) {
	common.EnableInstanceMetadataServiceLookup()
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %w", err)
	}
	client, err := newPostgresqlClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Postgresql client: %w", err)
	}
	return &OciPostgresClient{
		logger: config.AnotherLogger,
		config: config,
		client: client,
	}, nil
}

func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{Log: config.AnotherLogger}
	principalConfig := principals.Config{AuthType: *config.AuthType}
	return principalConfig.Build(principalOpts)
}

// newPostgresqlClient creates a new Postgresql client based on the configuration.
func newPostgresqlClient(configProvider common.ConfigurationProvider, config *Config) (*psql.PostgresqlClient, error) {
	client, err := psql.NewPostgresqlClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Postgresql client: %w", err)
	}

	// Set the region if specified in the configuration
	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		client.SetRegion(config.Region)
	}

	return &client, nil
}

// CreateDbSystem calls the underlying OCI Postgresql client to create a DB System.
func (p *OciPostgresClient) CreateDbSystem(
	ctx context.Context,
	request psql.CreateDbSystemRequest,
) (psql.CreateDbSystemResponse, error) {
	p.logger.Infof("Creating Postgresql DB System: displayName=%v, compartment=%v",
		request.CreateDbSystemDetails.DisplayName,
		request.CreateDbSystemDetails.CompartmentId,
	)

	resp, err := p.client.CreateDbSystem(ctx, request)
	if err != nil {
		p.logger.Errorf("Failed to create Postgresql DB System: %v", err)
		return resp, fmt.Errorf("create db system: %w", err)
	}

	id := ""
	if resp.DbSystem.Id != nil {
		id = *resp.DbSystem.Id
	}
	p.logger.Infof("Successfully created Postgresql DB System: %s", id)
	return resp, nil
}

// GetDbSystem retrieves details for a given DB System.
func (p *OciPostgresClient) GetDbSystem(
	ctx context.Context,
	request psql.GetDbSystemRequest,
) (psql.GetDbSystemResponse, error) {
	reqID := ""
	if request.DbSystemId != nil {
		reqID = *request.DbSystemId
	}
	p.logger.Infof("Getting Postgresql DB System: %s", reqID)

	resp, err := p.client.GetDbSystem(ctx, request)
	if err != nil {
		p.logger.Errorf("Failed to get Postgresql DB System %s: %v", reqID, err)
		return resp, fmt.Errorf("get db system %s: %w", reqID, err)
	}

	id := ""
	if resp.DbSystem.Id != nil {
		id = *resp.DbSystem.Id
	}
	p.logger.Infof("Successfully retrieved Postgresql DB System: %s", id)
	return resp, nil
}

// GetConnectionDetails gets the connection information for a DB System.
func (p *OciPostgresClient) GetConnectionDetails(
	ctx context.Context,
	request psql.GetConnectionDetailsRequest,
) (psql.GetConnectionDetailsResponse, error) {
	reqID := ""
	if request.DbSystemId != nil {
		reqID = *request.DbSystemId
	}
	p.logger.Infof("Getting connection details for Postgresql DB System: %s", reqID)

	resp, err := p.client.GetConnectionDetails(ctx, request)
	if err != nil {
		p.logger.Errorf("Failed to get connection details for DB System %s: %v", reqID, err)
		return resp, fmt.Errorf("get connection details %s: %w", reqID, err)
	}

	p.logger.Infof("Successfully retrieved connection details for DB System: %s", reqID)
	return resp, nil
}

// DeleteDbSystem deletes the specified DB System.
func (p *OciPostgresClient) DeleteDbSystem(
	ctx context.Context,
	request psql.DeleteDbSystemRequest,
) (psql.DeleteDbSystemResponse, error) {
	reqID := ""
	if request.DbSystemId != nil {
		reqID = *request.DbSystemId
	}
	p.logger.Infof("Deleting Postgresql DB System: %s", reqID)

	resp, err := p.client.DeleteDbSystem(ctx, request)
	if err != nil {
		p.logger.Errorf("Failed to delete Postgresql DB System %s: %v", reqID, err)
		return resp, fmt.Errorf("delete db system %s: %w", reqID, err)
	}

	p.logger.Infof("Successfully deleted Postgresql DB System: %s", reqID)
	return resp, nil
}
