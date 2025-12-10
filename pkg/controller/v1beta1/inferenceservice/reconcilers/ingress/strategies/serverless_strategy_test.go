package strategies

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
)

func TestServerlessStrategy_GetName(t *testing.T) {
	strategy := createServerlessStrategy(t)
	assert.Equal(t, "Serverless", strategy.GetName())
}

func TestServerlessStrategy_Reconcile(t *testing.T) {
	tests := []struct {
		name                   string
		isvc                   *v1beta1.InferenceService
		ingressConfig          *controllerconfig.IngressConfig
		existingVirtualService *istioclientv1beta1.VirtualService
		existingService        *corev1.Service
		expectedError          bool
		expectedURLScheme      string
		expectedIngressReady   corev1.ConditionStatus
	}{
		{
			name: "successful reconcile with predictor only",
			isvc: createTestInferenceService("test-isvc", "default"),
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
			expectedError:        false,
			expectedURLScheme:    "https",
			expectedIngressReady: corev1.ConditionTrue,
		},
		{
			name: "successful reconcile with router",
			isvc: createTestInferenceServiceWithRouter("test-isvc", "default"),
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
			expectedError:        false,
			expectedURLScheme:    "https",
			expectedIngressReady: corev1.ConditionTrue,
		},
		{
			name: "istio virtual host disabled",
			isvc: createTestInferenceService("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				IngressGateway:             "knative-serving/knative-ingress-gateway",
				LocalGateway:               "knative-serving/knative-local-gateway",
				IngressDomain:              "example.com",
				LocalGatewayServiceName:    "knative-local-gateway",
				KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
				DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:                  "https",
				DisableIstioVirtualHost:    true,
			},
			expectedError:        false,
			expectedURLScheme:    "https",
			expectedIngressReady: corev1.ConditionTrue,
		},
		{
			name: "update existing virtual service",
			isvc: createTestInferenceService("test-isvc", "default"),
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
			existingVirtualService: &istioclientv1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			expectedError:        false,
			expectedURLScheme:    "https",
			expectedIngressReady: corev1.ConditionTrue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			// Set component statuses to ready
			setComponentStatusReady(tt.isvc)

			objs := []client.Object{tt.isvc}
			if tt.existingVirtualService != nil {
				objs = append(objs, tt.existingVirtualService)
			}
			if tt.existingService != nil {
				objs = append(objs, tt.existingService)
			}

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()
			fakeClientset := fake.NewSimpleClientset()

			// Use the proper constructor to create the strategy with builder
			opts := interfaces.ReconcilerOptions{
				Client:        fakeClient,
				Scheme:        scheme,
				IngressConfig: tt.ingressConfig,
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			}
			strategy := NewServerlessStrategy(opts, fakeClientset, services.NewDomainService(), services.NewPathService())

			err := strategy.Reconcile(context.Background(), tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIngressReady, tt.isvc.Status.GetCondition(v1beta1.IngressReady).Status)
				if tt.isvc.Status.URL != nil {
					assert.Equal(t, tt.expectedURLScheme, tt.isvc.Status.URL.Scheme)
				}
			}
		})
	}
}

