package ingress

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

func TestIngressReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, netv1.AddToScheme(scheme))
	require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
	require.NoError(t, gatewayapiv1.Install(scheme))

	tests := []struct {
		name                 string
		isvc                 *v1beta1.InferenceService
		deploymentMode       constants.DeploymentModeType
		ingressConfig        *controllerconfig.IngressConfig
		isvcConfig           *controllerconfig.InferenceServicesConfig
		expectedStrategyName string
		expectError          bool
	}{
		{
			name:           "serverless deployment mode",
			isvc:           createTestInferenceServiceWithStatus("test-isvc", "default"),
			deploymentMode: constants.Serverless,
			ingressConfig: &controllerconfig.IngressConfig{
				IngressGateway:             "knative-serving/knative-ingress-gateway",
				LocalGateway:               "knative-serving/knative-local-gateway",
				IngressDomain:              "example.com",
				LocalGatewayServiceName:    "knative-local-gateway",
				KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
				DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:                  "https",
				DisableIstioVirtualHost:    false,
			},
			isvcConfig:           &controllerconfig.InferenceServicesConfig{},
			expectedStrategyName: "Serverless",
			expectError:          false,
		},
		{
			name:           "raw deployment mode with kubernetes ingress",
			isvc:           createTestInferenceServiceWithStatus("test-isvc", "default"),
			deploymentMode: constants.RawDeployment,
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       false,
				IngressDomain:          "example.com",
				IngressClassName:       stringPtr("nginx"),
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			isvcConfig:           &controllerconfig.InferenceServicesConfig{},
			expectedStrategyName: "KubernetesIngress",
			expectError:          false,
		},
		{
			name:           "raw deployment mode with gateway api",
			isvc:           createTestInferenceServiceWithStatus("test-isvc", "default"),
			deploymentMode: constants.RawDeployment,
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			isvcConfig:           &controllerconfig.InferenceServicesConfig{},
			expectedStrategyName: "GatewayAPI",
			expectError:          false,
		},
		{
			name:           "disabled ingress creation",
			isvc:           createTestInferenceServiceWithStatus("test-isvc", "default"),
			deploymentMode: constants.RawDeployment,
			ingressConfig: &controllerconfig.IngressConfig{
				IngressDomain:          "example.com",
				IngressClassName:       stringPtr("nginx"),
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: true,
			},
			isvcConfig:           &controllerconfig.InferenceServicesConfig{},
			expectedStrategyName: "", // No strategy should be used
			expectError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clients
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.isvc).
				Build()
			fakeClientset := fake.NewSimpleClientset()

			// Create reconciler
			reconciler := NewIngressReconciler(
				fakeClient,
				fakeClientset,
				scheme,
				tt.ingressConfig,
				tt.isvcConfig,
			).(*IngressReconciler)

			// Add specific handling for disabled ingress creation test
			if tt.name == "disabled ingress creation" {
				// Update the InferenceService in fake client before reconcile
				err := fakeClient.Update(context.Background(), tt.isvc)
				require.NoError(t, err, "Failed to update InferenceService in fake client")
			}

			// Set deployment mode annotation and update in fake client
			if tt.isvc.Annotations == nil {
				tt.isvc.Annotations = make(map[string]string)
			}
			tt.isvc.Annotations[constants.DeploymentMode] = string(tt.deploymentMode)

			// Update the object in the fake client with the annotation
			err := fakeClient.Update(context.Background(), tt.isvc)
			assert.NoError(t, err, "Failed to update InferenceService with deployment mode annotation")

			// Execute reconcile
			err = reconciler.Reconcile(context.Background(), tt.isvc)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Special verification for disabled ingress creation
			if tt.ingressConfig.DisableIngressCreation {
				// First check the condition directly on the isvc object (in-memory)
				condition := tt.isvc.Status.GetCondition(v1beta1.IngressReady)
				if condition == nil {
					// Get the updated object from the fake client
					updatedIsvc := &v1beta1.InferenceService{}
					err = fakeClient.Get(context.Background(), client.ObjectKey{
						Name:      tt.isvc.Name,
						Namespace: tt.isvc.Namespace,
					}, updatedIsvc)
					assert.NoError(t, err, "Failed to get updated InferenceService from fake client")

					condition = updatedIsvc.Status.GetCondition(v1beta1.IngressReady)
				}

				// Verify IngressReady condition is set to True
				assert.NotNil(t, condition, "IngressReady condition should be set when ingress is disabled")
				if condition != nil {
					assert.Equal(t, corev1.ConditionTrue, condition.Status, "IngressReady should be True when ingress creation is disabled")
					assert.Equal(t, "IngressDisabled", condition.Reason, "IngressReady condition reason should be IngressDisabled")
					assert.Contains(t, condition.Message, "Ingress creation is disabled", "IngressReady condition message should explain ingress is disabled")
				}
			}
		})
	}
}

