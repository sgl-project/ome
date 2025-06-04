package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

func TestDefaultDomainService_GenerateDomainName(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		obj            interface{}
		ingressConfig  *controllerconfig.IngressConfig
		expectedDomain string
		expectedError  bool
		errorContains  string
	}{
		{
			name:        "simple domain template with InferenceService",
			serviceName: "test-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "test-service.default.example.com",
			expectedError:  false,
		},
		{
			name:        "simple domain template with ObjectMeta",
			serviceName: "test-service",
			obj: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "production",
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "test-service.production.example.com",
			expectedError:  false,
		},
		{
			name:        "custom domain template",
			serviceName: "ml-model",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ml-model",
					Namespace: "ml-team",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "ml.company.com",
				DomainTemplate: "{{.Name}}-service.{{.IngressDomain}}",
			},
			expectedDomain: "ml-model-service.ml.company.com",
			expectedError:  false,
		},
		{
			name:        "domain template with annotations",
			serviceName: "annotated-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "annotated-service",
					Namespace: "default",
					Annotations: map[string]string{
						"environment": "production",
						"team":        "ml-ops",
					},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Annotations.environment}}-{{.Name}}.{{.IngressDomain}}",
			},
			expectedDomain: "production-annotated-service.example.com",
			expectedError:  false,
		},
		{
			name:        "domain template with labels",
			serviceName: "labeled-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-service",
					Namespace: "default",
					Labels: map[string]string{
						"version": "v2",
						"type":    "classifier",
					},
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Labels.type}}-{{.Name}}-{{.Labels.version}}.{{.IngressDomain}}",
			},
			expectedDomain: "classifier-labeled-service-v2.example.com",
			expectedError:  false,
		},
		{
			name:        "invalid template syntax",
			serviceName: "test-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "",
			expectedError:  true,
			errorContains:  "template",
		},
		{
			name:        "invalid domain name",
			serviceName: "test service", // space in name
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test service",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "",
			expectedError:  true,
			errorContains:  "invalid domain name",
		},
		{
			name:        "unsupported object type",
			serviceName: "test-service",
			obj:         "unsupported-string",
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "",
			expectedError:  true,
			errorContains:  "unsupported object type",
		},
		{
			name:        "namespace only template",
			serviceName: "service-in-ns",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-in-ns",
					Namespace: "special-ns",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "special-ns.example.com",
			expectedError:  false,
		},
		{
			name:        "ingress domain only",
			serviceName: "simple-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple-service",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "simple.example.com",
				DomainTemplate: "{{.IngressDomain}}",
			},
			expectedDomain: "simple.example.com",
			expectedError:  false,
		},
		{
			name:        "empty service name",
			serviceName: "",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedError: true, // Empty name creates invalid domain ".default.example.com"
			errorContains: "invalid domain name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDomainService()

			domain, err := service.GenerateDomainName(tt.serviceName, tt.obj, tt.ingressConfig)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDomain, domain)
			}
		})
	}
}

func TestDefaultDomainService_GenerateInternalDomainName(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		obj            interface{}
		ingressConfig  *controllerconfig.IngressConfig
		expectedDomain string
		expectedError  bool
	}{
		{
			name:        "internal domain with InferenceService",
			serviceName: "test-service",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "test-service.default.cluster.local", // Uses cluster domain
			expectedError:  false,
		},
		{
			name:        "internal domain with ObjectMeta",
			serviceName: "internal-service",
			obj: metav1.ObjectMeta{
				Name:      "internal-service",
				Namespace: "system",
			},
			ingressConfig: &controllerconfig.IngressConfig{
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedDomain: "internal-service.system.cluster.local",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDomainService()

			domain, err := service.GenerateInternalDomainName(tt.serviceName, tt.obj, tt.ingressConfig)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDomain, domain)
			}
		})
	}
}

