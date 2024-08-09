package principals

import "github.com/oracle/oci-go-sdk/v65/common"

// UserPrincipalConfig holds the configuration details for API key-based authentication.
type UserPrincipalConfig struct {
	TenancyOCID    string
	UserOCID       string
	Region         string
	PrivateKeyPath string
	Fingerprint    string
}

// UserPrincipalAuthProvider provides API key-based authentication.
// It supports initializing from direct values or loading from a configuration file.
type UserPrincipalAuthProvider struct {
	Config     *UserPrincipalConfig // Direct configuration values, if provided
	ConfigPath string               // Path to the OCI configuration file
	Profile    string               // Profile name to use within the configuration file
}

// ConfigurationProvider attempts to return a configuration provider based on the provided details.
func (upa *UserPrincipalAuthProvider) ConfigurationProvider() (common.ConfigurationProvider, error) {
	// If direct config is provided, use it to create a RawConfigurationProvider
	if upa.Config != nil {
		return common.NewRawConfigurationProvider(
			upa.Config.TenancyOCID,
			upa.Config.UserOCID,
			upa.Config.Region,
			upa.Config.Fingerprint,
			upa.Config.PrivateKeyPath,
			nil, // privateKey password, if your key is encrypted
		), nil
	}
	// Otherwise, attempt to load the configuration from a file
	return common.ConfigurationProviderFromFileWithProfile(upa.ConfigPath, upa.Profile, "")
}
