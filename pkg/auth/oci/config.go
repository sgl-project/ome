package oci

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Default values for OCI configuration
const (
	DefaultConfigPath = "~/.oci/config"
	DefaultProfile    = "DEFAULT"
)

// UserPrincipalConfig encapsulates configuration for user principal authentication.
// This method uses API key-based authentication with OCI configuration files.
type UserPrincipalConfig struct {
	// ConfigPath is the path to the OCI configuration file
	ConfigPath string `mapstructure:"config_path" json:"config_path"`

	// Profile is the profile name within the configuration file
	Profile string `mapstructure:"profile" json:"profile"`

	// UseSessionToken enables session token authentication
	UseSessionToken bool `mapstructure:"use_session_token" json:"use_session_token"`
}

// Validate validates the user principal configuration
func (c UserPrincipalConfig) Validate() error {
	if strings.TrimSpace(c.ConfigPath) == "" {
		return errors.New("config_path is required for user principal")
	}
	if strings.TrimSpace(c.Profile) == "" {
		return errors.New("profile is required for user principal")
	}
	return nil
}

// ApplyEnvironment applies environment variables and defaults to the configuration
func (c *UserPrincipalConfig) ApplyEnvironment() {
	// Apply environment variables
	if c.ConfigPath == "" {
		if configPath, ok := os.LookupEnv("OCI_CONFIG_PATH"); ok {
			c.ConfigPath = configPath
		} else {
			// Expand home directory
			c.ConfigPath = expandPath(DefaultConfigPath)
		}
	}

	if c.Profile == "" {
		if profile, ok := os.LookupEnv("OCI_PROFILE"); ok {
			c.Profile = profile
		} else {
			c.Profile = DefaultProfile
		}
	}

	if useSessionToken, ok := os.LookupEnv("OCI_USE_SESSION_TOKEN"); ok {
		// Use strconv.ParseBool for more flexible boolean parsing
		// Accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False
		if parsed, err := strconv.ParseBool(useSessionToken); err == nil {
			c.UseSessionToken = parsed
		}
	}
}

// expandPath expands the home directory in a path
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}
