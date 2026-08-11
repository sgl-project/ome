package traffic

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestResolve_Empty(t *testing.T) {
	isvc := &v1beta1.InferenceService{}
	got := Resolve(isvc)
	if got == nil {
		t.Fatalf("Resolve must always return a non-nil ResolvedIntent")
	}
	if got.HasIntent() {
		t.Fatalf("empty ISVC must produce no intent: %#v", got)
	}
	// Pass-through maps must be initialized (non-nil) even when empty,
	// so translators can range over them without nil checks.
	if got.PassthroughEnvoyGateway == nil {
		t.Fatalf("PassthroughEnvoyGateway map must be non-nil")
	}
	if got.PassthroughIstio == nil {
		t.Fatalf("PassthroughIstio map must be non-nil")
	}
}

func TestResolve_TypedTrafficOnly(t *testing.T) {
	alg := v1beta1.LoadBalancingTypeConsistentHash
	isvc := &v1beta1.InferenceService{
		Spec: v1beta1.InferenceServiceSpec{
			Traffic: &v1beta1.TrafficSpec{
				Algorithm: &alg,
				ConsistentHash: &v1beta1.ConsistentHashSpec{
					Type:    v1beta1.HashTypeHeader,
					Headers: []v1beta1.HashHeader{{Name: "X-Session-ID"}},
				},
			},
		},
	}
	got := Resolve(isvc)
	if !got.HasIntent() {
		t.Fatalf("typed traffic must count as intent: %#v", got)
	}
	if got.Traffic == nil || got.Traffic.Algorithm == nil || *got.Traffic.Algorithm != v1beta1.LoadBalancingTypeConsistentHash {
		t.Fatalf("typed traffic not propagated: %#v", got.Traffic)
	}
	if got.CircuitBreaker != nil || got.Retry != nil || got.Timeout != nil {
		t.Fatalf("annotation extension fields must be nil when no annotations set: %#v", got)
	}
}

func TestResolve_CircuitBreakerAnnotations(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.CircuitBreakerMaxConnectionsAnnotation:            "1024",
				constants.CircuitBreakerPerEndpointMaxConnectionsAnnotation: "1",
			},
		},
	}
	got := Resolve(isvc)
	if got.CircuitBreaker == nil {
		t.Fatalf("expected CircuitBreaker populated")
	}
	if got.CircuitBreaker.MaxConnections == nil || *got.CircuitBreaker.MaxConnections != 1024 {
		t.Fatalf("MaxConnections wrong: %v", got.CircuitBreaker.MaxConnections)
	}
	if got.CircuitBreaker.PerEndpointMaxConnections == nil || *got.CircuitBreaker.PerEndpointMaxConnections != 1 {
		t.Fatalf("PerEndpointMaxConnections wrong: %v", got.CircuitBreaker.PerEndpointMaxConnections)
	}
	// Unset knobs must be nil.
	if got.CircuitBreaker.MaxParallelRequests != nil {
		t.Fatalf("MaxParallelRequests must be nil when annotation absent, got: %v", got.CircuitBreaker.MaxParallelRequests)
	}
}

func TestResolve_RetryAnnotations(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.RetryAttemptsAnnotation:      "3",
				constants.RetryOnAnnotation:            "5xx,reset,gateway-error",
				constants.RetryPerTryTimeoutAnnotation: "30s",
			},
		},
	}
	got := Resolve(isvc)
	if got.Retry == nil {
		t.Fatalf("expected Retry populated")
	}
	if got.Retry.Attempts == nil || *got.Retry.Attempts != 3 {
		t.Fatalf("Attempts wrong: %v", got.Retry.Attempts)
	}
	wantOn := []string{"5xx", "reset", "gateway-error"}
	if !reflect.DeepEqual(got.Retry.RetryOn, wantOn) {
		t.Fatalf("RetryOn wrong: got %v want %v", got.Retry.RetryOn, wantOn)
	}
	if got.Retry.PerTryTimeout == nil || *got.Retry.PerTryTimeout != 30*time.Second {
		t.Fatalf("PerTryTimeout wrong: %v", got.Retry.PerTryTimeout)
	}
}

func TestResolve_RetryOn_TrimsAndSkipsEmpty(t *testing.T) {
	// The webhook rejects empty-token retry-on lists, but Resolve is
	// defensive: empty tokens are skipped so a degenerate annotation
	// doesn't produce a malformed RetryOn slice.
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.RetryAttemptsAnnotation: "1",
				constants.RetryOnAnnotation:       " 5xx , , reset  ",
			},
		},
	}
	got := Resolve(isvc)
	wantOn := []string{"5xx", "reset"}
	if !reflect.DeepEqual(got.Retry.RetryOn, wantOn) {
		t.Fatalf("RetryOn whitespace/empty handling wrong: got %v want %v", got.Retry.RetryOn, wantOn)
	}
}

