package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDefaultStrategyFactory_CreateStrategyWithOptions(t *testing.T) {
	tests := []struct {
		name             string
		deploymentMode   string
		opts             interfaces.ReconcilerOptions
		expectedStrategy string
		expectedError    bool
		errorContains    string
	}{
		{
			name:           "serverless deployment mode",
			deploymentMode: string(constants.Serverless),
			opts: interfaces.ReconcilerOptions{
				Client: createFakeClient(t),
				Scheme: createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{
					IngressGateway:             "knative-serving/knative-ingress-gateway",
					LocalGateway:               "knative-serving/knative-local-gateway",
					IngressDomain:              "example.com",
					LocalGatewayServiceName:    "knative-local-gateway",
					KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
					DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
					UrlScheme:                  "https",
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "Serverless",
			expectedError:    false,
		},
		{
			name:           "raw deployment mode with kubernetes ingress",
			deploymentMode: string(constants.RawDeployment),
			opts: interfaces.ReconcilerOptions{
				Client: createFakeClient(t),
				Scheme: createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{
					EnableGatewayAPI:       false,
					IngressDomain:          "example.com",
					IngressClassName:       stringPtr("nginx"),
					DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
					UrlScheme:              "https",
					DisableIngressCreation: false,
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "KubernetesIngress",
			expectedError:    false,
		},
		{
			name:           "raw deployment mode with gateway api",
			deploymentMode: string(constants.RawDeployment),
			opts: interfaces.ReconcilerOptions{
				Client: createFakeClient(t),
				Scheme: createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{
					EnableGatewayAPI:       true,
					IngressDomain:          "example.com",
					OmeIngressGateway:      "istio-system/gateway",
					DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
					UrlScheme:              "https",
					DisableIngressCreation: false,
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "GatewayAPI",
			expectedError:    false,
		},
		{
			name:           "unsupported deployment mode",
			deploymentMode: "unsupported-mode",
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "",
			expectedError:    true,
			errorContains:    "unsupported deployment mode",
		},
		{
			name:           "empty deployment mode",
			deploymentMode: "",
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "",
			expectedError:    true,
			errorContains:    "unsupported deployment mode",
		},
		{
			name:           "multinode deployment mode",
			deploymentMode: string(constants.MultiNode),
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "",
			expectedError:    true,
			errorContains:    "unsupported deployment mode",
		},
		{
			name:           "virtual deployment mode",
			deploymentMode: string(constants.VirtualDeployment),
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectedStrategy: "",
			expectedError:    true,
			errorContains:    "unsupported deployment mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()
			factory := NewStrategyFactory(fakeClientset)

			strategy, err := factory.CreateStrategyWithOptions(tt.deploymentMode, tt.opts)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, strategy)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, strategy)
				assert.Equal(t, tt.expectedStrategy, strategy.GetName())
			}
		})
	}
}

func TestDefaultStrategyFactory_ServicesCreation(t *testing.T) {
	// Test that the factory creates the expected services
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	// Verify factory is properly initialized
	defaultFactory, ok := factory.(*DefaultStrategyFactory)
	require.True(t, ok, "Factory should be of type *DefaultStrategyFactory")

	assert.NotNil(t, defaultFactory.clientset)
	assert.NotNil(t, defaultFactory.domainService)
	assert.NotNil(t, defaultFactory.pathService)
}

func TestDefaultStrategyFactory_StrategyConsistency(t *testing.T) {
	// Test that the same strategy type is returned for the same deployment mode
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	opts := interfaces.ReconcilerOptions{
		Client: createFakeClient(t),
		Scheme: createScheme(t),
		IngressConfig: &controllerconfig.IngressConfig{
			IngressDomain:  "example.com",
			DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	// Create multiple strategies of the same type
	strategy1, err1 := factory.CreateStrategyWithOptions(string(constants.Serverless), opts)
	strategy2, err2 := factory.CreateStrategyWithOptions(string(constants.Serverless), opts)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, strategy1.GetName(), strategy2.GetName())
}

func TestDefaultStrategyFactory_DifferentConfigurations(t *testing.T) {
	// Test different configurations for raw deployment mode
	tests := []struct {
		name             string
		ingressConfig    *controllerconfig.IngressConfig
		expectedStrategy string
	}{
		{
			name: "gateway api enabled",
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:  true,
				IngressDomain:     "example.com",
				OmeIngressGateway: "istio-system/gateway",
				DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedStrategy: "GatewayAPI",
		},
		{
			name: "gateway api disabled",
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI: false,
				IngressDomain:    "example.com",
				IngressClassName: stringPtr("nginx"),
				DomainTemplate:   "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedStrategy: "KubernetesIngress",
		},
		{
			name: "gateway api not specified (defaults to false)",
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedStrategy: "KubernetesIngress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()
			factory := NewStrategyFactory(fakeClientset)

			opts := interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: tt.ingressConfig,
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			}

			strategy, err := factory.CreateStrategyWithOptions(string(constants.RawDeployment), opts)

			assert.NoError(t, err)
			assert.NotNil(t, strategy)
			assert.Equal(t, tt.expectedStrategy, strategy.GetName())
		})
	}
}

func TestDefaultStrategyFactory_NilOptions(t *testing.T) {
	// Test behavior with nil or invalid options
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	tests := []struct {
		name        string
		opts        interfaces.ReconcilerOptions
		expectPanic bool
	}{
		{
			name: "nil client",
			opts: interfaces.ReconcilerOptions{
				Client:        nil,
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectPanic: false, // Should handle gracefully
		},
		{
			name: "nil scheme",
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        nil,
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectPanic: false, // Should handle gracefully
		},
		{
			name: "nil ingress config",
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: nil,
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectPanic: false, // Should handle gracefully
		},
		{
			name: "nil isvc config",
			opts: interfaces.ReconcilerOptions{
				Client:        createFakeClient(t),
				Scheme:        createScheme(t),
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    nil,
			},
			expectPanic: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					_, _ = factory.CreateStrategyWithOptions(string(constants.Serverless), tt.opts)
				})
			} else {
				// Should not panic, though it might return an error
				strategy, err := factory.CreateStrategyWithOptions(string(constants.Serverless), tt.opts)
				// We don't assert error/success here as different nil values might be handled differently
				_ = strategy
				_ = err
			}
		})
	}
}

