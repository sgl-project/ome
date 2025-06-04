package builders

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
)

func TestVirtualServiceBuilder_GetResourceType(t *testing.T) {
	builder := createVirtualServiceBuilder()
	assert.Equal(t, "VirtualService", builder.GetResourceType())
}

func TestVirtualServiceBuilder_Build(t *testing.T) {
	builder := createVirtualServiceBuilder()
	isvc := createTestInferenceServiceVirtualService("test-isvc", "default")
	setEngineReadyVirtualService(isvc)

	vs, err := builder.Build(context.Background(), isvc)
	assert.NoError(t, err)
	assert.NotNil(t, vs)

	virtualService, ok := vs.(*istioclientv1beta1.VirtualService)
	assert.True(t, ok)
	assert.Equal(t, "test-isvc", virtualService.Name)
	assert.Equal(t, "default", virtualService.Namespace)
}

func TestVirtualServiceBuilder_BuildVirtualService(t *testing.T) {
	tests := []struct {
		name             string
		isvc             *v1beta1.InferenceService
		domainList       *[]string
		expectedError    bool
		expectNil        bool
		expectedHosts    int
		expectedRoutes   int
		expectedGateways int
	}{
		{
			name:             "engine only",
			isvc:             createTestInferenceServiceVirtualService("test-isvc", "default"),
			domainList:       nil,
			expectedError:    false,
			expectedHosts:    2, // internal + external
			expectedRoutes:   1,
			expectedGateways: 3, // local + mesh + ingress
		},
		{
			name:             "with router",
			isvc:             createTestInferenceServiceWithRouterVirtualService("test-isvc", "default"),
			domainList:       nil,
			expectedError:    false,
			expectedHosts:    2,
			expectedRoutes:   1,
			expectedGateways: 3,
		},
		{
			name:             "with decoder",
			isvc:             createTestInferenceServiceWithDecoderVirtualService("test-isvc", "default"),
			domainList:       nil,
			expectedError:    false,
			expectedHosts:    2,
			expectedRoutes:   2, // decoder + engine
			expectedGateways: 3,
		},
		{
			name:          "predictor not ready",
			isvc:          createTestInferenceServiceVirtualService("test-isvc", "default"),
			expectNil:     true,
			expectedError: false,
		},
		{
			name:             "cluster local visibility",
			isvc:             createTestInferenceServiceClusterLocal("test-isvc", "default"),
			domainList:       nil,
			expectedError:    false,
			expectedHosts:    1, // only internal
			expectedRoutes:   1,
			expectedGateways: 2, // local + mesh (no ingress)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()

			if !tt.expectNil {
				setEngineReadyVirtualService(tt.isvc)
				if tt.isvc.Spec.Router != nil {
					setRouterReadyVirtualService(tt.isvc)
				}
				if tt.isvc.Spec.Decoder != nil {
					setDecoderReadyVirtualService(tt.isvc)
				}
			}

			result, err := builder.BuildVirtualService(context.Background(), tt.isvc, tt.domainList)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			vs, ok := result.(*istioclientv1beta1.VirtualService)
			require.True(t, ok)

			assert.Equal(t, tt.isvc.Name, vs.Name)
			assert.Equal(t, tt.isvc.Namespace, vs.Namespace)
			assert.Len(t, vs.Spec.Hosts, tt.expectedHosts)
			assert.Len(t, vs.Spec.Http, tt.expectedRoutes)
			assert.Len(t, vs.Spec.Gateways, tt.expectedGateways)
		})
	}
}

func TestVirtualServiceBuilder_DetermineIfInternal(t *testing.T) {
	tests := []struct {
		name             string
		isvc             *v1beta1.InferenceService
		expectedInternal bool
	}{
		{
			name:             "external service",
			isvc:             createTestInferenceServiceVirtualService("test-isvc", "default"),
			expectedInternal: false,
		},
		{
			name:             "cluster local visibility",
			isvc:             createTestInferenceServiceClusterLocal("test-isvc", "default"),
			expectedInternal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()
			// Mock component status to provide service host
			mockComponentStatus(tt.isvc)

			internal := builder.determineIfInternal(tt.isvc)
			assert.Equal(t, tt.expectedInternal, internal)
		})
	}
}

