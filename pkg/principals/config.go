package principals

import (
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// AuthenticationType is the enum for various authentication types.
type AuthenticationType string

const (
	UserPrincipal       AuthenticationType = "UserPrincipal"
	InstancePrincipal   AuthenticationType = "InstancePrincipal"
	ResourcePrincipal   AuthenticationType = "ResourcePrincipal"
	OkeWorkloadIdentity AuthenticationType = "OkeWorkloadIdentity"
)

// Config represents the configuration required to construct
// various authentication principals (a.k.a. oci-go-sdk ConfigurationProviders).
//
// This structure is meant to be used with spf13/viper.
type Config struct {
	// AuthType specifies which type of authentication to use.
	AuthType AuthenticationType `mapstructure:"auth_type"`

	// IPConfig encapsulates configuration for constructing
	// instance principal authentication.
	IPConfig InstancePrincipalConfig `mapstructure:"instance_principal"`

	// RPConfig encapsulates configuration for constructing
	// resource principal authentication.
	RPConfig ResourcePrincipalConfig `mapstructure:"resource_principal"`

	// OkeWIConfig encapsulates configuration for OKE workload identity
	OkeWIConfig OkeWorkloadIdentityConfig `mapstructure:"oke_workload_identity"`

	// UPConfig encapsulates configuration for constructing
	UPConfig UserPrincipalConfig `mapstructure:"user_principal"`

	// Fallback encapsulates a fallback configuration, which
	// is used in case the primary configuration provider wasn't constructed.
	//
	// Fallbacks can be chained.
	Fallback *Config `mapstructure:"fallback"`
}

// Validate validated the principal configuration.
func (c Config) Validate() error {
	switch c.AuthType {
	case InstancePrincipal:
		if err := c.IPConfig.Validate(); err != nil {
			return fmt.Errorf("invalid instance_principal config: %w", err)
		}
	case ResourcePrincipal:
		if err := c.RPConfig.Validate(); err != nil {
			return fmt.Errorf("invalid resource_principal config: %w", err)
		}
	case UserPrincipal:
		if err := c.UPConfig.Validate(); err != nil {
			return fmt.Errorf("invalid user_principal config: %w", err)
		}
	case OkeWorkloadIdentity:
		if err := c.OkeWIConfig.Validate(); err != nil {
			return fmt.Errorf("invalid oke_workload_identity config: %w", err)
		}
	default:
		return fmt.Errorf("invalid auth_type: %s", c.AuthType)
	}

	if c.Fallback != nil {
		return c.Fallback.Validate()
	}

	return nil
}

// Build builds an oci-go-sdk common.ConfigurationProvider from c.
func (c Config) Build(opts Opts) (common.ConfigurationProvider, error) {
	opts.Log.WithField("AuthType", c.AuthType).Info("Config provider principal type")

	var (
		confProvider common.ConfigurationProvider
		err          error
	)

	switch c.AuthType {
	case InstancePrincipal:
		confProvider, err = c.IPConfig.Build(opts)
	case ResourcePrincipal:
		confProvider, err = c.RPConfig.Build(opts)
	case OkeWorkloadIdentity:
		confProvider, err = c.OkeWIConfig.Build(opts)
	case UserPrincipal:
		confProvider, err = c.UPConfig.Build(opts)
	default:
		return nil, fmt.Errorf("unknown auth_type: %v", c.AuthType)
	}

	if err == nil {
		return confProvider, nil
	}

	if c.Fallback != nil {
		opts.Log.WithError(err).Warn("Failed to build configuration provider. Trying fallback..")
		return c.Fallback.Build(opts)
	}

	return nil, err
}
