package azure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents Azure authentication configuration
type Config struct {
	// Common fields
	TenantID string `json:"tenant_id,omitempty" mapstructure:"tenant_id"`
	ClientID string `json:"client_id,omitempty" mapstructure:"client_id"`

	// Client Secret auth
	ClientSecret string `json:"client_secret,omitempty" mapstructure:"client_secret"`

	// Client Certificate auth
	CertificatePath string `json:"certificate_path,omitempty" mapstructure:"certificate_path"`
	CertificateData []byte `json:"certificate_data,omitempty" mapstructure:"certificate_data"`
	Password        string `json:"password,omitempty" mapstructure:"password"`

	// Managed Identity auth
	ResourceID string `json:"resource_id,omitempty" mapstructure:"resource_id"`

	// Storage Account Key auth
	AccountName string `json:"account_name,omitempty" mapstructure:"account_name"`
	AccountKey  string `json:"account_key,omitempty" mapstructure:"account_key"`

	// Optional fields
	AuthorityHost string   `json:"authority_host,omitempty" mapstructure:"authority_host"`
	Scopes        []string `json:"scopes,omitempty" mapstructure:"scopes"`
}

// LoadFromEnvironment loads Azure configuration from environment variables
func (c *Config) LoadFromEnvironment() {
	// Common fields
	if c.TenantID == "" {
		c.TenantID = os.Getenv("AZURE_TENANT_ID")
	}
	if c.ClientID == "" {
		c.ClientID = os.Getenv("AZURE_CLIENT_ID")
	}

	// Client Secret
	if c.ClientSecret == "" {
		c.ClientSecret = os.Getenv("AZURE_CLIENT_SECRET")
	}

	// Client Certificate
	if c.CertificatePath == "" {
		c.CertificatePath = os.Getenv("AZURE_CLIENT_CERTIFICATE_PATH")
	}
	if c.Password == "" {
		c.Password = os.Getenv("AZURE_CLIENT_CERTIFICATE_PASSWORD")
	}

	// Managed Identity
	if c.ResourceID == "" {
		c.ResourceID = os.Getenv("AZURE_CLIENT_RESOURCE_ID")
	}

	// Storage Account
	if c.AccountName == "" {
		c.AccountName = os.Getenv("AZURE_STORAGE_ACCOUNT_NAME")
	}
	if c.AccountKey == "" {
		c.AccountKey = os.Getenv("AZURE_STORAGE_ACCOUNT_KEY")
	}

	// Authority Host
	if c.AuthorityHost == "" {
		c.AuthorityHost = os.Getenv("AZURE_AUTHORITY_HOST")
	}
}

// ExpandPaths expands any paths that use ~ or environment variables
func (c *Config) ExpandPaths() error {
	if c.CertificatePath != "" {
		expanded, err := expandPath(c.CertificatePath)
		if err != nil {
			return fmt.Errorf("failed to expand certificate path: %w", err)
		}
		c.CertificatePath = expanded
	}
	return nil
}

// expandPath expands ~ and environment variables in a file path
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path), nil
}

// Validate performs basic validation on the configuration
func (c *Config) Validate() error {
	// No validation needed for default auth
	if c.TenantID == "" && c.ClientID == "" && c.AccountName == "" && c.ClientSecret == "" &&
		c.CertificatePath == "" && len(c.CertificateData) == 0 && c.AccountKey == "" {
		return nil
	}

	// Count how many auth methods are configured
	authMethodCount := 0
	var errors []string

	// Validate account key auth
	if c.AccountName != "" || c.AccountKey != "" {
		authMethodCount++
		if c.AccountName == "" {
			errors = append(errors, "account_name is required when account_key is provided")
		}
		if c.AccountKey == "" {
			errors = append(errors, "account_key is required when account_name is provided")
		}
	}

	// Validate client secret auth
	if c.ClientSecret != "" {
		authMethodCount++
		if c.TenantID == "" {
			errors = append(errors, "tenant_id is required for client secret authentication")
		}
		if c.ClientID == "" {
			errors = append(errors, "client_id is required for client secret authentication")
		}
	}

	// Validate client certificate auth
	if c.CertificatePath != "" || len(c.CertificateData) > 0 {
		authMethodCount++
		if c.TenantID == "" {
			errors = append(errors, "tenant_id is required for client certificate authentication")
		}
		if c.ClientID == "" {
			errors = append(errors, "client_id is required for client certificate authentication")
		}
		if c.CertificatePath != "" && len(c.CertificateData) > 0 {
			errors = append(errors, "cannot specify both certificate_path and certificate_data")
		}
	}

	// Check for mutually exclusive auth methods
	if authMethodCount > 1 {
		errors = append(errors, "multiple authentication methods configured; only one method should be specified")
	}

	// Return combined errors
	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetConfig returns the Config as a map for compatibility
func (c *Config) GetConfig() map[string]interface{} {
	config := make(map[string]interface{})

	if c.TenantID != "" {
		config["tenant_id"] = c.TenantID
	}
	if c.ClientID != "" {
		config["client_id"] = c.ClientID
	}
	if c.ClientSecret != "" {
		config["client_secret"] = c.ClientSecret
	}
	if c.CertificatePath != "" {
		config["certificate_path"] = c.CertificatePath
	}
	if len(c.CertificateData) > 0 {
		config["certificate_data"] = c.CertificateData
	}
	if c.Password != "" {
		config["password"] = c.Password
	}
	if c.ResourceID != "" {
		config["resource_id"] = c.ResourceID
	}
	if c.AccountName != "" {
		config["account_name"] = c.AccountName
	}
	if c.AccountKey != "" {
		config["account_key"] = c.AccountKey
	}
	if c.AuthorityHost != "" {
		config["authority_host"] = c.AuthorityHost
	}
	if len(c.Scopes) > 0 {
		config["scopes"] = c.Scopes
	}

	return config
}
