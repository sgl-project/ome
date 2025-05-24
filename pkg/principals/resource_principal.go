package principals

import (
	"fmt"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// ResourcePrincipalConfig encapsulates configuration for constructing
// resource principal authentication provider.
//
// Zero value is ready to use - SDK expects container runtime to set required env vars.
type ResourcePrincipalConfig struct {
	// Optional region override
	Region string `mapstructure:"region"`

	// Optional version override (defaults to "1.1" if not set by container runtime)
	Version string `mapstructure:"version"`

	// Optional endpoint overrides - usually not needed
	RPTEndpoint          string `mapstructure:"rpt_endpoint"`
	RPTPath              string `mapstructure:"rpt_path"`
	RPSTEndpoint         string `mapstructure:"rpst_endpoint"`
	AuthEndpointOverride string `mapstructure:"auth_endpoint_override"`
}

// Validate validates r.
func (r ResourcePrincipalConfig) Validate() error {
	// No validation needed - all fields are optional
	return nil
}

// Build builds a configuration provider from r.
func (r ResourcePrincipalConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
	// Set optional overrides if provided
	if r.Region != "" {
		if err := os.Setenv("OCI_RESOURCE_PRINCIPAL_REGION", r.Region); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_RESOURCE_PRINCIPAL_REGION")
		}
	}

	if r.Version != "" {
		if err := os.Setenv("OCI_RESOURCE_PRINCIPAL_VERSION", r.Version); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_RESOURCE_PRINCIPAL_VERSION")
		}
	}

	if r.RPTEndpoint != "" {
		if err := os.Setenv("OCI_RESOURCE_PRINCIPAL_RPT_ENDPOINT", r.RPTEndpoint); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_RESOURCE_PRINCIPAL_RPT_ENDPOINT")
		}
	}

	if r.RPTPath != "" {
		if err := os.Setenv("OCI_RESOURCE_PRINCIPAL_RPT_PATH", r.RPTPath); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_RESOURCE_PRINCIPAL_RPT_PATH")
		}
	}

	if r.RPSTEndpoint != "" {
		if err := os.Setenv("OCI_RESOURCE_PRINCIPAL_RPST_ENDPOINT", r.RPSTEndpoint); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_RESOURCE_PRINCIPAL_RPST_ENDPOINT")
		}
	}

	if r.AuthEndpointOverride != "" {
		if err := os.Setenv("OCI_SDK_AUTH_CLIENT_REGION_URL", r.AuthEndpointOverride); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_SDK_AUTH_CLIENT_REGION_URL")
		}
	}

	resourcePrincipalConfig, err := opts.factory().NewResourcePrincipal()
	if err != nil {
		return nil, fmt.Errorf("couldn't get resource principal config provider: %v", err)
	}

	opts.Log.Info("Initialized resource principal configuration provider")

	return resourcePrincipalConfig, nil
}

// DefaultResourcePrincipalConfig provides default configuration
// for resource principal authentication.
func DefaultResourcePrincipalConfig() ResourcePrincipalConfig {
	return ResourcePrincipalConfig{}
}