func TestVirtualServiceBuilder_GetServiceHost(t *testing.T) {
	tests := []struct {
		name         string
		isvc         *v1beta1.InferenceService
		expectedHost string
	}{
		{
			name:         "engine only",
			isvc:         createTestInferenceServiceWithEngineStatusVirtualService("test-isvc", "default"),
			expectedHost: "test-isvc.default.example.com",
		},
		{
			name:         "with router",
			isvc:         createTestInferenceServiceWithRouterStatusVirtualService("test-isvc", "default"),
			expectedHost: "test-isvc.default.example.com",
		},
		{
			name:         "no components",
			isvc:         createTestInferenceServiceVirtualService("test-isvc", "default"),
			expectedHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()
			host := builder.getServiceHost(tt.isvc)
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}

func TestVirtualServiceBuilder_BuildHTTPRoutes(t *testing.T) {
	tests := []struct {
		name           string
		isvc           *v1beta1.InferenceService
		isInternal     bool
		expectedRoutes int
	}{
		{
			name:           "engine only external",
			isvc:           createTestInferenceServiceVirtualService("test-isvc", "default"),
			isInternal:     false,
			expectedRoutes: 1,
		},
		{
			name:           "with decoder external",
			isvc:           createTestInferenceServiceWithDecoderVirtualService("test-isvc", "default"),
			isInternal:     false,
			expectedRoutes: 2,
		},
		{
			name:           "engine only internal",
			isvc:           createTestInferenceServiceVirtualService("test-isvc", "default"),
			isInternal:     true,
			expectedRoutes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()
			serviceHost := "test-isvc.default.example.com"
			backend := "test-isvc"

			// Set decoder ready for decoder tests
			if tt.isvc.Spec.Decoder != nil {
				setDecoderReadyVirtualService(tt.isvc)
			}

			routes := builder.buildHTTPRoutes(tt.isvc, serviceHost, nil, tt.isInternal, backend)

			assert.Len(t, routes, tt.expectedRoutes)
		})
	}
}

func TestVirtualServiceBuilder_BuildPathBasedRoutes(t *testing.T) {
	tests := []struct {
		name           string
		isvc           *v1beta1.InferenceService
		backend        string
		expectedRoutes int
		expectedHosts  int
	}{
		{
			name:           "engine only",
			isvc:           createTestInferenceServiceVirtualService("test-isvc", "default"),
			backend:        "test-isvc",
			expectedRoutes: 1,
			expectedHosts:  1,
		},
		{
			name:           "with decoder",
			isvc:           createTestInferenceServiceWithDecoderVirtualService("test-isvc", "default"),
			backend:        "test-isvc",
			expectedRoutes: 2,
			expectedHosts:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilderWithPath()

			routes, hosts := builder.buildPathBasedRoutes(tt.isvc, tt.backend)

			assert.Len(t, routes, tt.expectedRoutes)
			assert.Len(t, hosts, tt.expectedHosts)
		})
	}
}

func TestVirtualServiceBuilder_AddAdditionalHosts(t *testing.T) {
	tests := []struct {
		name            string
		hosts           []string
		additionalHosts *[]string
		expectedHosts   int
	}{
		{
			name:            "no additional hosts",
			hosts:           []string{"host1.example.com"},
			additionalHosts: nil,
			expectedHosts:   1,
		},
		{
			name:            "with additional hosts",
			hosts:           []string{"host1.example.com"},
			additionalHosts: &[]string{"host2.example.com", "host3.example.com"},
			expectedHosts:   3,
		},
		{
			name:            "duplicate hosts",
			hosts:           []string{"host1.example.com"},
			additionalHosts: &[]string{"host1.example.com", "host2.example.com"},
			expectedHosts:   2, // duplicates filtered out
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()

			result := builder.addAdditionalHosts(tt.hosts, tt.additionalHosts)

			assert.Len(t, result, tt.expectedHosts)
		})
	}
}

func TestVirtualServiceBuilder_CreateHTTPRouteDestination(t *testing.T) {
	builder := createVirtualServiceBuilder()

	destination := builder.createHTTPRouteDestination("test-gateway-service")

	assert.Equal(t, "test-gateway-service", destination.Destination.Host)
	assert.Equal(t, uint32(80), destination.Destination.Port.Number)
	assert.Equal(t, int32(100), destination.Weight)
}