func TestIngressReconciler_GetDeploymentMode(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name         string
		engine       *v1beta1.EngineSpec
		decoder      *v1beta1.DecoderSpec
		router       *v1beta1.RouterSpec
		expectedMode constants.DeploymentModeType
	}{
		{
			name: "serverless deployment mode - engine with min replicas 0",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			decoder:      nil,
			router:       nil,
			expectedMode: constants.Serverless,
		},
		{
			name: "raw deployment mode - engine with min replicas > 0",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			decoder:      nil,
			router:       nil,
			expectedMode: constants.RawDeployment,
		},
		{
			name: "multinode deployment mode - engine with leader and worker",
			engine: &v1beta1.EngineSpec{
				Leader: &v1beta1.LeaderSpec{},
				Worker: &v1beta1.WorkerSpec{},
			},
			decoder:      nil,
			router:       nil,
			expectedMode: constants.MultiNode,
		},
		{
			name: "router takes precedence - serverless router with raw engine",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(1),
				},
			},
			decoder: nil,
			router: &v1beta1.RouterSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0),
				},
			},
			expectedMode: constants.Serverless,
		},
		{
			name:         "no components - defaults to raw deployment",
			engine:       nil,
			decoder:      nil,
			router:       nil,
			expectedMode: constants.RawDeployment,
		},
		{
			name: "decoder constraint - engine becomes raw when decoder present",
			engine: &v1beta1.EngineSpec{
				ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: intPtr(0), // Would be serverless, but decoder constraint applies
				},
			},
			decoder:      &v1beta1.DecoderSpec{}, // Decoder present
			router:       nil,
			expectedMode: constants.RawDeployment, // Engine forced to raw deployment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			fakeClientset := fake.NewSimpleClientset()

			reconciler := &IngressReconciler{
				client:        fakeClient,
				clientset:     fakeClientset,
				scheme:        scheme,
				ingressConfig: &controllerconfig.IngressConfig{},
				isvcConfig:    &controllerconfig.InferenceServicesConfig{},
			}

			isvc := &v1beta1.InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: v1beta1.InferenceServiceSpec{
					Engine:  tt.engine,
					Decoder: tt.decoder,
					Router:  tt.router,
				},
			}

			mode := reconciler.getDeploymentMode(isvc, tt.engine, tt.decoder, tt.router)
			assert.Equal(t, tt.expectedMode, mode)
		})
	}
}

func TestIngressReconciler_GetStrategy(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, netv1.AddToScheme(scheme))
	require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
	require.NoError(t, gatewayapiv1.Install(scheme))

	tests := []struct {
		name           string
		deploymentMode constants.DeploymentModeType
		opts           interfaces.ReconcilerOptions
		expectError    bool
		expectedName   string
	}{
		{
			name:           "serverless strategy",
			deploymentMode: constants.Serverless,
			opts: interfaces.ReconcilerOptions{
				Client: fakeclient.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme: scheme,
				IngressConfig: &controllerconfig.IngressConfig{
					IngressGateway:             "knative-serving/knative-ingress-gateway",
					LocalGateway:               "knative-serving/knative-local-gateway",
					IngressDomain:              "example.com",
					KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
					DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectError:  false,
			expectedName: "Serverless",
		},
		{
			name:           "raw deployment strategy",
			deploymentMode: constants.RawDeployment,
			opts: interfaces.ReconcilerOptions{
				Client: fakeclient.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme: scheme,
				IngressConfig: &controllerconfig.IngressConfig{
					EnableGatewayAPI: false,
					IngressDomain:    "example.com",
					DomainTemplate:   "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectError:  false,
			expectedName: "KubernetesIngress",
		},
		{
			name:           "gateway api strategy",
			deploymentMode: constants.RawDeployment,
			opts: interfaces.ReconcilerOptions{
				Client: fakeclient.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme: scheme,
				IngressConfig: &controllerconfig.IngressConfig{
					EnableGatewayAPI:  true,
					IngressDomain:     "example.com",
					OmeIngressGateway: "istio-system/gateway",
					DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				},
				IsvcConfig: &controllerconfig.InferenceServicesConfig{},
			},
			expectError:  false,
			expectedName: "GatewayAPI",
		},
		{
			name:           "unsupported deployment mode",
			deploymentMode: "unsupported",
			opts: interfaces.ReconcilerOptions{
				Client:        fakeclient.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme:        scheme,
				IngressConfig: &controllerconfig.IngressConfig{},
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			},
			expectError:  true,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()
			reconciler := NewIngressReconciler(
				tt.opts.Client,
				fakeClientset,
				tt.opts.Scheme,
				tt.opts.IngressConfig,
				tt.opts.IsvcConfig,
			)

			strategy, err := reconciler.(*IngressReconciler).getStrategy(tt.deploymentMode, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, strategy)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, strategy)
				assert.Equal(t, tt.expectedName, strategy.GetName())
			}
		})
	}
}

