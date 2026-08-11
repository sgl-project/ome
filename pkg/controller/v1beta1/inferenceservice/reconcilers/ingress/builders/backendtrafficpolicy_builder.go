package builders

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

var BackendTrafficPolicyGVK = schema.GroupVersionKind{
	Group:   "gateway.envoyproxy.io",
	Version: "v1alpha1",
	Kind:    "BackendTrafficPolicy",
}

type BackendTrafficPolicyBuilder struct {
	ingressConfig *controllerconfig.IngressConfig
}

func NewBackendTrafficPolicyBuilder(ingressConfig *controllerconfig.IngressConfig) *BackendTrafficPolicyBuilder {
	return &BackendTrafficPolicyBuilder{ingressConfig: ingressConfig}
}

func (b *BackendTrafficPolicyBuilder) Build(isvc *v1beta1.InferenceService) *unstructured.Unstructured {
	headerNames := b.ingressConfig.ConsistentHashHeaders
	if v, ok := isvc.Annotations[constants.IngressConsistentHashHeaders]; ok {
		headerNames = strings.Split(v, ",")
	}
	headers := make([]interface{}, len(headerNames))
	for i, h := range headerNames {
		headers[i] = map[string]interface{}{"name": strings.TrimSpace(h)}
	}

	policy := &unstructured.Unstructured{}
	policy.SetGroupVersionKind(BackendTrafficPolicyGVK)
	policy.SetName(isvc.Name)
	policy.SetNamespace(isvc.Namespace)
	policy.SetLabels(map[string]string{
		"app.kubernetes.io/part-of":    isvc.Name,
		"app.kubernetes.io/managed-by": "ome-controller",
	})
	policy.Object["spec"] = map[string]interface{}{
		// Merge into the Gateway-level parent BTP so this route inherits its
		// settings (e.g. circuitBreaker, timeout) instead of shadowing them.
		"mergeType": "StrategicMerge",
		"targetRefs": []interface{}{
			map[string]interface{}{
				"group": "gateway.networking.k8s.io",
				"kind":  "HTTPRoute",
				"name":  isvc.Name,
			},
		},
		"loadBalancer": map[string]interface{}{
			"type": "ConsistentHash",
			"consistentHash": map[string]interface{}{
				"type":    "Headers",
				"headers": headers,
			},
		},
	}
	return policy
}