func TestDefaultDomainService_GetAdditionalHosts(t *testing.T) {
	tests := []struct {
		name               string
		domainList         *[]string
		serviceHost        string
		config             *controllerconfig.IngressConfig
		expectedHosts      []string
		expectedHostsCount int
	}{
		{
			name:        "no additional domains",
			domainList:  &[]string{"example.com"},
			serviceHost: "test-service.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: nil,
			},
			expectedHosts:      []string{},
			expectedHostsCount: 0,
		},
		{
			name:        "single additional domain",
			domainList:  &[]string{"example.com"},
			serviceHost: "test-service.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com"},
			},
			expectedHosts:      []string{"test-service.default.dev.example.com"},
			expectedHostsCount: 1,
		},
		{
			name:        "multiple additional domains",
			domainList:  &[]string{"example.com"},
			serviceHost: "ml-model.production.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com", "staging.example.com", "test.example.com"},
			},
			expectedHosts:      []string{"ml-model.production.dev.example.com", "ml-model.production.staging.example.com", "ml-model.production.test.example.com"},
			expectedHostsCount: 3,
		},
		{
			name:        "duplicate additional domains",
			domainList:  &[]string{"example.com"},
			serviceHost: "test.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com", "dev.example.com", "staging.example.com"},
			},
			expectedHosts:      []string{"test.default.dev.example.com", "test.default.staging.example.com"},
			expectedHostsCount: 2,
		},
		{
			name:        "service host doesn't match domain list",
			domainList:  &[]string{"example.com"},
			serviceHost: "test.default.other.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com"},
			},
			expectedHosts:      []string{},
			expectedHostsCount: 0,
		},
		{
			name:        "empty domain list",
			domainList:  &[]string{},
			serviceHost: "test.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com"},
			},
			expectedHosts:      []string{},
			expectedHostsCount: 0,
		},
		{
			name:        "nil domain list",
			domainList:  nil,
			serviceHost: "test.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"dev.example.com"},
			},
			expectedHosts:      []string{},
			expectedHostsCount: 0,
		},
		{
			name:        "invalid additional domain",
			domainList:  &[]string{"example.com"},
			serviceHost: "test.default.example.com",
			config: &controllerconfig.IngressConfig{
				AdditionalIngressDomains: &[]string{"invalid domain name"},
			},
			expectedHosts:      []string{},
			expectedHostsCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDomainService()

			hosts := service.GetAdditionalHosts(tt.domainList, tt.serviceHost, tt.config)

			assert.NotNil(t, hosts)
			assert.Len(t, *hosts, tt.expectedHostsCount)

			if tt.expectedHostsCount > 0 {
				for _, expectedHost := range tt.expectedHosts {
					assert.Contains(t, *hosts, expectedHost)
				}
			}
		})
	}
}

func TestDefaultDomainService_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		obj           interface{}
		ingressConfig *controllerconfig.IngressConfig
		expectedError bool
		errorContains string
	}{
		{
			name:        "empty service name",
			serviceName: "",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedError: true, // Empty name creates invalid domain ".default.example.com"
			errorContains: "invalid domain name",
		},
		{
			name:        "empty namespace",
			serviceName: "test",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedError: true, // Empty namespace creates invalid domain "test..example.com"
			errorContains: "invalid domain name",
		},
		{
			name:        "template with undefined field",
			serviceName: "test",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.UndefinedField}}.{{.IngressDomain}}",
			},
			expectedError: true,
			errorContains: "can't evaluate field UndefinedField",
		},
		{
			name:        "very long domain name",
			serviceName: "very-long-service-name-that-exceeds-normal-limits-and-keeps-going",
			obj: &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "very-long-service-name-that-exceeds-normal-limits-and-keeps-going",
					Namespace: "very-long-namespace-name-that-also-exceeds-limits",
				},
			},
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "very-long-domain-name-example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedError: true, // Should exceed domain name limits
			errorContains: "invalid domain name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDomainService()

			_, err := service.GenerateDomainName(tt.serviceName, tt.obj, tt.ingressConfig)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultDomainService_Interface(t *testing.T) {
	// Test that our implementation satisfies the interface
	var _ interfaces.DomainService = &DefaultDomainService{}
	var _ interfaces.DomainService = NewDomainService()
}

// Benchmark tests
func BenchmarkDefaultDomainService_GenerateDomainName(b *testing.B) {
	service := NewDomainService()
	obj := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "production",
		},
	}
	config := &controllerconfig.IngressConfig{
		IngressDomain:  "example.com",
		DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateDomainName("test-service", obj, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefaultDomainService_GetAdditionalHosts(b *testing.B) {
	service := NewDomainService()
	domainList := &[]string{"example.com"}
	serviceHost := "test-service.default.example.com"
	config := &controllerconfig.IngressConfig{
		AdditionalIngressDomains: &[]string{"dev.example.com", "staging.example.com", "test.example.com"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetAdditionalHosts(domainList, serviceHost, config)
	}
}