func TestVirtualServiceBuilder_CreateHTTPMatchRequest(t *testing.T) {
	tests := []struct {
		name            string
		prefix          string
		targetHost      string
		internalHost    string
		additionalHosts *[]string
		isInternal      bool
		expectedMatches int
	}{
		{
			name:            "external service",
			prefix:          "",
			targetHost:      "test-isvc.default.example.com",
			internalHost:    "test-isvc.default.svc.cluster.local",
			additionalHosts: nil,
			isInternal:      false,
			expectedMatches: 2, // internal + external
		},
		{
			name:            "internal service",
			prefix:          "",
			targetHost:      "test-isvc.default.svc.cluster.local",
			internalHost:    "test-isvc.default.svc.cluster.local",
			additionalHosts: nil,
			isInternal:      true,
			expectedMatches: 1, // only internal
		},
		{
			name:            "with additional hosts",
			prefix:          "",
			targetHost:      "test-isvc.default.example.com",
			internalHost:    "test-isvc.default.svc.cluster.local",
			additionalHosts: &[]string{"test-alt.example.com"},
			isInternal:      false,
			expectedMatches: 3, // internal + external + additional
		},
		{
			name:            "with prefix",
			prefix:          "^/explain/.*$",
			targetHost:      "test-isvc.default.example.com",
			internalHost:    "test-isvc.default.svc.cluster.local",
			additionalHosts: nil,
			isInternal:      false,
			expectedMatches: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()

			matches := builder.createHTTPMatchRequest(tt.prefix, tt.targetHost, tt.internalHost, tt.additionalHosts, tt.isInternal)

			assert.Len(t, matches, tt.expectedMatches)

			// Check that all matches have proper URI and Authority
			for _, match := range matches {
				if tt.prefix != "" {
					assert.NotNil(t, match.Uri)
					assert.Equal(t, tt.prefix, match.Uri.GetRegex())
				}
				assert.NotNil(t, match.Authority)
				assert.NotEmpty(t, match.Gateways)
			}
		})
	}
}

