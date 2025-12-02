package ociredis

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/redis"
)

type OciRedisClient struct {
	logger        logging.Interface
	config        *Config
	userClient    *redis.OciCacheUserClient
	clusterClient *redis.RedisClusterClient
}

func NewOciRedisClient(config *Config) (*OciRedisClient, error) {
	common.EnableInstanceMetadataServiceLookup()
	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get redis config provider: %w", err)
	}

	clusterClient, err := newRedisClusterClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create RedisClusterClient: %w", err)
	}

	userClient, err := newRedisUserClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create RedisUserClient: %w", err)
	}

	return &OciRedisClient{
		logger:        config.AnotherLogger,
		config:        config,
		clusterClient: clusterClient,
		userClient:    userClient,
	}, nil
}

func getConfigProvider(config *Config) (common.ConfigurationProvider, error) {
	principalOpts := principals.Opts{Log: config.AnotherLogger}
	principalConfig := principals.Config{AuthType: *config.AuthType}
	return principalConfig.Build(principalOpts)
}

func newRedisClusterClient(
	configProvider common.ConfigurationProvider,
	config *Config,
) (*redis.RedisClusterClient, error) {
	client, err := redis.NewRedisClusterClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create RedisClusterClient: %w", err)
	}

	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		client.SetRegion(config.Region)
	}

	return &client, nil
}

func newRedisUserClient(
	configProvider common.ConfigurationProvider,
	config *Config,
) (*redis.OciCacheUserClient, error) {
	userClient, err := redis.NewOciCacheUserClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create RedisUserClient: %w", err)
	}

	if !utils.IsStringEmptyOrWithWhitespaces(config.Region) {
		userClient.SetRegion(config.Region)
	}

	return &userClient, nil
}

// CreateRedisCluster wraps the SDK call with logging + error wrapping.
func (c *OciRedisClient) CreateRedisCluster(
	ctx context.Context,
	req redis.CreateRedisClusterRequest,
) (redis.CreateRedisClusterResponse, error) {
	displayName := ""
	if req.CreateRedisClusterDetails.DisplayName != nil {
		displayName = *req.CreateRedisClusterDetails.DisplayName
	}
	compartment := ""
	if req.CreateRedisClusterDetails.CompartmentId != nil {
		compartment = *req.CreateRedisClusterDetails.CompartmentId
	}

	c.logger.Infof("Creating Redis cluster: displayName=%s, compartment=%s",
		displayName, compartment)

	resp, err := c.clusterClient.CreateRedisCluster(ctx, req)
	if err != nil {
		c.logger.Errorf("Failed to create Redis cluster %s: %v", displayName, err)
		return resp, fmt.Errorf("create redis cluster: %w", err)
	}

	id := ""
	if resp.RedisCluster.Id != nil {
		id = *resp.RedisCluster.Id
	}
	c.logger.Infof("Successfully created Redis cluster: %s", id)
	return resp, nil
}

func (c *OciRedisClient) GetRedisCluster(
	ctx context.Context,
	req redis.GetRedisClusterRequest,
) (redis.GetRedisClusterResponse, error) {
	clusterID := ""
	if req.RedisClusterId != nil {
		clusterID = *req.RedisClusterId
	}
	c.logger.Infof("Getting Redis cluster: %s", clusterID)

	resp, err := c.clusterClient.GetRedisCluster(ctx, req)
	if err != nil {
		c.logger.Errorf("Failed to get Redis cluster %s: %v", clusterID, err)
		return resp, fmt.Errorf("get redis cluster %s: %w", clusterID, err)
	}

	id := ""
	if resp.RedisCluster.Id != nil {
		id = *resp.RedisCluster.Id
	}
	c.logger.Infof("Successfully retrieved Redis cluster: %s", id)
	return resp, nil
}

func (c *OciRedisClient) DeleteRedisCluster(
	ctx context.Context,
	req redis.DeleteRedisClusterRequest,
) (redis.DeleteRedisClusterResponse, error) {
	clusterID := ""
	if req.RedisClusterId != nil {
		clusterID = *req.RedisClusterId
	}
	c.logger.Infof("Deleting Redis cluster: %s", clusterID)

	resp, err := c.clusterClient.DeleteRedisCluster(ctx, req)
	if err != nil {
		c.logger.Errorf("Failed to delete Redis cluster %s: %v", clusterID, err)
		return resp, fmt.Errorf("delete redis cluster %s: %w", clusterID, err)
	}

	c.logger.Infof("Successfully deleted Redis cluster: %s", clusterID)
	return resp, nil
}

// Create cache user with ACL and password-auth mode
func (c *OciRedisClient) CreateUserWithPassword(
	ctx context.Context,
	compartmentOCID string, name string, description string, acl string, hashedPasswords []string,
) (redis.CreateOciCacheUserResponse, error) {
	authMode := redis.PasswordAuthenticationMode{
		HashedPasswords: hashedPasswords,
	}

	details := redis.CreateOciCacheUserDetails{
		CompartmentId:      &compartmentOCID,
		Name:               &name,
		Description:        &description,
		AclString:          &acl,
		AuthenticationMode: authMode,
		Status:             redis.OciCacheUserStatusOn,
	}

	req := redis.CreateOciCacheUserRequest{
		CreateOciCacheUserDetails: details,
		//OpcRetryToken:             &name,
	}

	return c.userClient.CreateOciCacheUser(ctx, req)
}

// Attach user to a Redis cluster
func (c *OciRedisClient) AttachUserToCluster(
	ctx context.Context,
	clusterOCID string,
	userOCIDs []string,
) (redis.AttachOciCacheUsersResponse, error) {
	details := redis.AttachOciCacheUsersDetails{
		OciCacheUsers: userOCIDs,
	}
	req := redis.AttachOciCacheUsersRequest{
		RedisClusterId:             &clusterOCID,
		AttachOciCacheUsersDetails: details,
	}
	return c.clusterClient.AttachOciCacheUsers(ctx, req)
}

func (c *OciRedisClient) GetOciCacheUser(
	ctx context.Context,
	userOCID string,
) (redis.GetOciCacheUserResponse, error) {
	req := redis.GetOciCacheUserRequest{
		OciCacheUserId: &userOCID,
	}
	return c.userClient.GetOciCacheUser(ctx, req)
}

func (c *OciRedisClient) DeleteOciCacheUser(
	ctx context.Context,
	userOCID string,
) (redis.DeleteOciCacheUserResponse, error) {
	req := redis.DeleteOciCacheUserRequest{
		OciCacheUserId: &userOCID,
	}
	return c.userClient.DeleteOciCacheUser(ctx, req)
}
