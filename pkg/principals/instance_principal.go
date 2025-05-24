package principals

import (
	"fmt"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// InstancePrincipalConfig encapsulates configuration for constructing
// instance principal authentication provider.
//
// Zero value is ready to use - SDK will auto-detect everything from IMDS.
type InstancePrincipalConfig struct {
	// Optional region override - if not set, SDK auto-detects from IMDS
	Region string `mapstructure:"region"`

	// Optional auth endpoint override - for special cases like gov clouds
	AuthEndpointOverride string `mapstructure:"auth_endpoint_override"`
}

// Validate validates the config.
func (c *InstancePrincipalConfig) Validate() error {
	// No validation needed - all fields are optional
	return nil
}

// DefaultInstancePrincipalConfig provides default configuration
// for instance principal authentication.
//
// Applies to overlay enclave only.
func DefaultInstancePrincipalConfig() InstancePrincipalConfig {
	return InstancePrincipalConfig{}
}

// Build builds instance principal configuration provider from c.
func (c *InstancePrincipalConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
	// Set optional overrides if provided
	if c.Region != "" {
		if err := os.Setenv("OCI_REGION", c.Region); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_REGION")
		}
	}

	if c.AuthEndpointOverride != "" {
		if err := os.Setenv("OCI_SDK_AUTH_CLIENT_REGION_URL", c.AuthEndpointOverride); err != nil {
			opts.Log.WithError(err).Warn("Failed to set OCI_SDK_AUTH_CLIENT_REGION_URL")
		}
	}

	result, err := opts.factory().NewInstancePrincipal()
	if err != nil {
		return nil, fmt.Errorf("creating instance principal: %v", err)
	}

	opts.Log.Info("Initialized instance principal configuration provider")

	return result, nil
}