func TestVirtualServiceBuilder_StringMatchEqual(t *testing.T) {
	builder := createVirtualServiceBuilder()

	tests := []struct {
		name     string
		match1   *istiov1beta1.StringMatch
		match2   *istiov1beta1.StringMatch
		expected bool
	}{
		{
			name:     "both nil",
			match1:   nil,
			match2:   nil,
			expected: true,
		},
		{
			name: "same regex matches",
			match1: &istiov1beta1.StringMatch{
				MatchType: &istiov1beta1.StringMatch_Regex{Regex: "test"},
			},
			match2: &istiov1beta1.StringMatch{
				MatchType: &istiov1beta1.StringMatch_Regex{Regex: "test"},
			},
			expected: true,
		},
		{
			name: "different match types",
			match1: &istiov1beta1.StringMatch{
				MatchType: &istiov1beta1.StringMatch_Regex{Regex: "test"},
			},
			match2: &istiov1beta1.StringMatch{
				MatchType: &istiov1beta1.StringMatch_Exact{Exact: "test"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.stringMatchEqual(tt.match1, tt.match2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVirtualServiceBuilder_ComponentReadiness(t *testing.T) {
	tests := []struct {
		name              string
		isvc              *v1beta1.InferenceService
		setupReadiness    func(*v1beta1.InferenceService)
		expectedNil       bool
		expectedCondition corev1.ConditionStatus
		expectedReason    string
	}{
		{
			name: "router not ready",
			isvc: createTestInferenceServiceWithRouterVirtualService("test-isvc", "default"),
			setupReadiness: func(isvc *v1beta1.InferenceService) {
				setEngineReadyVirtualService(isvc)
				// Don't set router ready
			},
			expectedNil:       true,
			expectedCondition: corev1.ConditionUnknown,
			expectedReason:    "Router ingress not created",
		},
		{
			name: "decoder not ready",
			isvc: createTestInferenceServiceWithDecoderVirtualService("test-isvc", "default"),
			setupReadiness: func(isvc *v1beta1.InferenceService) {
				setEngineReadyVirtualService(isvc)
				// Don't set decoder ready - this should still create VirtualService
			},
			expectedNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createVirtualServiceBuilder()
			tt.setupReadiness(tt.isvc)

			result, err := builder.BuildVirtualService(context.Background(), tt.isvc, nil)

			assert.NoError(t, err)

			if tt.expectedNil {
				assert.Nil(t, result)
				condition := tt.isvc.Status.GetCondition(v1beta1.IngressReady)
				assert.NotNil(t, condition)
				assert.Equal(t, tt.expectedCondition, condition.Status)
				assert.Equal(t, tt.expectedReason, condition.Reason)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

// Helper functions
func createVirtualServiceBuilder() *VirtualServiceBuilder {
	return &VirtualServiceBuilder{
		ingressConfig: &controllerconfig.IngressConfig{
			IngressGateway:             "istio-system/istio-gateway",
			LocalGateway:               "istio-system/istio-local-gateway",
			IngressDomain:              "example.com",
			KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
			DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
		},
		isvcConfig:    &controllerconfig.InferenceServicesConfig{},
		domainService: services.NewDomainService(),
		pathService:   services.NewPathService(),
	}
}

func createVirtualServiceBuilderWithPath() *VirtualServiceBuilder {
	return &VirtualServiceBuilder{
		ingressConfig: &controllerconfig.IngressConfig{
			IngressGateway:             "istio-system/istio-gateway",
			LocalGateway:               "istio-system/istio-local-gateway",
			IngressDomain:              "example.com",
			KnativeLocalGatewayService: "knative-local-gateway.istio-system.svc.cluster.local",
			DomainTemplate:             "{{.Name}}.{{.Namespace}}.{{.IngressDomain}}",
			PathTemplate:               "/models/{{.Namespace}}/{{.Name}}",
		},
		isvcConfig:    &controllerconfig.InferenceServicesConfig{},
		domainService: services.NewDomainService(),
		pathService:   services.NewPathService(),
	}
}

func createTestInferenceServiceVirtualService(name, namespace string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					Runtime: stringPtrVirtualService("sklearn"),
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

func createTestInferenceServiceClusterLocal(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceVirtualService(name, namespace)
	isvc.Labels = map[string]string{
		constants.VisibilityLabel: constants.ClusterLocalVisibility,
	}
	return isvc
}

func createTestInferenceServiceWithRouterVirtualService(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceVirtualService(name, namespace)
	isvc.Spec.Router = &v1beta1.RouterSpec{}
	return isvc
}

func createTestInferenceServiceWithDecoderVirtualService(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceVirtualService(name, namespace)
	isvc.Spec.Decoder = &v1beta1.DecoderSpec{}
	return isvc
}

func createTestInferenceServiceWithEngineStatusVirtualService(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceVirtualService(name, namespace)
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.EngineComponent: {
			URL: &apis.URL{
				Scheme: "http",
				Host:   name + "-engine-default." + namespace + ".example.com",
			},
		},
	}
	return isvc
}

func createTestInferenceServiceWithRouterStatusVirtualService(name, namespace string) *v1beta1.InferenceService {
	isvc := createTestInferenceServiceWithRouterVirtualService(name, namespace)
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.RouterComponent: {
			URL: &apis.URL{
				Scheme: "http",
				Host:   name + "-router-default." + namespace + ".example.com",
			},
		},
	}
	return isvc
}

func mockComponentStatus(isvc *v1beta1.InferenceService) {
	if isvc.Status.Components == nil {
		isvc.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
	}

	isvc.Status.Components[v1beta1.EngineComponent] = v1beta1.ComponentStatusSpec{
		URL: &apis.URL{
			Scheme: "http",
			Host:   isvc.Name + "-engine-default." + isvc.Namespace + ".example.com",
		},
	}
}

func setEngineReadyVirtualService(isvc *v1beta1.InferenceService) {
	isvc.Status.SetCondition(v1beta1.PredictorReady, &apis.Condition{
		Type:   v1beta1.PredictorReady,
		Status: corev1.ConditionTrue,
	})
	isvc.Status.SetCondition(v1beta1.EngineReady, &apis.Condition{
		Type:   v1beta1.EngineReady,
		Status: corev1.ConditionTrue,
	})
	mockComponentStatus(isvc)
}

func setRouterReadyVirtualService(isvc *v1beta1.InferenceService) {
	isvc.Status.SetCondition(v1beta1.RoutesReady, &apis.Condition{
		Type:   v1beta1.RoutesReady,
		Status: corev1.ConditionTrue,
	})
}

func setDecoderReadyVirtualService(isvc *v1beta1.InferenceService) {
	isvc.Status.SetCondition(v1beta1.DecoderReady, &apis.Condition{
		Type:   v1beta1.DecoderReady,
		Status: corev1.ConditionTrue,
	})
}

func stringPtrVirtualService(s string) *string {
	return &s
}
