package principals

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env/vars"
	"github.com/oracle/oci-go-sdk/v65/common"
)

type OkeWorkloadIdentityConfig struct {
	Version  *EnvVar `mapstructure:"version"` // Both 1.1 and 2.2 works
	RPRegion *EnvVar `mapstructure:"region"`
}

// Validate validates r.
func (r OkeWorkloadIdentityConfig) Validate() error {
	for name, envVar := range r.allEnvVars() {
		if err := envVar.Validate(); err != nil {
			return fmt.Errorf("invalid %s: %w", name, err)
		}
	}
	return nil
}

func (r OkeWorkloadIdentityConfig) allEnvVars() map[string]*EnvVar {
	return map[string]*EnvVar{
		"version": r.Version,
		"region":  r.RPRegion,
	}
}

// Build builds a configuration provider from r.
func (r OkeWorkloadIdentityConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
	if err := r.setEnvVars(opts); err != nil {
		return nil, err
	}

	okeWorkloadIdentityConfig, err := opts.factory().NewOkeWorkloadIdentity()
	if err != nil {
		return nil, fmt.Errorf("couldn't get oke workload identity config provider: %v", err)
	}

	opts.Log.WithField("okeWorkloadIdentityConfig", okeWorkloadIdentityConfig).
		WithField("tenancyId", env.TryResolve(opts.Env, vars.TenancyId.String())).
		WithField("resourceCompartmentId", env.TryResolve(opts.Env, vars.ResourceCompartmentId.String())).
		Info("Initialized OKE workload identity configuration provider")

	return okeWorkloadIdentityConfig, nil
}

func (r OkeWorkloadIdentityConfig) setEnvVars(opts Opts) error {
	if err := r.RPRegion.SetenvOrDefault(EnvResourcePrincipalRegion, opts, DefaultRegion); err != nil {
		return err
	}
	if err := r.Version.SetenvOrDefault(EnvResourcePrincipalVersion, opts, DefaultResourcePrincipalVersion11); err != nil {
		return err
	}

	// TODO(achebatu): unset these env variables to what they were after construction
	return nil
}

func DefaultOkeWorkloadIdentityConfig() OkeWorkloadIdentityConfig {
	return OkeWorkloadIdentityConfig{}
}
