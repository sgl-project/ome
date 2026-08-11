package builders

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestBackendTrafficPolicyBuilder_Build(t *testing.T) {
	tests := []struct {
		name              string
		isvc              *v1beta1.InferenceService
		headers           []string
		expectedName      string
		expectedNamespace string
		expectedHeaders   []string
	}{
		{
			name:              "PD deployment with router uses default headers",
			isvc:              createTestISVCWithRouter("test-isvc", "playground"),
			headers:           controllerconfig.DefaultConsistentHashHeaders,
			expectedName:      "test-isvc",
			expectedNamespace: "playground",
			expectedHeaders:   []string{"x-routing-key"},
		},
		{
			name:              "non-PD deployment engine only uses default headers",
			isvc:              createTestISVCEngineOnly("test-engine", "default"),
			headers:           controllerconfig.DefaultConsistentHashHeaders,
			expectedName:      "test-engine",
			expectedNamespace: "default",
			expectedHeaders:   []string{"x-routing-key"},
		},
		{
			name:              "custom consistent hash headers via config",
			isvc:              createTestISVCEngineOnly("test-custom", "default"),
			headers:           []string{"x-custom-key"},
			expectedName:      "test-custom",
			expectedNamespace: "default",
			expectedHeaders:   []string{"x-custom-key"},
		},
		{
			name: "annotation overrides config",
			isvc: createTestISVCWithAnnotations("test-anno", "default", map[string]string{
				constants.IngressConsistentHashHeaders: "x-override-a, x-override-b",
			}),
			headers:           controllerconfig.DefaultConsistentHashHeaders,
			expectedName:      "test-anno",
			expectedNamespace: "default",
			expectedHeaders:   []string{"x-override-a", "x-override-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &controllerconfig.IngressConfig{ConsistentHashHeaders: tt.headers}
			builder := NewBackendTrafficPolicyBuilder(cfg)
			policy := builder.Build(tt.isvc)

			require.NotNil(t, policy)
			assert.Equal(t, tt.expectedName, policy.GetName())
			assert.Equal(t, tt.expectedNamespace, policy.GetNamespace())
			assert.Equal(t, BackendTrafficPolicyGVK, policy.GroupVersionKind())

			spec, ok := policy.Object["spec"].(map[string]interface{})
			require.True(t, ok)

			// Route BTP must opt into merging so it inherits the Gateway-level
			// parent's settings (e.g. circuitBreaker, timeout) instead of shadowing them.
			assert.Equal(t, "StrategicMerge", spec["mergeType"])

			targetRefs, ok := spec["targetRefs"].([]interface{})
			require.True(t, ok)
			require.Len(t, targetRefs, 1)

			ref := targetRefs[0].(map[string]interface{})
			assert.Equal(t, "gateway.networking.k8s.io", ref["group"])
			assert.Equal(t, "HTTPRoute", ref["kind"])
			assert.Equal(t, tt.expectedName, ref["name"])

			lb, ok := spec["loadBalancer"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "ConsistentHash", lb["type"])

			ch := lb["consistentHash"].(map[string]interface{})
			assert.Equal(t, "Headers", ch["type"])

			headers := ch["headers"].([]interface{})
			require.Len(t, headers, len(tt.expectedHeaders))
			for i, expected := range tt.expectedHeaders {
				assert.Equal(t, expected, headers[i].(map[string]interface{})["name"])
			}

			labels := policy.GetLabels()
			assert.Equal(t, tt.expectedName, labels["app.kubernetes.io/part-of"])
			assert.Equal(t, "ome-controller", labels["app.kubernetes.io/managed-by"])
		})
	}
}

func createTestISVCWithRouter(name, namespace string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1beta1.InferenceServiceSpec{
			Engine: &v1beta1.EngineSpec{},
			Router: &v1beta1.RouterSpec{},
		},
		Status: v1beta1.InferenceServiceStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{}},
		},
	}
}

func createTestISVCWithAnnotations(name, namespace string, annotations map[string]string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations},
		Spec: v1beta1.InferenceServiceSpec{
			Engine: &v1beta1.EngineSpec{},
		},
		Status: v1beta1.InferenceServiceStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{}},
		},
	}
}

func createTestISVCEngineOnly(name, namespace string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1beta1.InferenceServiceSpec{
			Engine: &v1beta1.EngineSpec{},
		},
		Status: v1beta1.InferenceServiceStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{}},
		},
	}
}
