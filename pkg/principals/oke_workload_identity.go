package principals

import (
	"fmt"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// OkeWorkloadIdentityConfig encapsulates configuration for OKE workload identity
//
// Zero value is ready to use - SDK handles this automatically.
type OkeWorkloadIdentityConfig struct {
	// Optional version override (defaults to "1.1" if not set)
	Version string `mapstructure:"version"`

	// Optional region override
	Region string `mapstructure:"region"`
}

// Validate validates r.
func (r OkeWorkloadIdentityConfig) Validate() error {
	// No validation needed - all fields are optional
	return nil
}

// Build builds a configuration provider from r.
func (r OkeWorkloadIdentityConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
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

	okeWorkloadIdentityConfig, err := opts.factory().NewOkeWorkloadIdentity()
	if err != nil {
		return nil, fmt.Errorf("couldn't get oke workload identity config provider: %v", err)
	}

	opts.Log.Info("Initialized OKE workload identity configuration provider")

	return okeWorkloadIdentityConfig, nil
}

// DefaultOkeWorkloadIdentityConfig provides default configuration for OKE workload identity.
func DefaultOkeWorkloadIdentityConfig() OkeWorkloadIdentityConfig {
	return OkeWorkloadIdentityConfig{}
}