func TestServerlessStrategy_GetServiceHost(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		expectedHost string
	}{
		{
			name:         "predictor only",
			isvc:         createTestInferenceServiceWithEngineStatus("test-isvc", "default"),
			expectedHost: "test-isvc.default.example.com",
		},
		{
			name:         "with router",
			isvc:         createTestInferenceServiceWithRouterStatus("test-isvc", "default"),
			expectedHost: "test-isvc.default.example.com",
		},
		{
			name:         "no components",
			isvc:         createTestInferenceService("test-isvc", "default"),
			expectedHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createServerlessStrategy(t)
			host := strategy.(*ServerlessStrategy).getServiceHost(tt.isvc)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

func TestServerlessStrategy_GetServiceUrl(t *testing.T) {
	tests := []struct {
		name        string
		isvc        *v1beta1.InferenceService
		config      *controllerconfig.IngressConfig
		expectedURL string
	}{
		{
			name: "host based url",
			isvc: createTestInferenceServiceWithEngineStatus("test-isvc", "default"),
			config: &controllerconfig.IngressConfig{
				UrlScheme:     "https",
				PathTemplate:  "",
				IngressDomain: "example.com",
			},
			expectedURL: "https://test-isvc.default.example.com",
		},
		{
			name: "path based url",
			isvc: createTestInferenceServiceWithEngineStatus("test-isvc", "default"),
			config: &controllerconfig.IngressConfig{
				UrlScheme:     "https",
				PathTemplate:  "/models/{{.Namespace}}/{{.Name}}",
				IngressDomain: "example.com",
			},
			expectedURL: "https://example.com/models/default/test-isvc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &ServerlessStrategy{
				ingressConfig: tt.config,
				pathService:   services.NewPathService(),
			}
			url := strategy.getServiceUrl(tt.isvc)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestServerlessStrategy_GetHostPrefix(t *testing.T) {
	tests := []struct {
		name                    string
		isvc                    *v1beta1.InferenceService
		disableIstioVirtualHost bool
		expectedPrefix          string
	}{
		{
			name:                    "with virtual host enabled and router",
			isvc:                    createTestInferenceServiceWithRouter("test-isvc", "default"),
			disableIstioVirtualHost: false,
			expectedPrefix:          "test-isvc",
		},
		{
			name:                    "with virtual host enabled and no router",
			isvc:                    createTestInferenceService("test-isvc", "default"),
			disableIstioVirtualHost: false,
			expectedPrefix:          "test-isvc",
		},
		{
			name:                    "with virtual host disabled and router",
			isvc:                    createTestInferenceServiceWithRouter("test-isvc", "default"),
			disableIstioVirtualHost: true,
			expectedPrefix:          "test-isvc-router-default",
		},
		{
			name:                    "with virtual host disabled and no router",
			isvc:                    createTestInferenceService("test-isvc", "default"),
			disableIstioVirtualHost: true,
			expectedPrefix:          "test-isvc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createServerlessStrategy(t)
			prefix := strategy.(*ServerlessStrategy).getHostPrefix(tt.isvc, tt.disableIstioVirtualHost)
			assert.Equal(t, tt.expectedPrefix, prefix)
		})
	}
}

func TestServerlessStrategy_ReconcileVirtualService(t *testing.T) {
	tests := []struct {
		name                   string
		isvc                   *v1beta1.InferenceService
		ingressConfig          *controllerconfig.IngressConfig
		existingVirtualService *istioclientv1beta1.VirtualService
		expectedError          bool
		expectVirtualService   bool
	}{
		{
			name: "create new virtual service",
			isvc: createTestInferenceService("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIstioVirtualHost: false,
			},
			expectedError:        false,
			expectVirtualService: true,
		},
		{
			name: "virtual service disabled",
			isvc: createTestInferenceService("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				DisableIstioVirtualHost: true,
			},
			expectedError:        false,
			expectVirtualService: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, istioclientv1beta1.AddToScheme(scheme))

			setComponentStatusReady(tt.isvc)

			objs := []client.Object{tt.isvc}
			if tt.existingVirtualService != nil {
				objs = append(objs, tt.existingVirtualService)
			}

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			strategy := createServerlessStrategyWithConfig(t, tt.ingressConfig)
			strategy.(*ServerlessStrategy).client = fakeClient

			err := strategy.(*ServerlessStrategy).reconcileVirtualService(context.Background(), tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServerlessStrategy_ReconcileExternalService(t *testing.T) {
	tests := []struct {
		name            string
		isvc            *v1beta1.InferenceService
		ingressConfig   *controllerconfig.IngressConfig
		existingService *corev1.Service
		expectedError   bool
	}{
		{
			name: "create new external service",
			isvc: createTestInferenceService("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				LocalGatewayServiceName: "knative-local-gateway",
				DisableIstioVirtualHost: false,
			},
			expectedError: false,
		},
		{
			name: "update existing external service",
			isvc: createTestInferenceService("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				LocalGatewayServiceName: "knative-local-gateway",
				DisableIstioVirtualHost: false,
			},
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeExternalName,
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			objs := []client.Object{tt.isvc}
			if tt.existingService != nil {
				objs = append(objs, tt.existingService)
			}

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			strategy := createServerlessStrategyWithConfig(t, tt.ingressConfig)
			strategy.(*ServerlessStrategy).client = fakeClient

			err := strategy.(*ServerlessStrategy).reconcileExternalService(context.Background(), tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper functions
func createServerlessStrategy(t *testing.T) interfaces.IngressStrategy {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	fakeClientset := fake.NewSimpleClientset()

	opts := interfaces.ReconcilerOptions{
		Client: fakeClient,
		Scheme: scheme,
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
	}

	return NewServerlessStrategy(opts, fakeClientset, services.NewDomainService(), services.NewPathService())
}

func createServerlessStrategyWithConfig(t *testing.T, config *controllerconfig.IngressConfig) interfaces.IngressStrategy {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	fakeClientset := fake.NewSimpleClientset()

	opts := interfaces.ReconcilerOptions{
		Client:        fakeClient,
		Scheme:        scheme,
		IngressConfig: config,
		IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
	}

	return NewServerlessStrategy(opts, fakeClientset, services.NewDomainService(), services.NewPathService())
}

func createTestInferenceService(name, namespace string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
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
		},
	}
}

func createTestInferenceServiceWithRouter(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceService(name, namespace)
	isvc.Spec.Router = &v1beta1.RouterSpec{}
	return isvc
}

func createTestInferenceServiceWithEngineStatus(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceService(name, namespace)
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.EngineComponent: {
			URL: &apis.URL{
				Scheme: "http",
				Host:   name + "-engine-default.default.example.com",
			},
		},
	}
	return isvc
}

func createTestInferenceServiceWithRouterStatus(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceWithRouter(name, namespace)
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.RouterComponent: {
			URL: &apis.URL{
				Scheme: "http",
				Host:   name + "-router-default.default.example.com",
			},
		},
	}
	return isvc
}

func setComponentStatusReady(isvc *v1beta1.InferenceService) {
	isvc.Status.SetCondition(v1beta1.PredictorReady, &apis.Condition{
		Type:   v1beta1.PredictorReady,
		Status: corev1.ConditionTrue,
	})

	isvc.Status.SetCondition(v1beta1.EngineReady, &apis.Condition{
		Type:   v1beta1.EngineReady,
		Status: corev1.ConditionTrue,
	})

	// Set up component status with URLs that serverless strategy needs
	if isvc.Status.Components == nil {
		isvc.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
	}

	// Set engine component status with URL (serverless strategy uses EngineComponent)
	isvc.Status.Components[v1beta1.EngineComponent] = v1beta1.ComponentStatusSpec{
		URL: &apis.URL{
			Scheme: "http",
			Host:   isvc.Name + "-engine-default." + isvc.Namespace + ".example.com",
		},
	}

	if isvc.Spec.Router != nil {
		isvc.Status.SetCondition(v1beta1.RoutesReady, &apis.Condition{
			Type:   v1beta1.RoutesReady,
			Status: corev1.ConditionTrue,
		})

		// Set router component status with URL
		isvc.Status.Components[v1beta1.RouterComponent] = v1beta1.ComponentStatusSpec{
			URL: &apis.URL{
				Scheme: "http",
				Host:   isvc.Name + "-router-default." + isvc.Namespace + ".example.com",
			},
		}
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestServerlessStrategy_URLWithPort(t *testing.T) {
	tests := []struct {
		name                string
		isvc                *v1beta1.InferenceService
		ingressConfig       *controllerconfig.IngressConfig
		services            []corev1.Service
		expectedURLHost     string
		expectedAddressHost string
	}{
		{
			name: "engine only with custom port",
			isvc: createTestInferenceServiceWithEngineStatus("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				IngressGateway:             "knative-serving/knative-ingress-gateway",
				LocalGateway:               "knative-serving/knative-local-gateway",
				IngressDomain:              "example.com",
				LocalGatewayServiceName:    "knative-local-gateway",
				KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
				DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:                  "https",
				DisableIstioVirtualHost:    true, // Skip VirtualService creation to focus on URL
			},
			services: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc", // PredictorServiceName returns just the name
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 8081},
						},
					},
				},
			},
			// URL host comes from engine status URL (test-isvc-engine-default.default.example.com)
			expectedURLHost:     "test-isvc-engine-default.default.example.com:8081",
			expectedAddressHost: "test-isvc.default.svc.cluster.local:8081",
		},
		{
			name: "with router and custom port",
			isvc: createTestInferenceServiceWithRouterStatus("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				IngressGateway:             "knative-serving/knative-ingress-gateway",
				LocalGateway:               "knative-serving/knative-local-gateway",
				IngressDomain:              "example.com",
				LocalGatewayServiceName:    "knative-local-gateway",
				KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
				DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:                  "https",
				DisableIstioVirtualHost:    true,
			},
			services: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-isvc-router-default", // DefaultRouterServiceName
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 8082},
						},
					},
				},
			},
			// URL host comes from router status URL (test-isvc-router-default.default.example.com)
			expectedURLHost:     "test-isvc-router-default.default.example.com:8082",
			expectedAddressHost: "test-isvc-router-default.default.svc.cluster.local:8082",
		},
		{
			name: "service not found uses default port",
			isvc: createTestInferenceServiceWithEngineStatus("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				IngressGateway:             "knative-serving/knative-ingress-gateway",
				LocalGateway:               "knative-serving/knative-local-gateway",
				IngressDomain:              "example.com",
				LocalGatewayServiceName:    "knative-local-gateway",
				KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
				DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:                  "https",
				DisableIstioVirtualHost:    true,
			},
			services: []corev1.Service{}, // No services
			// URL host comes from engine status URL
			expectedURLHost:     "test-isvc-engine-default.default.example.com:8080",
			expectedAddressHost: "test-isvc.default.svc.cluster.local:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, istioclientv1beta1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			setComponentStatusReady(tt.isvc)

			objs := []client.Object{tt.isvc}
			for i := range tt.services {
				objs = append(objs, &tt.services[i])
			}

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()
			fakeClientset := fake.NewSimpleClientset()

			opts := interfaces.ReconcilerOptions{
				Client:        fakeClient,
				Scheme:        scheme,
				IngressConfig: tt.ingressConfig,
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			}
			strategy := NewServerlessStrategy(opts, fakeClientset, services.NewDomainService(), services.NewPathService())

			err := strategy.Reconcile(context.Background(), tt.isvc)

			assert.NoError(t, err)
			assert.NotNil(t, tt.isvc.Status.URL)
			assert.Equal(t, tt.expectedURLHost, tt.isvc.Status.URL.Host)
			assert.NotNil(t, tt.isvc.Status.Address)
			assert.Equal(t, tt.expectedAddressHost, tt.isvc.Status.Address.URL.Host)
		})
	}
}
