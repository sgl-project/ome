package traffic

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// classifyStub is a lightweight Translator that implements only the
// classification surface ComputeUnsupportedAnnotations cares about.
// Translate / ObserveAcceptance / Watches panic if called — they MUST
// NOT be invoked by the classifier.
type classifyStub struct {
	name     string
	supports sets.Set[string]
	fields   sets.Set[string]
	prefixes []string
}

func (c *classifyStub) Name() string                           { return c.name }
func (c *classifyStub) SupportedAnnotations() sets.Set[string] { return c.supports }
func (c *classifyStub) SupportedPassthroughPrefixes() []string { return c.prefixes }
func (c *classifyStub) SupportedTrafficFields() sets.Set[string] {
	if c.fields == nil {
		return sets.New[string]()
	}
	return c.fields
}
func (c *classifyStub) Watches() client.Object { panic("Watches not relevant") }
func (c *classifyStub) Translate(
	_ *v1beta1.InferenceService, _ []string, _ *ResolvedIntent,
) (client.Object, []string, error) {
	panic("Translate not relevant")
}
func (c *classifyStub) ObserveAcceptance(_ client.Object) AcceptanceObservation {
	panic("ObserveAcceptance not relevant")
}

func TestComputeUnsupportedAnnotations_AllSupported_ReturnsNil(t *testing.T) {
	tr := &classifyStub{
		name: "envoy-gateway",
		supports: sets.New(
			constants.CircuitBreakerMaxConnectionsAnnotation,
			constants.RetryAttemptsAnnotation,
		),
		prefixes: []string{constants.PassthroughEnvoyGatewayPrefix},
	}
	in := map[string]string{
		constants.CircuitBreakerMaxConnectionsAnnotation:                   "100",
		constants.RetryAttemptsAnnotation:                                  "3",
		constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart": "30s",
		// Foreign-namespace annotations are ignored entirely.
		"foo.example.com/anything": "ignored",
	}
	if got := ComputeUnsupportedAnnotations(in, tr); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestComputeUnsupportedAnnotations_WrongPassthroughPrefix(t *testing.T) {
	// EG-active cluster, operator set an Istio pass-through.
	tr := &classifyStub{
		name:     "envoy-gateway",
		supports: sets.New(constants.RetryAttemptsAnnotation),
		prefixes: []string{constants.PassthroughEnvoyGatewayPrefix},
	}
	in := map[string]string{
		constants.PassthroughIstioPrefix + "trafficPolicy.connectionPool.tcp.tcpKeepalive.time": "30s",
		constants.RetryAttemptsAnnotation: "1",
	}
	got := ComputeUnsupportedAnnotations(in, tr)
	want := []string{constants.PassthroughIstioPrefix + "trafficPolicy.connectionPool.tcp.tcpKeepalive.time"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeUnsupportedAnnotations_UnsupportedPerKey(t *testing.T) {
	// EG translator that doesn't yet support per-endpoint cap (subset
	// translator). Operator set it -> unsupported.
	tr := &classifyStub{
		name: "alpha-eg",
		supports: sets.New(
			constants.CircuitBreakerMaxConnectionsAnnotation,
			// MaxParallelRequests intentionally NOT supported.
		),
		prefixes: []string{constants.PassthroughEnvoyGatewayPrefix},
	}
	in := map[string]string{
		constants.CircuitBreakerMaxConnectionsAnnotation:      "100",
		constants.CircuitBreakerMaxParallelRequestsAnnotation: "200",
	}
	got := ComputeUnsupportedAnnotations(in, tr)
	want := []string{constants.CircuitBreakerMaxParallelRequestsAnnotation}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeUnsupportedAnnotations_NoopTranslator_AllBackendUnsupported(t *testing.T) {
	// Noop supports nothing — every backend key on the ISVC ends up in
	// the list. Pass-throughs of both kinds also unsupported.
	tr := &classifyStub{name: "noop", supports: sets.New[string](), prefixes: nil}
	in := map[string]string{
		constants.CircuitBreakerMaxConnectionsAnnotation:                                "1",
		constants.RetryAttemptsAnnotation:                                               "1",
		constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart.window":       "30s",
		constants.PassthroughIstioPrefix + "trafficPolicy.connectionPool.tcp.connectTo": "30s",
	}
	got := ComputeUnsupportedAnnotations(in, tr)
	want := []string{
		constants.CircuitBreakerMaxConnectionsAnnotation,
		constants.RetryAttemptsAnnotation,
		constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart.window",
		constants.PassthroughIstioPrefix + "trafficPolicy.connectionPool.tcp.connectTo",
	}
	// Ordering: sorted lexicographically by ComputeUnsupportedAnnotations.
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want %q to be in unsupported list, got: %v", w, got)
		}
	}
	// Verify sorted.
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("output not sorted: %v", got)
		}
	}
}

