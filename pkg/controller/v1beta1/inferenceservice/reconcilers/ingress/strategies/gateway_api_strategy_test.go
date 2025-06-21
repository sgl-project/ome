package strategies

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
)

func TestGatewayAPIStrategy_GetName(t *testing.T) {
	strategy := createGatewayAPIStrategy(t)
	assert.Equal(t, "GatewayAPI", strategy.GetName())
}

func TestGatewayAPIStrategy_Reconcile(t *testing.T) {
	tests := []struct {
		name                    string
		isvc                    *v1beta1.InferenceService
		ingressConfig           *controllerconfig.IngressConfig
		existingHTTPRoutes      []client.Object
		expectedError           bool
		expectedIngressReady    corev1.ConditionStatus
		expectedHTTPRoutesCount int
	}{
		{
			name: "successful reconcile with predictor only",
			isvc: createTestInferenceServiceGateway("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			expectedError:           false,
			expectedIngressReady:    corev1.ConditionTrue,
			expectedHTTPRoutesCount: 2, // engine + toplevel
		},
		{
			name: "successful reconcile with router",
			isvc: createTestInferenceServiceWithRouterGateway("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			expectedError:           false,
			expectedIngressReady:    corev1.ConditionTrue,
			expectedHTTPRoutesCount: 3, // engine + router + toplevel
		},
		{
			name: "successful reconcile with decoder",
			isvc: createTestInferenceServiceWithDecoderGateway("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			expectedError:           false,
			expectedIngressReady:    corev1.ConditionTrue,
			expectedHTTPRoutesCount: 3, // engine + decoder + toplevel
		},
		{
			name: "ingress creation disabled",
			isvc: createTestInferenceServiceGateway("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: true,
			},
			expectedError:           false,
			expectedIngressReady:    corev1.ConditionTrue,
			expectedHTTPRoutesCount: 0,
		},
		{
			name: "cluster local visibility",
			isvc: createTestInferenceServiceWithClusterLocal("test-isvc", "default"),
			ingressConfig: &controllerconfig.IngressConfig{
				EnableGatewayAPI:       true,
				IngressDomain:          "example.com",
				OmeIngressGateway:      "istio-system/gateway",
				DomainTemplate:         "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
				UrlScheme:              "https",
				DisableIngressCreation: false,
			},
			expectedError:           false,
			expectedIngressReady:    corev1.ConditionTrue,
			expectedHTTPRoutesCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, gatewayapiv1.Install(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			// Set component statuses to ready
			setComponentStatusReadyGateway(tt.isvc)

			objs := []client.Object{tt.isvc}
			objs = append(objs, tt.existingHTTPRoutes...)

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Use the proper constructor to create the strategy with builder
			opts := interfaces.ReconcilerOptions{
				Client:        fakeClient,
				Scheme:        scheme,
				IngressConfig: tt.ingressConfig,
				IsvcConfig:    &controllerconfig.InferenceServicesConfig{},
			}
			strategy := NewGatewayAPIStrategy(opts, services.NewDomainService(), services.NewPathService())

			err := strategy.Reconcile(context.Background(), tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIngressReady, tt.isvc.Status.GetCondition(v1beta1.IngressReady).Status)
			}
		})
	}
}

func TestGatewayAPIStrategy_ReconcileComponentHTTPRoute(t *testing.T) {
	tests := []struct {
		name           string
		isvc           *v1beta1.InferenceService
		componentType  string
		existingRoute  *gatewayapiv1.HTTPRoute
		expectedError  bool
		expectedCreate bool
		expectedUpdate bool
	}{
		{
			name:           "create new engine HTTPRoute",
			isvc:           createTestInferenceServiceGateway("test-isvc", "default"),
			componentType:  "engine",
			expectedError:  false,
			expectedCreate: true,
		},
		{
			name:           "create new router HTTPRoute",
			isvc:           createTestInferenceServiceWithRouterGateway("test-isvc", "default"),
			componentType:  "router",
			expectedError:  false,
			expectedCreate: true,
		},
		{
			name:           "create new decoder HTTPRoute",
			isvc:           createTestInferenceServiceWithDecoderGateway("test-isvc", "default"),
			componentType:  "decoder",
			expectedError:  false,
			expectedCreate: true,
		},
		{
			name:          "update existing HTTPRoute",
			isvc:          createTestInferenceServiceGateway("test-isvc", "default"),
			componentType: "engine",
			existingRoute: &gatewayapiv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-isvc",
					Namespace: "default",
				},
			},
			expectedError:  false,
			expectedUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, gatewayapiv1.Install(scheme))
			require.NoError(t, corev1.AddToScheme(scheme))

			setComponentStatusReadyGateway(tt.isvc)

			objs := []client.Object{tt.isvc}
			if tt.existingRoute != nil {
				objs = append(objs, tt.existingRoute)
			}

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			strategy := createGatewayAPIStrategyWithClient(t, fakeClient)

			err := strategy.(*GatewayAPIStrategy).reconcileComponentHTTPRoute(context.Background(), tt.isvc, tt.componentType)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGatewayAPIStrategy_CheckHTTPRouteStatuses(t *testing.T) {
	tests := []struct {
		name              string
		isvc              *v1beta1.InferenceService
		httpRoutes        []client.Object
		expectedError     bool
		expectedCondition corev1.ConditionStatus
	}{
		{
			name: "all HTTPRoutes ready",
			isvc: createTestInferenceServiceGateway("test-isvc", "default"),
			httpRoutes: []client.Object{
				createReadyHTTPRoute("test-isvc", "default"),     // engine (same name as isvc)
				createReadyHTTPRoute("test-isvc-top", "default"), // toplevel (different name)
			},
			expectedError:     false,
			expectedCondition: corev1.ConditionTrue,
		},
		{
			name: "HTTPRoute not ready",
			isvc: createTestInferenceServiceGateway("test-isvc", "default"),
			httpRoutes: []client.Object{
				createNotReadyHTTPRoute("test-isvc", "default"),  // engine
				createReadyHTTPRoute("test-isvc-top", "default"), // toplevel
			},
			expectedError:     false,
			expectedCondition: corev1.ConditionFalse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, v1beta1.AddToScheme(scheme))
			require.NoError(t, gatewayapiv1.Install(scheme))

			objs := []client.Object{tt.isvc}
			objs = append(objs, tt.httpRoutes...)

			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			strategy := createGatewayAPIStrategyWithClient(t, fakeClient)

			err := strategy.(*GatewayAPIStrategy).checkHTTPRouteStatuses(context.Background(), tt.isvc)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGatewayAPIStrategy_CreateRawURL(t *testing.T) {
	tests := []struct {
		name        string
		isvc        *v1beta1.InferenceService
		config      *controllerconfig.IngressConfig
		expectedURL string
	}{
		{
			name: "simple domain generation",
			isvc: createTestInferenceServiceGateway("test-isvc", "default"),
			config: &controllerconfig.IngressConfig{
				UrlScheme:      "https",
				IngressDomain:  "example.com",
				DomainTemplate: "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			},
			expectedURL: "https://test-isvc.default.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &GatewayAPIStrategy{
				ingressConfig: tt.config,
				domainService: services.NewDomainService(),
			}

			url, err := strategy.createRawURL(tt.isvc)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, url.String())
		})
	}
}

func TestGatewayAPIStrategy_GetRawServiceHost(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		expectedHost string
	}{
		{
			name:         "with router",
			isvc:         createTestInferenceServiceWithRouterGateway("test-isvc", "default"),
			expectedHost: "test-isvc-router.default.svc.cluster.local",
		},
		{
			name:         "without router",
			isvc:         createTestInferenceServiceGateway("test-isvc", "default"),
			expectedHost: "test-isvc.default.svc.cluster.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createGatewayAPIStrategy(t)
			host := strategy.(*GatewayAPIStrategy).getRawServiceHost(tt.isvc)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

func TestGatewayAPIStrategy_IsHTTPRouteReady(t *testing.T) {
	tests := []struct {
		name            string
		status          gatewayapiv1.HTTPRouteStatus
		expectedReady   bool
		expectedReason  *string
		expectedMessage *string
	}{
		{
			name:          "empty status",
			status:        gatewayapiv1.HTTPRouteStatus{},
			expectedReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createGatewayAPIStrategy(t)
			ready, _, _ := strategy.(*GatewayAPIStrategy).isHTTPRouteReady(tt.status)
			assert.Equal(t, tt.expectedReady, ready)
		})
	}
}

func TestGatewayAPIStrategy_GetComponentType(t *testing.T) {
	tests := []struct {
		name         string
		serviceName  string
		isvc         *v1beta1.InferenceService
		expectedType string
	}{
		{
			name:         "engine component",
			serviceName:  "test-isvc",
			isvc:         createTestInferenceServiceGateway("test-isvc", "default"),
			expectedType: "Engine",
		},
		{
			name:         "router component",
			serviceName:  "test-isvc-router",
			isvc:         createTestInferenceServiceGateway("test-isvc", "default"),
			expectedType: "Router",
		},
		{
			name:         "decoder component",
			serviceName:  "test-isvc-decoder",
			isvc:         createTestInferenceServiceGateway("test-isvc", "default"),
			expectedType: "Decoder",
		},
		{
			name:         "toplevel component",
			serviceName:  "test-isvc",
			isvc:         createTestInferenceServiceGateway("test-isvc", "default"),
			expectedType: "Engine", // For engine service name it returns Engine
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := createGatewayAPIStrategy(t)
			componentType := strategy.(*GatewayAPIStrategy).getComponentType(tt.serviceName, tt.isvc)
			assert.Equal(t, tt.expectedType, componentType)
		})
	}
}

// Helper functions
func createGatewayAPIStrategy(t *testing.T) interfaces.IngressStrategy {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, gatewayapiv1.Install(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	opts := interfaces.ReconcilerOptions{
		Client: fakeClient,
		Scheme: scheme,
		IngressConfig: &controllerconfig.IngressConfig{
			EnableGatewayAPI:  true,
			IngressDomain:     "example.com",
			OmeIngressGateway: "istio-system/gateway",
			DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			UrlScheme:         "https",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	return NewGatewayAPIStrategy(opts, services.NewDomainService(), services.NewPathService())
}

func createGatewayAPIStrategyWithClient(t *testing.T, client client.Client) interfaces.IngressStrategy {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, gatewayapiv1.Install(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	opts := interfaces.ReconcilerOptions{
		Client: client,
		Scheme: scheme,
		IngressConfig: &controllerconfig.IngressConfig{
			EnableGatewayAPI:  true,
			IngressDomain:     "example.com",
			OmeIngressGateway: "istio-system/gateway",
			DomainTemplate:    "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			UrlScheme:         "https",
		},
		IsvcConfig: &controllerconfig.InferenceServicesConfig{},
	}

	return NewGatewayAPIStrategy(opts, services.NewDomainService(), services.NewPathService())
}

func createTestInferenceServiceGateway(name, namespace string) *v1beta1.InferenceService {
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

func createTestInferenceServiceWithRouterGateway(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceGateway(name, namespace)
	isvc.Spec.Router = &v1beta1.RouterSpec{}
	return isvc
}

func createTestInferenceServiceWithDecoderGateway(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceGateway(name, namespace)
	isvc.Spec.Decoder = &v1beta1.DecoderSpec{}
	return isvc
}

func createTestInferenceServiceWithClusterLocal(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceGateway(name, namespace)
	isvc.Labels = map[string]string{
		constants.VisibilityLabel: constants.ClusterLocalVisibility,
	}
	return isvc
}

func setComponentStatusReadyGateway(isvc *v1beta1.InferenceService) {
	isvc.Status.SetCondition(v1beta1.PredictorReady, &apis.Condition{
		Type:   v1beta1.PredictorReady,
		Status: corev1.ConditionTrue,
	})

	isvc.Status.SetCondition(v1beta1.EngineReady, &apis.Condition{
		Type:   v1beta1.EngineReady,
		Status: corev1.ConditionTrue,
	})

	if isvc.Spec.Router != nil {
		isvc.Status.SetCondition(v1beta1.RoutesReady, &apis.Condition{
			Type:   v1beta1.RoutesReady,
			Status: corev1.ConditionTrue,
		})
	}

	if isvc.Spec.Decoder != nil {
		isvc.Status.SetCondition(v1beta1.DecoderReady, &apis.Condition{
			Type:   v1beta1.DecoderReady,
			Status: corev1.ConditionTrue,
		})
	}
}

func createReadyHTTPRoute(name, namespace string) *gatewayapiv1.HTTPRoute {
	return &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		// Simplified without detailed status
	}
}

func createNotReadyHTTPRoute(name, namespace string) *gatewayapiv1.HTTPRoute {
	return &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		// Simplified without detailed status
	}
}
