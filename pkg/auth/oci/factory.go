package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	authlib "github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Factory creates OCI credentials
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new OCI auth factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates OCI credentials based on config
func (f *Factory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
	if config.Provider != auth.ProviderOCI {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", auth.ProviderOCI, config.Provider)
	}

	var configProvider common.ConfigurationProvider
	var err error

	switch config.AuthType {
	case auth.OCIUserPrincipal:
		configProvider, err = f.createUserPrincipal(config)
	case auth.OCIInstancePrincipal:
		configProvider, err = f.createInstancePrincipal(config)
	case auth.OCIResourcePrincipal:
		configProvider, err = f.createResourcePrincipal(config)
	case auth.OCIOkeWorkloadIdentity:
		configProvider, err = f.createOkeWorkloadIdentity(config)
	default:
		return nil, fmt.Errorf("unsupported OCI auth type: %s", config.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OCI config provider: %w", err)
	}

	return &OCICredentials{
		configProvider: configProvider,
		authType:       config.AuthType,
		region:         config.Region,
		logger:         f.logger,
	}, nil
}

// SupportedAuthTypes returns supported OCI auth types
func (f *Factory) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.OCIUserPrincipal,
		auth.OCIInstancePrincipal,
		auth.OCIResourcePrincipal,
		auth.OCIOkeWorkloadIdentity,
	}
}

// createUserPrincipal creates a user principal configuration provider
func (f *Factory) createUserPrincipal(config auth.Config) (common.ConfigurationProvider, error) {
	// Extract user principal config
	upConfig := UserPrincipalConfig{}

	if config.Extra != nil {
		if up, ok := config.Extra["user_principal"].(map[string]interface{}); ok {
			if configPath, ok := up["config_path"].(string); ok {
				upConfig.ConfigPath = configPath
			}
			if profile, ok := up["profile"].(string); ok {
				upConfig.Profile = profile
			}
			if useSessionToken, ok := up["use_session_token"].(bool); ok {
				upConfig.UseSessionToken = useSessionToken
			}
		}
	}

	// Apply environment variables
	upConfig.ApplyEnvironment()

	// Validate
	if err := upConfig.Validate(); err != nil {
		return nil, err
	}

	// Create provider
	if upConfig.UseSessionToken {
		return common.CustomProfileSessionTokenConfigProvider(upConfig.ConfigPath, upConfig.Profile), nil
	}
	return common.CustomProfileConfigProvider(upConfig.ConfigPath, upConfig.Profile), nil
}

// createInstancePrincipal creates an instance principal configuration provider
func (f *Factory) createInstancePrincipal(config auth.Config) (common.ConfigurationProvider, error) {
	// Enable instance metadata service lookup
	common.EnableInstanceMetadataServiceLookup()

	return authlib.InstancePrincipalConfigurationProvider()
}

// createResourcePrincipal creates a resource principal configuration provider
func (f *Factory) createResourcePrincipal(config auth.Config) (common.ConfigurationProvider, error) {
	return authlib.ResourcePrincipalConfigurationProvider()
}

// createOkeWorkloadIdentity creates an OKE workload identity configuration provider
func (f *Factory) createOkeWorkloadIdentity(config auth.Config) (common.ConfigurationProvider, error) {
	return authlib.OkeWorkloadIdentityConfigurationProvider()
}
