package aws

import (
	"fmt"
	"time"
)

// AccessKeyConfig represents AWS access key configuration
type AccessKeyConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" json:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token" json:"session_token,omitempty"`
}

// Validate validates the access key configuration
func (c *AccessKeyConfig) Validate() error {
	if c.AccessKeyID == "" {
		return fmt.Errorf("access_key_id is required")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("secret_access_key is required")
	}
	return nil
}

// AssumeRoleConfig represents AWS assume role configuration
type AssumeRoleConfig struct {
	RoleARN         string            `mapstructure:"role_arn" json:"role_arn"`
	RoleSessionName string            `mapstructure:"role_session_name" json:"role_session_name,omitempty"`
	ExternalID      string            `mapstructure:"external_id" json:"external_id,omitempty"`
	Duration        time.Duration     `mapstructure:"duration" json:"duration,omitempty"`
	Tags            map[string]string `mapstructure:"tags" json:"tags,omitempty"`
}

// Validate validates the assume role configuration
func (c *AssumeRoleConfig) Validate() error {
	if c.RoleARN == "" {
		return fmt.Errorf("role_arn is required")
	}
	return nil
}

// WebIdentityConfig represents AWS web identity configuration
type WebIdentityConfig struct {
	RoleARN         string `mapstructure:"role_arn" json:"role_arn"`
	TokenFile       string `mapstructure:"token_file" json:"token_file"`
	RoleSessionName string `mapstructure:"role_session_name" json:"role_session_name,omitempty"`
}

// Validate validates the web identity configuration
func (c *WebIdentityConfig) Validate() error {
	if c.RoleARN == "" {
		return fmt.Errorf("role_arn is required for web identity")
	}
	if c.TokenFile == "" {
		return fmt.Errorf("token_file is required for web identity")
	}
	return nil
}

// ECSTaskRoleConfig represents ECS task role configuration
type ECSTaskRoleConfig struct {
	// RelativeURI is the relative URI to the ECS credentials endpoint
	// If not specified, it will be read from AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
	RelativeURI string `mapstructure:"relative_uri" json:"relative_uri,omitempty"`

	// FullURI is the full URI to the ECS credentials endpoint
	// If not specified, it will be read from AWS_CONTAINER_CREDENTIALS_FULL_URI
	FullURI string `mapstructure:"full_uri" json:"full_uri,omitempty"`

	// AuthorizationToken is used for authentication with the ECS credentials endpoint
	// If not specified, it will be read from AWS_CONTAINER_AUTHORIZATION_TOKEN
	AuthorizationToken string `mapstructure:"authorization_token" json:"authorization_token,omitempty"`
}

// Validate validates the ECS task role configuration
func (c *ECSTaskRoleConfig) Validate() error {
	// Either RelativeURI or FullURI must be specified
	if c.RelativeURI == "" && c.FullURI == "" {
		return fmt.Errorf("either relative_uri or full_uri must be specified for ECS task role")
	}
	return nil
}

// ProcessConfig represents process credentials provider configuration
type ProcessConfig struct {
	// Command is the command to execute to retrieve credentials
	Command string `mapstructure:"command" json:"command"`

	// Timeout is the maximum time to wait for the process to complete
	Timeout time.Duration `mapstructure:"timeout" json:"timeout,omitempty"`
}

// Validate validates the process configuration
func (c *ProcessConfig) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("command is required for process credentials provider")
	}
	return nil
}
