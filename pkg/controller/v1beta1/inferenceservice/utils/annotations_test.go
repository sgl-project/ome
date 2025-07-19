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
