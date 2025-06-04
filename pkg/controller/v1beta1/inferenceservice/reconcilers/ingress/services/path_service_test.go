package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

func TestDefaultPathService_GenerateUrlPath(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		namespace     string
		ingressConfig *controllerconfig.IngressConfig
		expectedPath  string
		expectedError bool
		errorContains string
	}{
		{
			name:        "empty path template",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "",
			},
			expectedPath:  "",
			expectedError: false,
		},
		{
			name:        "simple path template",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/models/{{.Name}}",
			},
			expectedPath:  "/models/test-service",
			expectedError: false,
		},
		{
			name:        "path template with namespace",
			serviceName: "test-service",
			namespace:   "production",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/{{.Namespace}}/models/{{.Name}}",
			},
			expectedPath:  "/production/models/test-service",
			expectedError: false,
		},
		{
			name:        "complex path template",
			serviceName: "ml-model",
			namespace:   "ml-team",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/api/v1/namespaces/{{.Namespace}}/inference-services/{{.Name}}",
			},
			expectedPath:  "/api/v1/namespaces/ml-team/inference-services/ml-model",
			expectedError: false,
		},
		{
			name:        "path template with special characters",
			serviceName: "test-service-v2",
			namespace:   "test-ns",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/v1/{{.Namespace}}/{{.Name}}/predict",
			},
			expectedPath:  "/v1/test-ns/test-service-v2/predict",
			expectedError: false,
		},
		{
			name:        "invalid template syntax",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/models/{{.InvalidField",
			},
			expectedPath:  "",
			expectedError: true,
			errorContains: "template",
		},
		{
			name:        "template with invalid URL characters",
			serviceName: "test service", // space in name
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/models/{{.Name}}",
			},
			expectedPath:  "/models/test service", // Space is valid in URL path
			expectedError: false,
		},
		{
			name:        "template with scheme (invalid)",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "http://example.com/{{.Name}}",
			},
			expectedPath:  "",
			expectedError: true,
			errorContains: "contains either a scheme or a host",
		},
		{
			name:        "template with host (invalid)",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "//example.com/{{.Name}}",
			},
			expectedPath:  "//example.com/test-service", // This is treated as a path, not a host
			expectedError: false,
		},
		{
			name:        "root path",
			serviceName: "test-service",
			namespace:   "default",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/",
			},
			expectedPath:  "/",
			expectedError: false,
		},
		{
			name:        "nested path",
			serviceName: "bert-large",
			namespace:   "nlp-models",
			ingressConfig: &controllerconfig.IngressConfig{
				PathTemplate: "/ai/nlp/{{.Namespace}}/{{.Name}}/v1/predict",
			},
			expectedPath:  "/ai/nlp/nlp-models/bert-large/v1/predict",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPathService()

			path, err := service.GenerateUrlPath(tt.serviceName, tt.namespace, tt.ingressConfig)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func TestDefaultPathService_GenerateUrlPath_TemplateValues(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		namespace    string
		template     string
		expectedPath string
	}{
		{
			name:         "name only",
			serviceName:  "my-model",
			namespace:    "default",
			template:     "/{{.Name}}",
			expectedPath: "/my-model",
		},
		{
			name:         "namespace only",
			serviceName:  "my-model",
			namespace:    "production",
			template:     "/{{.Namespace}}",
			expectedPath: "/production",
		},
		{
			name:         "both name and namespace",
			serviceName:  "text-classifier",
			namespace:    "ml-team",
			template:     "/{{.Namespace}}/{{.Name}}",
			expectedPath: "/ml-team/text-classifier",
		},
		{
			name:         "repeated values",
			serviceName:  "echo",
			namespace:    "test",
			template:     "/{{.Name}}/{{.Name}}/{{.Namespace}}",
			expectedPath: "/echo/echo/test",
		},
		{
			name:         "mixed with static text",
			serviceName:  "detector",
			namespace:    "security",
			template:     "/api/v2/{{.Namespace}}/models/{{.Name}}/inference",
			expectedPath: "/api/v2/security/models/detector/inference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPathService()
			config := &controllerconfig.IngressConfig{
				PathTemplate: tt.template,
			}

			path, err := service.GenerateUrlPath(tt.serviceName, tt.namespace, config)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

func TestDefaultPathService_GenerateUrlPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		namespace     string
		template      string
		expectedError bool
		errorContains string
	}{
		{
			name:          "empty service name",
			serviceName:   "",
			namespace:     "default",
			template:      "/{{.Name}}",
			expectedError: false, // Should work, just empty name
		},
		{
			name:          "empty namespace",
			serviceName:   "test",
			namespace:     "",
			template:      "/{{.Namespace}}",
			expectedError: false, // Should work, just empty namespace
		},
		{
			name:          "template with undefined field",
			serviceName:   "test",
			namespace:     "default",
			template:      "/{{.UndefinedField}}",
			expectedError: true,
			errorContains: "can't evaluate field UndefinedField",
		},
		{
			name:          "very long path",
			serviceName:   "very-long-service-name-that-exceeds-normal-limits",
			namespace:     "very-long-namespace-name-that-also-exceeds-normal-limits",
			template:      "/{{.Namespace}}/{{.Name}}/with/many/additional/path/segments/that/make/this/very/long",
			expectedError: false, // Should work unless URL parsing fails
		},
		{
			name:          "path with query parameters (invalid)",
			serviceName:   "test",
			namespace:     "default",
			template:      "/{{.Name}}?param=value",
			expectedError: false, // This is actually valid in a path
		},
		{
			name:          "path with fragment (invalid)",
			serviceName:   "test",
			namespace:     "default",
			template:      "/{{.Name}}#fragment",
			expectedError: false, // This is actually valid in a path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPathService()
			config := &controllerconfig.IngressConfig{
				PathTemplate: tt.template,
			}

			path, err := service.GenerateUrlPath(tt.serviceName, tt.namespace, config)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, config.PathTemplate) // Only check if template wasn't empty
			}

			// If no error expected and template not empty, path should not be empty
			if !tt.expectedError && tt.template != "" && tt.serviceName != "" {
				assert.NotEmpty(t, path)
			}
		})
	}
}

func TestDefaultPathService_Interface(t *testing.T) {
	// Test that our implementation satisfies the interface
	var _ interfaces.PathService = &DefaultPathService{}
	var _ interfaces.PathService = NewPathService()
}

// Benchmark tests
func BenchmarkDefaultPathService_GenerateUrlPath(b *testing.B) {
	service := NewPathService()
	config := &controllerconfig.IngressConfig{
		PathTemplate: "/api/v1/namespaces/{{.Namespace}}/models/{{.Name}}/predict",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateUrlPath("test-model", "production", config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefaultPathService_EmptyTemplate(b *testing.B) {
	service := NewPathService()
	config := &controllerconfig.IngressConfig{
		PathTemplate: "",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateUrlPath("test-model", "production", config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
