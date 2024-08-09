package principals

import (
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
)

// InstancePrincipalConfig encapsulates configuration for constructing
// instance principal authentication provider.
//
// Zero value is ready to use.
type InstancePrincipalConfig struct {
	AuthClientRegionUrl *EnvVar `mapstructure:"auth_client_region_url"`
}

// Validate validates the config.
func (c *InstancePrincipalConfig) Validate() error {
	if err := c.AuthClientRegionUrl.Validate(); err != nil {
		return fmt.Errorf("invalid auth_client_region_url: %w", err)
	}

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
	if err := c.AuthClientRegionUrl.SetenvOrDefault(EnvOciSdkAuthClientRegionUrl, opts, DefaultAuthClientRegionURLOverlay); err != nil {
		return nil, err
	}

	result, err := opts.factory().NewInstancePrincipal()
	if err != nil {
		return nil, fmt.Errorf("creating instance principal: %v", err)
	}

	opts.Log.
		WithField("resolved tenancyId", env.TryResolve(opts.Env, vars.TenancyId.String())).
		WithField("resolved compartmentId", env.TryResolve(opts.Env, vars.InstanceCompartmentId.String())).
		Info("Initialized instance principal configuration provider")

	return result, nil
}