func TestComputeUnsupportedAnnotations_OperationalAnnotationsExcluded(t *testing.T) {
	// rollout-promote / managed-by-conflict-acked etc are NOT
	// translator-handled; they MUST NOT appear in the unsupported
	// list even when the translator is noop.
	tr := &classifyStub{name: "noop", supports: sets.New[string](), prefixes: nil}
	in := map[string]string{
		constants.RolloutPromoteAnnotation:         "1",
		constants.RolloutRollbackAnnotation:        "true",
		constants.RolloutReadyTimeoutAnnotation:    "10m",
		constants.RevisionHistoryLimitAnnotation:   "5",
		constants.ManagedByConflictAckedAnnotation: "true",
	}
	if got := ComputeUnsupportedAnnotations(in, tr); got != nil {
		t.Fatalf("operational annotations must be excluded, got: %v", got)
	}
}

func TestComputeUnsupportedAnnotations_NilAnnotations_ReturnsNil(t *testing.T) {
	tr := &classifyStub{name: "noop", supports: sets.New[string]()}
	if got := ComputeUnsupportedAnnotations(nil, tr); got != nil {
		t.Fatalf("nil annotations -> nil, got %v", got)
	}
}

func TestComputeUnsupportedAnnotations_ForeignNamespaceIgnored(t *testing.T) {
	tr := &classifyStub{name: "envoy-gateway", supports: sets.New[string]()}
	in := map[string]string{
		"foo.example.com/whatever":          "ignored",
		"app.kubernetes.io/name":            "ignored",
		"prometheus.io/scrape":              "ignored",
		"random.invalid/cb.max-connections": "looks suspicious but not ours",
	}
	if got := ComputeUnsupportedAnnotations(in, tr); got != nil {
		t.Fatalf("foreign annotations must be ignored, got: %v", got)
	}
}

func TestComputeUnsupportedTrafficFields_AllDeclared_ReturnsNil(t *testing.T) {
	tr := &classifyStub{
		name: "envoy-gateway",
		fields: sets.New(
			constants.TrafficCapabilityAlgorithm,
			constants.TrafficCapabilityHashHeader,
			constants.TrafficCapabilityEndpointOverrideHeader,
		),
	}
	alg := v1beta1.LoadBalancingTypeConsistentHash
	spec := &v1beta1.TrafficSpec{
		Algorithm: &alg,
		ConsistentHash: &v1beta1.ConsistentHashSpec{
			Type:    v1beta1.HashTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-tenant"}},
		},
		EndpointOverride: &v1beta1.EndpointOverrideSpec{
			Type:    v1beta1.EndpointOverrideTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-endpoint"}},
		},
	}
	if got := ComputeUnsupportedTrafficFields(spec, tr); got != nil {
		t.Fatalf("all fields declared -> nil, got %v", got)
	}
}

func TestComputeUnsupportedTrafficFields_UndeclaredFieldsSurface(t *testing.T) {
	// DestinationRule-shaped translator: hashes a single header and
	// has no endpoint-override analogue. Both dropped behaviors must
	// surface, sorted.
	tr := &classifyStub{
		name: "istio",
		fields: sets.New(
			constants.TrafficCapabilityAlgorithm,
			constants.TrafficCapabilityHashHeader,
		),
	}
	alg := v1beta1.LoadBalancingTypeConsistentHash
	spec := &v1beta1.TrafficSpec{
		Algorithm: &alg,
		ConsistentHash: &v1beta1.ConsistentHashSpec{
			Type:    v1beta1.HashTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-tenant"}, {Name: "x-session"}},
		},
		EndpointOverride: &v1beta1.EndpointOverrideSpec{
			Type:    v1beta1.EndpointOverrideTypeHeader,
			Headers: []v1beta1.HashHeader{{Name: "x-endpoint"}},
		},
	}
	got := ComputeUnsupportedTrafficFields(spec, tr)
	want := []string{
		constants.TrafficCapabilityHashMultipleHeaders,
		constants.TrafficCapabilityEndpointOverrideHeader,
	}
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	if !reflect.DeepEqual(got, sortedWant) {
		t.Fatalf("got %v, want %v", got, sortedWant)
	}
}

func TestComputeUnsupportedTrafficFields_NilTraffic_ReturnsNil(t *testing.T) {
	tr := &classifyStub{name: "istio", fields: sets.New[string]()}
	if got := ComputeUnsupportedTrafficFields(nil, tr); got != nil {
		t.Fatalf("nil traffic -> nil, got %v", got)
	}
}
