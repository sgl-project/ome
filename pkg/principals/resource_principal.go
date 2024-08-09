package principals

import (
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/vars"
)

// ResourcePrincipalConfig encapsulates configuration for constructing
// resource principal authentication provider.
//
// Zero value is ready to use.
type ResourcePrincipalConfig struct {
	Version             *EnvVar `mapstructure:"version"`
	RPTEndpoint         *EnvVar `mapstructure:"rpt_endpoint"`
	RPTPath             *EnvVar `mapstructure:"rpt_path"`
	RPSTEndpoint        *EnvVar `mapstructure:"rpst_endpoint"`
	AuthClientRegionURL *EnvVar `mapstructure:"auth_client_region_url"`
}

// Validate validates r.
func (r ResourcePrincipalConfig) Validate() error {
	for name, envVar := range r.allEnvVars() {
		if err := envVar.Validate(); err != nil {
			return fmt.Errorf("invalid %s: %w", name, err)
		}
	}

	return nil
}

func (r ResourcePrincipalConfig) allEnvVars() map[string]*EnvVar {
	return map[string]*EnvVar{
		"version":                    r.Version,
		"rpt_endpoint":               r.RPTEndpoint,
		"rpt_path":                   r.RPTPath,
		"rpst_endpoint":              r.RPSTEndpoint,
		"sdk_auth_client_region_url": r.AuthClientRegionURL,
	}
}

// Build builds a configuration provider from r.
func (r ResourcePrincipalConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
	if err := r.setEnvVars(opts); err != nil {
		return nil, err
	}

	resourcePrincipalConfig, err := opts.factory().NewResourcePrincipal()
	if err != nil {
		return nil, fmt.Errorf("couldn't get resource principal config provider: %v", err)
	}

	opts.Log.WithField("resourcePrincipalConfig", resourcePrincipalConfig).
		WithField("tenancyId", env.TryResolve(opts.Env, vars.TenancyId.String())).
		WithField("resourceCompartmentId", env.TryResolve(opts.Env, vars.ResourceCompartmentId.String())).
		Info("Initialized resource principal configuration provider")

	return resourcePrincipalConfig, nil
}

func (r ResourcePrincipalConfig) setEnvVars(opts Opts) error {
	if err := r.AuthClientRegionURL.SetenvOrDefault(EnvOciSdkAuthClientRegionUrl, opts, DefaultAuthClientRegionURLOverlay); err != nil {
		return err
	}
	if err := r.Version.SetenvOrDefault(EnvResourcePrincipalVersion, opts, DefaultResourcePrincipalVersion11); err != nil {
		return err
	}
	if err := r.RPTEndpoint.SetenvOrDefault(EnvResourcePrincipalRPTEndpoint, opts, DefaultResourcePrincipalRPTEndpoint); err != nil {
		return err
	}
	if err := r.RPTPath.SetenvOrDefault(EnvResourcePrincipalRPTPath, opts, DefaultResourcePrincipalRPTPath); err != nil {
		return err
	}
	if err := r.RPSTEndpoint.SetenvOrDefault(EnvResourcePrincipalRPSTEndpoint, opts, DefaultResourcePrincipalRPSTEndpoint); err != nil {
		return err
	}

	// TODO(achebatu): unset these env variables to what they were after construction
	return nil
}

// DefaultResourcePrincipalConfig provides default configuration
// for resource principal authentication.
//
// Applies to overlay enclave only.
func DefaultResourcePrincipalConfig() ResourcePrincipalConfig {
	return ResourcePrincipalConfig{}
}