func TestResolve_TimeoutAnnotations(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.TimeoutIdleAnnotation:                  "60s",
				constants.TimeoutMaxConnectionDurationAnnotation: "5m",
				constants.TimeoutTCPConnectAnnotation:            "5s",
			},
		},
	}
	got := Resolve(isvc)
	if got.Timeout == nil {
		t.Fatalf("expected Timeout populated")
	}
	if got.Timeout.Idle == nil || *got.Timeout.Idle != 60*time.Second {
		t.Fatalf("Idle wrong: %v", got.Timeout.Idle)
	}
	if got.Timeout.MaxConnectionDuration == nil || *got.Timeout.MaxConnectionDuration != 5*time.Minute {
		t.Fatalf("MaxConnectionDuration wrong: %v", got.Timeout.MaxConnectionDuration)
	}
	if got.Timeout.TCPConnect == nil || *got.Timeout.TCPConnect != 5*time.Second {
		t.Fatalf("TCPConnect wrong: %v", got.Timeout.TCPConnect)
	}
}

func TestResolve_PassthroughNamespaces(t *testing.T) {
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart.window":               "30s",
				constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart.aggression":           "1.5",
				constants.PassthroughIstioPrefix + "trafficPolicy.connectionPool.tcp.tcpKeepalive.time": "30s",
				// Foreign-namespace annotations must be ignored.
				"foo.example.com/anything": "ignored",
			},
		},
	}
	got := Resolve(isvc)
	if len(got.PassthroughEnvoyGateway) != 2 {
		t.Fatalf("expected 2 envoy passthroughs, got %d: %v", len(got.PassthroughEnvoyGateway), got.PassthroughEnvoyGateway)
	}
	if got.PassthroughEnvoyGateway["loadBalancer.slowStart.window"] != "30s" {
		t.Fatalf("envoy passthrough path stripping wrong: %v", got.PassthroughEnvoyGateway)
	}
	if got.PassthroughIstio["trafficPolicy.connectionPool.tcp.tcpKeepalive.time"] != "30s" {
		t.Fatalf("istio passthrough not captured: %v", got.PassthroughIstio)
	}
}

func TestResolve_PassthroughCountsAsIntent(t *testing.T) {
	// HasIntent must report true when pass-through annotations are
	// the only thing set, otherwise the translator never runs for
	// pass-through-only operators.
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.PassthroughEnvoyGatewayPrefix + "loadBalancer.slowStart.window": "30s",
			},
		},
	}
	got := Resolve(isvc)
	if !got.HasIntent() {
		t.Fatalf("pass-through-only intent must count as HasIntent: %#v", got)
	}
}

func TestResolve_MalformedAnnotationsSilentlySkipped(t *testing.T) {
	// The webhook rejects these at admission, so production never
	// hits this path. Resolve is defensive: an unparsable value
	// produces nil rather than a panic. This protects controllers
	// that observe the API in a half-applied state (e.g. just after
	// CRD upgrade) from crash-looping.
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.CircuitBreakerMaxConnectionsAnnotation: "not-a-number",
				constants.TimeoutIdleAnnotation:                  "not-a-duration",
			},
		},
	}
	got := Resolve(isvc)
	// Annotations are present so the parent struct is created, but
	// the malformed fields are nil.
	if got.CircuitBreaker == nil {
		t.Fatalf("CircuitBreaker should be allocated when any cb-* key is present")
	}
	if got.CircuitBreaker.MaxConnections != nil {
		t.Fatalf("malformed int must yield nil, got: %v", got.CircuitBreaker.MaxConnections)
	}
	if got.Timeout == nil {
		t.Fatalf("Timeout should be allocated when any timeout-* key is present")
	}
	if got.Timeout.Idle != nil {
		t.Fatalf("malformed duration must yield nil, got: %v", got.Timeout.Idle)
	}
}

func TestHasIntent_Cases(t *testing.T) {
	alg := v1beta1.LoadBalancingTypeRoundRobin
	cases := []struct {
		name string
		in   *ResolvedIntent
		want bool
	}{
		{"nil", nil, false},
		{"zero", &ResolvedIntent{}, false},
		{
			"traffic typed",
			&ResolvedIntent{Traffic: &v1beta1.TrafficSpec{Algorithm: &alg}},
			true,
		},
		{
			"empty traffic struct does not count",
			&ResolvedIntent{Traffic: &v1beta1.TrafficSpec{}},
			false,
		},
		{"circuit breaker", &ResolvedIntent{CircuitBreaker: &CircuitBreakerIntent{}}, true},
		{"retry", &ResolvedIntent{Retry: &RetryIntent{}}, true},
		{"timeout", &ResolvedIntent{Timeout: &TimeoutIntent{}}, true},
		{
			"envoy passthrough",
			&ResolvedIntent{PassthroughEnvoyGateway: map[string]string{"a": "b"}},
			true,
		},
		{
			"istio passthrough",
			&ResolvedIntent{PassthroughIstio: map[string]string{"a": "b"}},
			true,
		},
		{
			"empty passthrough maps don't count",
			&ResolvedIntent{
				PassthroughEnvoyGateway: map[string]string{},
				PassthroughIstio:        map[string]string{},
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.HasIntent(); got != tc.want {
				t.Fatalf("HasIntent() = %v, want %v", got, tc.want)
			}
		})
	}
}
