package autoscaler

import (
	"context"
	"strings"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/autoscalerpolicy/render"
)

func policyFixture() *v1beta1.AutoscalerPolicy {
	return &v1beta1.AutoscalerPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "request-activity-v1", Namespace: "ns", Generation: 3},
		Spec: v1beta1.AutoscalerPolicySpec{
			Class: v1beta1.AutoscalerKEDA,
			Keda: &v1beta1.KedaPolicyTemplate{
				Triggers: []v1beta1.KedaTriggerTemplate{{
					Type:        "prometheus",
					ProviderRef: &v1beta1.MetricProviderRef{Name: "cluster-prometheus"},
					MetricType:  autoscalingv2.AverageValueMetricType,
					Metadata: map[string]string{
						"threshold":        "1",
						"ignoreNullValues": "false",
						"query":            `((sum({inferenceservice="{{ .ISVCName }}"}) > bool 0) * {{ .MaxReplicas }})`,
					},
				}},
				Fallback: &v1beta1.FallbackTemplate{
					FailureThreshold: 3,
					Replicas:         v1beta1.ReplicaValueSource{FromComponent: ptr.To(v1beta1.BoundsFieldMaxReplicas)},
				},
			},
		},
	}
}

func isvcWithRef(component v1beta1.ComponentType, inline *v1beta1.ComponentAutoscaler) *v1beta1.InferenceService {
	ext := v1beta1.ComponentExtensionSpec{
		MinReplicas:         ptr.To(1),
		MaxReplicas:         8,
		Autoscaler:          inline,
		AutoscalerPolicyRef: &v1beta1.AutoscalerPolicyRef{Name: "request-activity-v1"},
	}
	isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "llm-a", Namespace: "ns"}}
	switch component {
	case v1beta1.EngineComponent:
		isvc.Spec.Engine = &v1beta1.EngineSpec{ComponentExtensionSpec: ext}
	case v1beta1.DecoderComponent:
		isvc.Spec.Decoder = &v1beta1.DecoderSpec{ComponentExtensionSpec: ext}
	case v1beta1.RouterComponent:
		isvc.Spec.Router = &v1beta1.RouterSpec{ComponentExtensionSpec: ext}
	}
	return isvc
}

func testResolver(t *testing.T, objects ...runtime.Object) *PolicyResolver {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &PolicyResolver{
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build(),
		Providers:     render.Providers{"cluster-prometheus": {ServerAddress: "http://prometheus.example.invalid:9090"}},
		Enabled:       true,
		KedaAvailable: true,
	}
}

func TestResolvePolicyRendered(t *testing.T) {
	resolver := testResolver(t, policyFixture())
	isvc := isvcWithRef(v1beta1.EngineComponent, nil)

	outcome, err := resolver.Resolve(context.Background(), isvc, v1beta1.EngineComponent, Bounds{Min: 1, Max: 8})
	if err != nil {
		t.Fatalf("transient error: %v", err)
	}
	if outcome.Hold || outcome.Rendered == nil || outcome.Provenance == nil {
		t.Fatalf("expected rendered outcome, got %+v", outcome)
	}
	if outcome.Provenance.ObservedGeneration != 3 || !strings.HasPrefix(outcome.Provenance.PortableDigest, "pv1:") {
		t.Errorf("provenance = %+v", outcome.Provenance)
	}

	resolved, source, hold := ResolveComponentAutoscalerWithPolicy(nil, isvc, v1beta1.EngineComponent, outcome)
	if hold || source != SpecSourcePolicy || resolved == nil || resolved.Class != v1beta1.AutoscalerKEDA {
		t.Fatalf("resolution = (%v, %v, %v)", resolved, source, hold)
	}
	if !strings.Contains(resolved.Keda.Triggers[0].Metadata["query"], "* 8)") {
		t.Errorf("query not rendered from bounds: %s", resolved.Keda.Triggers[0].Metadata["query"])
	}
}

// The inline block outranks the ref entirely — hold included: the escape
// hatch must work precisely when the policy machinery is broken.
func TestResolveInlineOutranksPolicy(t *testing.T) {
	resolver := testResolver(t) // policy object absent -> hold outcome
	inline := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA}
	isvc := isvcWithRef(v1beta1.EngineComponent, inline)

	outcome, err := resolver.Resolve(context.Background(), isvc, v1beta1.EngineComponent, Bounds{Min: 1, Max: 8})
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Hold || outcome.HoldReason != v1beta1.AutoscalerResolvedReasonPolicyNotFound {
		t.Fatalf("expected PolicyNotFound hold, got %+v", outcome)
	}
	resolved, source, hold := ResolveComponentAutoscalerWithPolicy(nil, isvc, v1beta1.EngineComponent, outcome)
	if hold || source != SpecSourceISVC || resolved.Class != v1beta1.AutoscalerHPA {
		t.Fatalf("inline must win: (%v, %v, %v)", resolved, source, hold)
	}
}

