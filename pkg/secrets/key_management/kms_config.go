package key_management

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/spf13/viper"
)

const (
	KmsConfigViperKeyNameKey = "kms_config_viper_prefix"

	/*
	 * These Viper key name have to be consistent with mapstructure tags in the struct definition
	 */
	NameViperKeyName                  = "name"
	AuthTypeViperKeyName              = "auth_type"
	VaultPrefixViperKeyName           = "vault_prefix"
	VaultIdViperKeyName               = "vault_id"
	KmsCryptoEndpointViperKeyName     = "kms_crypto_endpoint"
	KmsManagementEndpointViperKeyName = "kms_management_endpoint"
)

type KmsConfig struct {
	AnotherLogger         logging.Interface
	Name                  string                         `mapstructure:"name"`
	AuthType              *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	VaultId               *string                        `mapstructure:"vault_id"`
	VaultPrefix           *string                        `mapstructure:"vault_prefix"`
	KmsCryptoEndpoint     *string                        `mapstructure:"kms_crypto_endpoint" validate:"required"`
	KmsManagementEndpoint *string                        `mapstructure:"kms_management_endpoint" validate:"required"`
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
func WithViper(v *viper.Viper) Option {
	return func(c *KmsConfig) error {
		prefix := v.GetString(KmsConfigViperKeyNameKey)
		if prefix != "" {
			prefix = prefix + "."
		}

		c.Name = v.GetString(fmt.Sprintf("%s%s", prefix, NameViperKeyName))
		c.VaultId = common.String(v.GetString(fmt.Sprintf("%s%s", prefix, VaultIdViperKeyName)))
		c.VaultPrefix = common.String(v.GetString(fmt.Sprintf("%s%s", prefix, VaultPrefixViperKeyName)))
		c.KmsCryptoEndpoint = common.String(v.GetString(fmt.Sprintf("%s%s", prefix, KmsCryptoEndpointViperKeyName)))
		c.KmsManagementEndpoint = common.String(v.GetString(fmt.Sprintf("%s%s", prefix, KmsManagementEndpointViperKeyName)))

		if err := v.UnmarshalKey(fmt.Sprintf("%s%s", prefix, AuthTypeViperKeyName), &c.AuthType); err != nil {
			return fmt.Errorf("error occurred when unmarshalling auth_type: %+v", err)
		}

		// Set up vault prefix via vault Id
		if c.VaultPrefix == nil || *c.VaultPrefix == "" {
			c.VaultPrefix = common.String(secrets.ResolveVaultPrefix(*c.VaultId))
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
	validate := validator.New()
	// Validate by using package validator
	if err := validate.Struct(c); err != nil {
		return err
	}

	// Validate further
	if *c.KmsCryptoEndpoint == "" {
		return errors.New("missing config variable - no kms_crypto_endpoint in KMS config struct")
	}
	if *c.KmsManagementEndpoint == "" {
		return errors.New("missing config variable - no kms_management_endpoint in KMS config struct")
	}
	return nil
}
