package imds

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	// BaseEndpoint is the v2 IMDS base URL
	BaseEndpoint string `mapstructure:"base_endpoint"`

	// FallbackBaseEndpoint is the v1 IMDS base URL
	FallbackBaseEndpoint string `mapstructure:"fallback_base_endpoint"`

	// TimeoutAfter defines the REST call timeout duration
	TimeoutAfter time.Duration `mapstructure:"timeout_after"`

	// AuthHeaderKey is the HEADER key value required to pass in for authz by IMDS
	AuthHeaderKey string `mapstructure:"auth_header_key"`

	// AuthHeaderValue is the HEADER value required by the AuthHeaderKey
	AuthHeaderValue string `mapstructure:"auth_header_value"`

	// InstanceEndpointSuffix appended to the base endpoint to get the instance metadata
	InstanceEndpointSuffix string `mapstructure:"instance_endpoint_suffix"`

	// IdentityCertEndpointSuffix appended to the base endpoint to get the leaf cert from IMDS
	IdentityCertEndpointSuffix string `mapstructure:"identity_cert_endpoint_suffix"`

	// IdentityCertPrivateKeyEndpointSuffix appended to the base endpoint to get the leaf cert's private key from IMDS
	IdentityCertPrivateKeyEndpointSuffix string `mapstructure:"identity_cert_private_key_endpoint_suffix"`

	// IdentityCertEndpointSuffix appended to the base endpoint to get the leaf cert from IMDS
	IdentityIntermediateCertEndpointSuffix string `mapstructure:"identity_intermediate_cert_endpoint_suffix"`

	// IaasInfoEndpointSuffix appended to the base endpoint to get the IaasInfo for the instance from IMDS
	IaasInfoEndpointSuffix string `mapstructure:"iaas_info_endpoint_suffix"`
}

func DefaultConfig() Config {
	return Config{
		BaseEndpoint:                           "http://169.254.169.254/opc/v2",
		FallbackBaseEndpoint:                   "http://169.254.169.254/opc/v1",
		TimeoutAfter:                           60 * time.Second,
		AuthHeaderKey:                          "Authorization",
		AuthHeaderValue:                        "Bearer Oracle",
		InstanceEndpointSuffix:                 "/instance",
		IdentityCertEndpointSuffix:             "/identity/cert.pem",
		IdentityCertPrivateKeyEndpointSuffix:   "/identity/key.pem",
		IdentityIntermediateCertEndpointSuffix: "/identity/intermediate.pem",
		IaasInfoEndpointSuffix:                 "/iaasInfo",
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("nil config")
	}
	if c.BaseEndpoint == "" {
		return errors.New("base_endpoint empty")
	}
	if c.TimeoutAfter <= 0 {
		return fmt.Errorf("timeout_after non positive: %d", c.TimeoutAfter)
	}
	if c.AuthHeaderKey == "" {
		return errors.New("auth_header_key empty")
	}
	if c.AuthHeaderValue == "" {
		return errors.New("auth_header_value empty")
	}
	if c.InstanceEndpointSuffix == "" {
		return errors.New("instance_endpoint_suffix empty")
	}
	if c.IdentityCertEndpointSuffix == "" {
		return errors.New("identity_cert_endpoint_suffix empty")
	}
	if c.IaasInfoEndpointSuffix == "" {
		return errors.New("iaas_info_endpoint_suffix empty")
	}
	if c.IdentityCertPrivateKeyEndpointSuffix == "" {
		return errors.New("identity_cert_private_key_endpoint_suffix empty")
	}
	if c.IdentityIntermediateCertEndpointSuffix == "" {
		return errors.New("identity_intermediate_cert_endpoint_suffix empty")
	}

	return nil
}
