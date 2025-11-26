package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestResolveIngressConfig(t *testing.T) {
	// Base config from ConfigMap
	baseConfig := &controllerconfig.IngressConfig{
		IngressGateway:          "knative-serving/knative-ingress-gateway",
		IngressServiceName:      "istio-ingressgateway.istio-system.svc.cluster.local",
		IngressDomain:           "svc.cluster.local",
		DomainTemplate:          "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
		UrlScheme:               "http",
		PathTemplate:            "",
		DisableIstioVirtualHost: false,
		DisableIngressCreation:  false,
	}

	tests := []struct {
		name        string
		annotations map[string]string
		expected    *controllerconfig.IngressConfig
	}{
		{
			name:        "no annotations - returns base config",
			annotations: map[string]string{},
			expected:    baseConfig,
		},
		{
			name: "custom domain template override",
			annotations: map[string]string{
				constants.IngressDomainTemplate: "{{ .Name }}-custom.example.com",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:          "knative-serving/knative-ingress-gateway",
				IngressServiceName:      "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:           "svc.cluster.local",
				DomainTemplate:          "{{ .Name }}-custom.example.com",
				UrlScheme:               "http",
				PathTemplate:            "",
				DisableIstioVirtualHost: false,
				DisableIngressCreation:  false,
			},
		},
		{
			name: "custom domain and URL scheme",
			annotations: map[string]string{
				constants.IngressDomain:    "my-domain.com",
				constants.IngressURLScheme: "https",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:          "knative-serving/knative-ingress-gateway",
				IngressServiceName:      "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:           "my-domain.com",
				DomainTemplate:          "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
				UrlScheme:               "https",
				PathTemplate:            "",
				DisableIstioVirtualHost: false,
				DisableIngressCreation:  false,
			},
		},
		{
			name: "additional domains with comma separation",
			annotations: map[string]string{
				constants.IngressAdditionalDomains: "alt1.com, alt2.com, alt3.com",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:           "knative-serving/knative-ingress-gateway",
				IngressServiceName:       "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:            "svc.cluster.local",
				DomainTemplate:           "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
				UrlScheme:                "http",
				PathTemplate:             "",
				DisableIstioVirtualHost:  false,
				DisableIngressCreation:   false,
				AdditionalIngressDomains: &[]string{"alt1.com", "alt2.com", "alt3.com"},
			},
		},
		{
			name: "boolean overrides",
			annotations: map[string]string{
				constants.IngressDisableIstioVirtualHost: "true",
				constants.IngressDisableCreation:         "true",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:          "knative-serving/knative-ingress-gateway",
				IngressServiceName:      "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:           "svc.cluster.local",
				DomainTemplate:          "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
				UrlScheme:               "http",
				PathTemplate:            "",
				DisableIstioVirtualHost: true,
				DisableIngressCreation:  true,
			},
		},
		{
			name: "path template override",
			annotations: map[string]string{
				constants.IngressPathTemplate: "/api/v1/models/{{ .Name }}",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:          "knative-serving/knative-ingress-gateway",
				IngressServiceName:      "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:           "svc.cluster.local",
				DomainTemplate:          "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
				UrlScheme:               "http",
				PathTemplate:            "/api/v1/models/{{ .Name }}",
				DisableIstioVirtualHost: false,
				DisableIngressCreation:  false,
			},
		},
		{
			name: "comprehensive override",
			annotations: map[string]string{
				constants.IngressDomainTemplate:          "{{ .Name }}-prod.company.com",
				constants.IngressDomain:                  "company.com",
				constants.IngressURLScheme:               "https",
				constants.IngressPathTemplate:            "/ml/{{ .Name }}",
				constants.IngressAdditionalDomains:       "backup.com,mirror.net",
				constants.IngressDisableIstioVirtualHost: "false",
				constants.IngressDisableCreation:         "false",
			},
			expected: &controllerconfig.IngressConfig{
				IngressGateway:           "knative-serving/knative-ingress-gateway",
				IngressServiceName:       "istio-ingressgateway.istio-system.svc.cluster.local",
				IngressDomain:            "company.com",
				DomainTemplate:           "{{ .Name }}-prod.company.com",
				UrlScheme:                "https",
				PathTemplate:             "/ml/{{ .Name }}",
				DisableIstioVirtualHost:  false,
				DisableIngressCreation:   false,
				AdditionalIngressDomains: &[]string{"backup.com", "mirror.net"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveIngressConfig(baseConfig, tt.annotations)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDeploymentModeFromAnnotations(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		expectedMode  constants.DeploymentModeType
		expectedFound bool
	}{
		{
			name:          "nil annotations - returns empty and false",
			annotations:   nil,
			expectedMode:  "",
			expectedFound: false,
		},
		{
			name:          "empty annotations - returns empty and false",
			annotations:   map[string]string{},
			expectedMode:  "",
			expectedFound: false,
		},
		{
			name: "valid Serverless mode",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.Serverless),
			},
			expectedMode:  constants.Serverless,
			expectedFound: true,
		},
		{
			name: "valid RawDeployment mode",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.RawDeployment),
			},
			expectedMode:  constants.RawDeployment,
			expectedFound: true,
		},
		{
			name: "valid MultiNodeRayVLLM mode",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.MultiNodeRayVLLM),
			},
			expectedMode:  constants.MultiNodeRayVLLM,
			expectedFound: true,
		},
		{
			name: "valid MultiNode mode",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.MultiNode),
			},
			expectedMode:  constants.MultiNode,
			expectedFound: true,
		},
		{
			name: "valid VirtualDeployment mode",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.VirtualDeployment),
			},
			expectedMode:  constants.VirtualDeployment,
			expectedFound: true,
		},
		{
			name: "invalid deployment mode - returns empty and false",
			annotations: map[string]string{
				constants.DeploymentMode: "InvalidMode",
			},
			expectedMode:  "",
			expectedFound: false,
		},
		{
			name: "empty string deployment mode - returns empty and false",
			annotations: map[string]string{
				constants.DeploymentMode: "",
			},
			expectedMode:  "",
			expectedFound: false,
		},
		{
			name: "other annotations present but no deployment mode",
			annotations: map[string]string{
				"some.other/annotation": "value",
			},
			expectedMode:  "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, found := GetDeploymentModeFromAnnotations(tt.annotations)
			assert.Equal(t, tt.expectedMode, mode)
			assert.Equal(t, tt.expectedFound, found)
		})
	}
}

func TestGetDeploymentMode(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		deployConfig *controllerconfig.DeployConfig
		expectedMode constants.DeploymentModeType
	}{
		{
			name: "valid annotation overrides config",
			annotations: map[string]string{
				constants.DeploymentMode: string(constants.RawDeployment),
			},
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.Serverless),
			},
			expectedMode: constants.RawDeployment,
		},
		{
			name:        "no annotation uses config default",
			annotations: map[string]string{},
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.Serverless),
			},
			expectedMode: constants.Serverless,
		},
		{
			name:        "nil annotations uses config default",
			annotations: nil,
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.RawDeployment),
			},
			expectedMode: constants.RawDeployment,
		},
		{
			name: "invalid annotation falls back to config default",
			annotations: map[string]string{
				constants.DeploymentMode: "InvalidMode",
			},
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: string(constants.MultiNode),
			},
			expectedMode: constants.MultiNode,
		},
		{
			name:        "empty config default returns empty string",
			annotations: map[string]string{},
			deployConfig: &controllerconfig.DeployConfig{
				DefaultDeploymentMode: "",
			},
			expectedMode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := GetDeploymentMode(tt.annotations, tt.deployConfig)
			assert.Equal(t, tt.expectedMode, mode)
		})
	}
}
