package imds

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	BaseEndpoint               string        `mapstructure:"base_endpoint"`
	FallbackBaseEndpoint       string        `mapstructure:"fallback_base_endpoint"`
	TimeoutAfter               time.Duration `mapstructure:"timeout_after"`
	AuthHeaderKey              string        `mapstructure:"auth_header_key"`
	AuthHeaderValue            string        `mapstructure:"auth_header_value"`
	InstanceEndpointSuffix     string        `mapstructure:"instance_endpoint_suffix"`
	IdentityCertEndpointSuffix string        `mapstructure:"identity_cert_endpoint_suffix"`
	IaasInfoEndpointSuffix     string        `mapstructure:"iaas_info_endpoint_suffix"`
}

func DefaultConfig() Config {
	return Config{
		BaseEndpoint:               "http://169.254.169.254/opc/v2",
		FallbackBaseEndpoint:       "http://169.254.169.254/opc/v1",
		TimeoutAfter:               60 * time.Second,
		AuthHeaderKey:              "Authorization",
		AuthHeaderValue:            "Bearer Oracle",
		InstanceEndpointSuffix:     "/instance",
		IdentityCertEndpointSuffix: "/identity/cert.pem",
		IaasInfoEndpointSuffix:     "/iaasInfo",
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

	return nil
}
