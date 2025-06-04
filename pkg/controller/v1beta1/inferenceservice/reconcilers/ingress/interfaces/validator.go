package interfaces

import (
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
)

// ConfigValidator validates ingress configurations
type ConfigValidator interface {
	ValidateIngressConfig(config *controllerconfig.IngressConfig) error
	ValidateInferenceServiceConfig(config *controllerconfig.InferenceServicesConfig) error
}

// IngressConfigValidator validates ingress-specific configurations
type IngressConfigValidator interface {
	ConfigValidator
	ValidateGatewayConfig(config *controllerconfig.IngressConfig) error
	ValidatePathTemplate(pathTemplate string) error
	ValidateDomainTemplate(domainTemplate string) error
}
