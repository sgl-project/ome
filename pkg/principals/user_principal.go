package principals

import (
	"errors"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// UserPrincipalConfig encapsulates configuration for constructing
// user principal authentication provider.
type UserPrincipalConfig struct {
	ConfigPath string `mapstructure:"config_path"`
	Profile    string `mapstructure:"profile"`
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

	result, err := newUserPrincipalConfigurationProvider(opts)
	if err != nil {
		return nil, fmt.Errorf("could not construct configuration provider: %w", err)
	}

	return result, nil
}

// TODO - implement this function and pass down UserPrincipalConfig from viper
func newUserPrincipalConfigurationProvider(opts Opts) (common.ConfigurationProvider, error) {

	return opts.factory().NewApiKeyUserPrincipal(
		"", "",
	)
}