func TestResolveHoldPaths(t *testing.T) {
	cases := []struct {
		name       string
		resolver   func(t *testing.T) *PolicyResolver
		wantReason string
	}{
		{"missing policy", func(t *testing.T) *PolicyResolver { return testResolver(t) },
			v1beta1.AutoscalerResolvedReasonPolicyNotFound},
		{"feature disabled", func(t *testing.T) *PolicyResolver {
			r := testResolver(t, policyFixture())
			r.Enabled = false
			return r
		}, v1beta1.AutoscalerResolvedReasonPolicyNotFound},
		{"provider unbound", func(t *testing.T) *PolicyResolver {
			r := testResolver(t, policyFixture())
			r.Providers = render.Providers{}
			return r
		}, v1beta1.AutoscalerResolvedReasonPolicyInvalid},
		{"keda unavailable", func(t *testing.T) *PolicyResolver {
			r := testResolver(t, policyFixture())
			r.KedaAvailable = false
			return r
		}, v1beta1.AutoscalerResolvedReasonClassUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isvc := isvcWithRef(v1beta1.EngineComponent, nil)
			outcome, err := tc.resolver(t).Resolve(context.Background(), isvc, v1beta1.EngineComponent, Bounds{Min: 1, Max: 8})
			if err != nil {
				t.Fatalf("hold states must not be transient errors: %v", err)
			}
			if !outcome.Hold || outcome.HoldReason != tc.wantReason {
				t.Fatalf("outcome = %+v, want hold %s", outcome, tc.wantReason)
			}
			// The chain must never fall through to runtime/default on a hold.
			resolved, source, hold := ResolveComponentAutoscalerWithPolicy(nil, isvc, v1beta1.EngineComponent, outcome)
			if !hold || resolved != nil || source != SpecSourcePolicy {
				t.Fatalf("hold must short-circuit: (%v, %v, %v)", resolved, source, hold)
			}
		})
	}
}

// A rendered policy outranks the runtime's vendor-default block: with both
// present the chain resolves source=policy and dispatches the render, never
// the runtime autoscaler.
func TestResolvePolicyOutranksRuntime(t *testing.T) {
	resolver := testResolver(t, policyFixture())
	isvc := isvcWithRef(v1beta1.EngineComponent, nil)
	outcome, err := resolver.Resolve(context.Background(), isvc, v1beta1.EngineComponent, Bounds{Min: 1, Max: 8})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Hold || outcome.Rendered == nil {
		t.Fatalf("expected rendered outcome, got %+v", outcome)
	}

	rt := &v1beta1.ServingRuntimeSpec{
		EngineConfig: &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				Autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA},
			},
		},
	}
	resolved, source, hold := ResolveComponentAutoscalerWithPolicy(rt, isvc, v1beta1.EngineComponent, outcome)
	if hold || source != SpecSourcePolicy {
		t.Fatalf("policy must outrank the runtime block: (%v, %v, %v)", resolved, source, hold)
	}
	if resolved == nil || resolved.Class != v1beta1.AutoscalerKEDA {
		t.Fatalf("resolved block must be the render, not the runtime HPA: %+v", resolved)
	}
}

// A ref without a policy layer outcome (no ref set) leaves the ordinary
// chain untouched, and the raw legacy branch still substitutes only the
// default slot below the policy layer.
func TestResolveRawPolicyPrecedence(t *testing.T) {
	resolver := testResolver(t, policyFixture())
	isvc := isvcWithRef(v1beta1.EngineComponent, nil)
	outcome, err := resolver.Resolve(context.Background(), isvc, v1beta1.EngineComponent, Bounds{Min: 1, Max: 8})
	if err != nil {
		t.Fatal(err)
	}
	annotations := map[string]string{"ome.io/autoscalerClass": "hpa"}
	resolved, source, hold, err := ResolveRawComponentAutoscalerWithPolicy(nil, isvc, v1beta1.EngineComponent, annotations, outcome)
	if err != nil || hold {
		t.Fatalf("unexpected: %v %v", err, hold)
	}
	if source != SpecSourcePolicy || resolved.Class != v1beta1.AutoscalerKEDA {
		t.Fatalf("policy must outrank the legacy annotation: (%v, %v)", resolved.Class, source)
	}
}

func TestLastRenderedRoundTrip(t *testing.T) {
	block := &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA, Keda: &v1beta1.KedaAutoscaler{}}
	encoded, err := MarshalLastRendered(block)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := UnmarshalLastRendered(encoded)
	if err != nil || decoded.Class != v1beta1.AutoscalerKEDA {
		t.Fatalf("round trip: %v %v", decoded, err)
	}
	if none, err := UnmarshalLastRendered(""); none != nil || err != nil {
		t.Fatalf("empty annotation must read as no record: %v %v", none, err)
	}
}