func TestIngressReconciler_NilFactory(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	reconciler := &IngressReconciler{
		client:        fakeClient,
		scheme:        scheme,
		ingressConfig: &controllerconfig.IngressConfig{},
		isvcConfig:    &controllerconfig.InferenceServicesConfig{},
		factory:       nil, // nil factory
	}

	opts := interfaces.ReconcilerOptions{
		Client:        fakeClient,
		Scheme:        scheme,
		IngressConfig: &controllerconfig.IngressConfig{},
		IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
	}

	_, err := reconciler.getStrategy(constants.Serverless, opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "strategy factory is not initialized")
}

// Mock strategy for testing
type mockStrategy struct {
	name          string
	reconcileFunc func(ctx context.Context, isvc *v1beta1.InferenceService) error
}

func (m *mockStrategy) GetName() string {
	return m.name
}

func (m *mockStrategy) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) error {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(ctx, isvc)
	}
	return nil
}

// Mock factory for testing
type mockFactory struct {
	strategy interfaces.IngressStrategy
	err      error
}

func (m *mockFactory) CreateStrategyWithOptions(deploymentMode string, opts interfaces.ReconcilerOptions) (interfaces.IngressStrategy, error) {
	return m.strategy, m.err
}

func TestIngressReconciler_ReconcileWithMockStrategy(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	isvc := createTestInferenceServiceWithStatus("test-isvc", "default")

	tests := []struct {
		name        string
		strategy    interfaces.IngressStrategy
		factoryErr  error
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful reconcile",
			strategy: &mockStrategy{
				name: "TestStrategy",
				reconcileFunc: func(ctx context.Context, isvc *v1beta1.InferenceService) error {
					return nil
				},
			},
			factoryErr:  nil,
			expectError: false,
		},
		{
			name:        "factory returns error",
			strategy:    nil,
			factoryErr:  assert.AnError,
			expectError: true,
			errorMsg:    "failed to get ingress strategy",
		},
		{
			name: "strategy reconcile returns error",
			strategy: &mockStrategy{
				name: "TestStrategy",
				reconcileFunc: func(ctx context.Context, isvc *v1beta1.InferenceService) error {
					return assert.AnError
				},
			},
			factoryErr:  nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(isvc).
				Build()
			fakeClientset := fake.NewSimpleClientset()

			reconciler := &IngressReconciler{
				client:        fakeClient,
				clientset:     fakeClientset,
				scheme:        scheme,
				ingressConfig: &controllerconfig.IngressConfig{},
				isvcConfig:    &controllerconfig.InferenceServicesConfig{},
				factory:       &mockFactory{strategy: tt.strategy, err: tt.factoryErr},
			}

			err := reconciler.Reconcile(context.Background(), isvc)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// Helper function to create test InferenceService with proper status
func createTestInferenceServiceWithStatus(name, namespace string) *v1beta1.InferenceService {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					Runtime: stringPtr("sklearn"),
				},
			},
		},
		Status: v1beta1.InferenceServiceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{},
			},
			Components: map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
				v1beta1.EngineComponent: {
					URL: &apis.URL{
						Scheme: "http",
						Host:   name + "-engine-default." + namespace + ".example.com",
					},
				},
			},
		},
	}

	// Set predictor ready condition
	isvc.Status.SetCondition(v1beta1.PredictorReady, &apis.Condition{
		Type:   v1beta1.PredictorReady,
		Status: corev1.ConditionTrue,
	})

	// Set engine ready condition (required by HTTPRoute builder)
	isvc.Status.SetCondition(v1beta1.EngineReady, &apis.Condition{
		Type:   v1beta1.EngineReady,
		Status: corev1.ConditionTrue,
	})

	return isvc
}