func TestDefaultStrategyFactory_Interface(t *testing.T) {
	// Test that our implementation satisfies the interface
	var _ interfaces.StrategyFactory = &DefaultStrategyFactory{}

	fakeClientset := fake.NewSimpleClientset()
	var _ interfaces.StrategyFactory = NewStrategyFactory(fakeClientset)
}

// Helper functions
func createFakeClient(t *testing.T) client.Client {
	scheme := createScheme(t)
	return fakeclient.NewClientBuilder().WithScheme(scheme).Build()
}

func createScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	return scheme
}

func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkDefaultStrategyFactory_CreateServerlessStrategy(b *testing.B) {
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	opts := interfaces.ReconcilerOptions{
		Client: createFakeClient(&testing.T{}),
		Scheme: createScheme(&testing.T{}),
		IngressConfig: &controllerconfig.IngressConfig{
			IngressGateway:             "knative-serving/knative-ingress-gateway",
			LocalGateway:               "knative-serving/knative-local-gateway",
			IngressDomain:              "example.com",
			KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
			DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy, err := factory.CreateStrategyWithOptions(string(constants.Serverless), opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = strategy
	}
}

func BenchmarkDefaultStrategyFactory_CreateKubernetesIngressStrategy(b *testing.B) {
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	opts := interfaces.ReconcilerOptions{
		Client: createFakeClient(&testing.T{}),
		Scheme: createScheme(&testing.T{}),
		IngressConfig: &controllerconfig.IngressConfig{
			EnableGatewayAPI: false,
			IngressDomain:    "example.com",
			IngressClassName: stringPtr("nginx"),
			DomainTemplate:   "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy, err := factory.CreateStrategyWithOptions(string(constants.RawDeployment), opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = strategy
	}
}

func BenchmarkDefaultStrategyFactory_CreateGatewayAPIStrategy(b *testing.B) {
	fakeClientset := fake.NewSimpleClientset()
	factory := NewStrategyFactory(fakeClientset)

	opts := interfaces.ReconcilerOptions{
		Client: createFakeClient(&testing.T{}),
		Scheme: createScheme(&testing.T{}),
		IngressConfig: &controllerconfig.IngressConfig{
			EnableGatewayAPI:  true,
			IngressDomain:     "example.com",
			OmeIngressGateway: "istio-system/gateway",
			DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy, err := factory.CreateStrategyWithOptions(string(constants.RawDeployment), opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = strategy
	}
}
