package oci

import (
	"errors"
	"os"
	"strings"
)

// UserPrincipalConfig encapsulates configuration for constructing
// user principal authentication provider.
type UserPrincipalConfig struct {
	ConfigPath      string `mapstructure:"config_path" json:"config_path"`
	Profile         string `mapstructure:"profile" json:"profile"`
	UseSessionToken bool   `mapstructure:"use_session_token" json:"use_session_token"`
}

// Validate validates the user principal config
func (c UserPrincipalConfig) Validate() error {
	if strings.TrimSpace(c.ConfigPath) == "" {
		return errors.New("config_path is required for user principal")
	}
	if strings.TrimSpace(c.Profile) == "" {
		return errors.New("profile is required for user principal")
	}
	return nil
}

// ApplyEnvironment applies environment variables to the config
func (c *UserPrincipalConfig) ApplyEnvironment() {
	if c.ConfigPath == "" {
		if configPath, ok := os.LookupEnv("OCI_CONFIG_PATH"); ok {
			c.ConfigPath = configPath
		}
	}
	if c.Profile == "" {
		if profile, ok := os.LookupEnv("OCI_PROFILE"); ok {
			c.Profile = profile
		}
	}
	if useSessionToken, ok := os.LookupEnv("OCI_USE_SESSION_TOKEN"); ok {
		c.UseSessionToken = strings.ToLower(useSessionToken) == "true"
	}
}

// InstancePrincipalConfig encapsulates configuration for instance principal
type InstancePrincipalConfig struct {
	// No specific configuration needed for instance principal
}

// ResourcePrincipalConfig encapsulates configuration for resource principal
type ResourcePrincipalConfig struct {
	// Resource principal uses environment variables
}

// OkeWorkloadIdentityConfig encapsulates configuration for OKE workload identity
type OkeWorkloadIdentityConfig struct {
	// OKE workload identity uses environment variables
}
