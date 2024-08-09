package key_management

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/principals"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/secrets"
	"errors"
	"fmt"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/spf13/viper"
	"strings"
)

const (
	VaultPrefixViperKeyNameSuffix           = "vault_prefix"
	VaultIdViperKeyNameSuffix               = "vault_id"
	KmsCryptoEndpointViperKeyNameSuffix     = "kms_crypto_endpoint"
	KmsManagementEndpointViperKeyNameSuffix = "kms_management_endpoint"
)

type KmsConfig struct {
	AnotherLogger         logging.Interface
	AuthType              *principals.AuthenticationType `mapstructure:"auth_type"`
	VaultId               *string                        `mapstructure:"vault_id"`
	VaultPrefix           *string                        `mapstructure:"vault_prefix"`
	KmsCryptoEndpoint     *string                        `mapstructure:"kms_crypto_endpoint"`
	KmsManagementEndpoint *string                        `mapstructure:"kms_management_endpoint"`
}

// Option represents a server configuration option.
type Option func(*KmsConfig) error

// Apply applies the given options to the configuration.
func (c *KmsConfig) Apply(opts ...Option) error {
	for _, o := range opts {
		if o == nil {
			continue
		}

		if err := o(c); err != nil {
			return err
		}
	}
	return nil
}

// NewKmsConfig builds and returns a new kms configuration from the given options.
func NewKmsConfig(opts ...Option) (*KmsConfig, error) {
	c := &KmsConfig{}
	if err := c.Apply(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

// WithViper attempts to resolve the configuration using Viper.
func WithViper(v *viper.Viper, viperKeyNames []string) Option {
	return func(c *KmsConfig) error {
		fmt.Printf("What viper looks like: %+v", viperKeyNames)

		// Set up config using viperKeyNames
		for _, keyName := range viperKeyNames {
			if strings.Contains(keyName, VaultPrefixViperKeyNameSuffix) {
				c.VaultPrefix = common.String(v.GetString(keyName))
				continue
			}
			if strings.Contains(keyName, VaultIdViperKeyNameSuffix) {
				c.VaultId = common.String(v.GetString(keyName))
				continue
			}
			if strings.Contains(keyName, KmsCryptoEndpointViperKeyNameSuffix) {
				c.KmsCryptoEndpoint = common.String(v.GetString(keyName))
				continue
			}
			if strings.Contains(keyName, KmsManagementEndpointViperKeyNameSuffix) {
				c.KmsManagementEndpoint = common.String(v.GetString(keyName))
				continue
			}
		}

		if c.VaultPrefix == nil || *c.VaultPrefix == "" {
			c.VaultPrefix = common.String(secrets.ResolveVaultPrefix(*c.VaultId))
		}

		if err := v.UnmarshalKey("auth_type", &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}
		return nil
	}
}

// WithEnv attempts to resolve the configuration using Environment module.
func WithEnv(env *env.Environment) Option {
	return func(c *KmsConfig) error {
		if err := c.SetEndpoints(env); err != nil {
			return fmt.Errorf("error occurred set up kms endpoints: %+v", err)
		}
		return nil
	}
}

// WithAnotherLog specifies the logger.
func WithAnotherLog(logger logging.Interface) Option {
	return func(c *KmsConfig) error {
		if logger == nil {
			return errors.New("nil another logger")
		}

		c.AnotherLogger = logger
		return nil
	}
}

func (c *KmsConfig) SetEndpoints(env *env.Environment) error {
	if c.KmsCryptoEndpoint == nil || *c.KmsCryptoEndpoint == "" {
		kmsCryptoEndpointEnvVar, err := c.buildKmsCryptoEndpoint()
		if err != nil {
			return err
		}

		value, err := kmsCryptoEndpointEnvVar.ResolveValue(env)
		if err != nil {
			return fmt.Errorf("cannot resolve KMS Crypto Endpoint: %+v", err)
		}
		c.KmsCryptoEndpoint = &value
	}

	if c.KmsManagementEndpoint == nil || *c.KmsManagementEndpoint == "" {
		kmsManagementEndpointEnvVar, err := c.buildKmsManagementEndpoint()
		if err != nil {
			return err
		}
		value, err := kmsManagementEndpointEnvVar.ResolveValue(env)
		if err != nil {
			return fmt.Errorf("cannot resolve KMS Management Endpoint: %+v", err)
		}
		c.KmsManagementEndpoint = &value
	}
	return nil
}

func (c *KmsConfig) buildKmsCryptoEndpoint() (*secrets.EnvVar, error) {
	if c.VaultPrefix == nil || *c.VaultPrefix == "" {
		return nil, fmt.Errorf("cannot build KMS Crypto Endpoint: VaultPrefix is nil/empty")
	}
	nonOCICryptoEndpointTemplate := fmt.Sprintf("https://%s-crypto.kms.${region}.${realmTLD}", *c.VaultPrefix)
	OCICryptoEndpointTemplate := fmt.Sprintf("https://%s-crypto.kms.${region}.oci.${realmTLD}", *c.VaultPrefix)

	return secrets.EnsureValid(&secrets.EnvVar{
		ValueByRegion: env.StringByRegion{
			"us-seattle-1":   nonOCICryptoEndpointTemplate,
			"us-ashburn-1":   nonOCICryptoEndpointTemplate,
			"us-phoenix-1":   nonOCICryptoEndpointTemplate,
			"eu-frankfurt-1": nonOCICryptoEndpointTemplate,
			"uk-london-1":    nonOCICryptoEndpointTemplate,
			"ap-melbourne-1": nonOCICryptoEndpointTemplate,
			"ap-mumbai-1":    nonOCICryptoEndpointTemplate,
			"ap-osaka-1":     nonOCICryptoEndpointTemplate,
			"ap-seoul-1":     nonOCICryptoEndpointTemplate,
			"ap-sydney-1":    nonOCICryptoEndpointTemplate,
			"ap-tokyo-1":     nonOCICryptoEndpointTemplate,
			"eu-amsterdam-1": nonOCICryptoEndpointTemplate,
			"eu-zurich-1":    nonOCICryptoEndpointTemplate,
			"ca-montreal-1":  nonOCICryptoEndpointTemplate,
			"ca-toronto-1":   nonOCICryptoEndpointTemplate,
			"sa-saopaulo-1":  nonOCICryptoEndpointTemplate,
			"me-jeddah-1":    nonOCICryptoEndpointTemplate,
			"default":        OCICryptoEndpointTemplate,
		},
	}), nil
}

func (c *KmsConfig) buildKmsManagementEndpoint() (*secrets.EnvVar, error) {
	if c.VaultPrefix == nil || *c.VaultPrefix == "" {
		return nil, fmt.Errorf("cannot build KMS Management Endpoint: VaultPrefix is nil")
	}
	NonOCIManagementEndpointTemplate := fmt.Sprintf("https://%s-management.kms.${region}.${realmTLD}", *c.VaultPrefix)
	OCIManagementEndpointTemplate := fmt.Sprintf("https://%s-management.kms.${region}.oci.${realmTLD}", *c.VaultPrefix)

	return secrets.EnsureValid(&secrets.EnvVar{
		ValueByRegion: env.StringByRegion{
			"us-seattle-1":   NonOCIManagementEndpointTemplate,
			"us-ashburn-1":   NonOCIManagementEndpointTemplate,
			"us-phoenix-1":   NonOCIManagementEndpointTemplate,
			"eu-frankfurt-1": NonOCIManagementEndpointTemplate,
			"uk-london-1":    NonOCIManagementEndpointTemplate,
			"ap-melbourne-1": NonOCIManagementEndpointTemplate,
			"ap-mumbai-1":    NonOCIManagementEndpointTemplate,
			"ap-osaka-1":     NonOCIManagementEndpointTemplate,
			"ap-seoul-1":     NonOCIManagementEndpointTemplate,
			"ap-sydney-1":    NonOCIManagementEndpointTemplate,
			"ap-tokyo-1":     NonOCIManagementEndpointTemplate,
			"eu-amsterdam-1": NonOCIManagementEndpointTemplate,
			"eu-zurich-1":    NonOCIManagementEndpointTemplate,
			"ca-montreal-1":  NonOCIManagementEndpointTemplate,
			"ca-toronto-1":   NonOCIManagementEndpointTemplate,
			"sa-saopaulo-1":  NonOCIManagementEndpointTemplate,
			"me-jeddah-1":    NonOCIManagementEndpointTemplate,
			"default":        OCIManagementEndpointTemplate,
		},
	}), nil
}

func (c *KmsConfig) Validate() error {
	if c.AuthType == nil {
		return errors.New("missing config variable - no auth_type in KMS config struct")
	}
	if c.KmsCryptoEndpoint == nil || *c.KmsCryptoEndpoint == "" {
		return errors.New("missing config variable - no kms_crypto_endpoint in KMS config struct")
	}
	if c.KmsManagementEndpoint == nil || *c.KmsManagementEndpoint == "" {
		return errors.New("missing config variable - no kms_management_endpoint in KMS config struct")
	}
	return nil
}
