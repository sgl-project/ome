package principals

import (
	"errors"
	"os"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
)

// UserPrincipalConfig encapsulates configuration for constructing
// user principal authentication provider.
type UserPrincipalConfig struct {
	ConfigPath      string `mapstructure:"config_path"`
	Profile         string `mapstructure:"profile"`
	UseSessionToken bool   `mapstructure:"use_session_token"`
}

// Validate validates c.
func (c UserPrincipalConfig) Validate() error {
	if c.ConfigPath == "" {
		return errors.New("nil user_principal.config_path")
	}
	if c.Profile == "" {
		return errors.New("nil user_principal.profile")
	}
	return nil
}

// Build builds a user principal from c.
func (c UserPrincipalConfig) Build(opts Opts) (common.ConfigurationProvider, error) {
	if configPath, ok := os.LookupEnv("OCI_CONFIG_PATH"); ok {
		c.ConfigPath = configPath
	}
	if profile, ok := os.LookupEnv("PROFILE"); ok {
		c.Profile = profile
	}
	if useSessionToken, ok := os.LookupEnv("USE_SESSION_TOKEN"); ok {
		c.UseSessionToken = strings.ToLower(useSessionToken) == "true"
	}

	return opts.factory().NewUserPrincipal(
		c.ConfigPath, c.Profile, c.UseSessionToken,
	)
}
