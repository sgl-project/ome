package principals

import (
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

// Factory defines the interface for creating various principals.
//
// This is used as a shim within tests and acts like an anti-corruption layer to oci-go-sdk.
type Factory interface {
	// NewInstancePrincipal returns a configuration provider for instance principals.
	NewInstancePrincipal() (common.ConfigurationProvider, error)

	// NewUserPrincipal creates a configuration provider that using
	// API key or security token to construct a user principal.
	NewUserPrincipal(configPath string, profile string, useSessionToken bool) (common.ConfigurationProvider, error)

	// NewResourcePrincipal returns a resource principal configuration provider using well known
	// environment variables to look up token information. The environment variables can either paths or contain the material value
	// of the keys. However, in the case of the keys and tokens paths and values can not be mixed.
	NewResourcePrincipal() (common.ConfigurationProvider, error)

	// NewOkeWorkloadIdentity returns a OKE workload identity principal configuration provider that grants OKE workloads access to OCI resources.
	NewOkeWorkloadIdentity() (common.ConfigurationProvider, error)
}

// commonAuthFactory uses oci-go-sdk common & auth packages.
type commonAuthFactory struct{}

func (c commonAuthFactory) NewInstancePrincipal() (common.ConfigurationProvider, error) {
	return auth.InstancePrincipalConfigurationProvider()
}

func (c commonAuthFactory) NewUserPrincipal(configPath string, profile string, useSessionToken bool) (common.ConfigurationProvider, error) {
	if useSessionToken {
		return common.CustomProfileSessionTokenConfigProvider(configPath, profile), nil
	} else {
		return common.CustomProfileConfigProvider(configPath, profile), nil
	}
}

func (c commonAuthFactory) NewResourcePrincipal() (common.ConfigurationProvider, error) {
	return auth.ResourcePrincipalConfigurationProvider()
}

func (c commonAuthFactory) NewOkeWorkloadIdentity() (common.ConfigurationProvider, error) {
	return auth.OkeWorkloadIdentityConfigurationProvider()
}

var defaultFactory Factory = &commonAuthFactory{}
