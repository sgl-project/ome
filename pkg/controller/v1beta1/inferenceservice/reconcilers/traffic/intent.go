// Package traffic translates an InferenceService's traffic policy into
// gateway-specific routing resources.
//
// The translator reads the operator's intent (TrafficSpec typed core +
// ome.io/* annotation extension) from an InferenceService, resolves it
// into a ResolvedIntent struct, and hands that to a per-Gateway-
// implementation translator. The translator emits one backend policy
// resource (e.g. Envoy Gateway BackendTrafficPolicy) targeting every
// OME-managed HTTPRoute for the ISVC.
//
// This file defines the resolved-intent data model and the resolve
// function. The Translator interface lives in interfaces/translator.go.
// Translator implementations live under translators/.
package traffic

import (
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// ResolvedIntent is the per-ISVC traffic-management intent after
// merging the typed core (spec.traffic.*) and the ome.io/* annotation
// extension. Translators receive a ResolvedIntent and decide how to
// represent it in their implementation's policy CRD.
//
// An intent is "empty" (HasIntent() == false) when the operator has
// not declared any traffic-management knobs on the InferenceService.
// In that case the reconciler skips translation entirely — no backend
// policy resource is emitted.
type ResolvedIntent struct {
	// Traffic is the typed core from spec.traffic. Nil when unset.
	// Note: this is a pointer to the live TrafficSpec on the ISVC;
	// translators must treat it as read-only.
	Traffic *v1beta1.TrafficSpec

	// CircuitBreaker holds the resolved circuit-breaker knobs from
	// the ome.io/circuit-breaker-* annotation extension. Nil when
	// none are set.
	CircuitBreaker *CircuitBreakerIntent

	// Retry holds the resolved retry knobs from ome.io/retry-*.
	// Nil when none are set.
	Retry *RetryIntent

	// Timeout holds the resolved sub-second / connection timeouts
	// from ome.io/timeout-*. The request-level timeout stays on
	// the per-Component ComponentExtensionSpec.TimeoutSeconds and is
	// applied by the HTTPRoute builder, not by the translator. Nil
	// when no annotations are set.
	Timeout *TimeoutIntent

	// PassthroughEnvoyGateway is the set of raw
	// ome.io/btp.<path>=<value> annotations, with the prefix
	// stripped. The Envoy Gateway translator stitches these
	// verbatim into the emitted BackendTrafficPolicy. Other
	// translators surface them as UnsupportedField.
	//
	// Map key is the dotted path (e.g. "loadBalancer.slowStart.window")
	// and value is the raw annotation value (e.g. "30s"). Always
	// non-nil but possibly empty.
	PassthroughEnvoyGateway map[string]string

	// PassthroughIstio is the equivalent for ome.io/dr.<path>.
	// Always non-nil but possibly empty.
	PassthroughIstio map[string]string
}

// CircuitBreakerIntent holds resolved circuit-breaker values. Each
// field is a pointer so the translator can distinguish "operator
// asked for this value" from "operator did not set this knob".
type CircuitBreakerIntent struct {
	MaxConnections            *int32
	MaxParallelRequests       *int32
	MaxPendingRequests        *int32
	MaxParallelRetries        *int32
	PerEndpointMaxConnections *int32
}

// RetryIntent holds resolved retry-related values. RetryOn is the
// parsed (and validated, by the webhook) list of retry-condition
// tokens.
type RetryIntent struct {
	Attempts      *int32
	RetryOn       []string
	PerTryTimeout *time.Duration
}

// TimeoutIntent holds resolved non-request-level timeout values.
type TimeoutIntent struct {
	Idle                  *time.Duration
	MaxConnectionDuration *time.Duration
	TCPConnect            *time.Duration
}

// HasIntent reports whether the operator has declared any
// traffic-management intent on the ISVC. Translators should not be
// invoked when this returns false — the reconciler skips emission
// entirely in that case (the per-mode reconciler may still populate
// status.components[*].traffic[] for rollouts, but that's HTTPRoute-
// builder territory, not translator territory).
func (r *ResolvedIntent) HasIntent() bool {
	if r == nil {
		return false
	}
	if r.Traffic != nil && (r.Traffic.Algorithm != nil ||
		r.Traffic.ConsistentHash != nil ||
		r.Traffic.EndpointOverride != nil) {
		return true
	}
	if r.CircuitBreaker != nil {
		return true
	}
	if r.Retry != nil {
		return true
	}
	if r.Timeout != nil {
		return true
	}
	if len(r.PassthroughEnvoyGateway) > 0 || len(r.PassthroughIstio) > 0 {
		return true
	}
	return false
}

// Resolve produces a ResolvedIntent from an InferenceService spec.
// It merges the typed core with the ome.io/* annotation extension and
// extracts pass-through namespaces into their dedicated maps.
//
// Values that fail to parse are silently skipped — the validating
// webhook (pkg/validation/traffic_annotations.go) rejects malformed
// values at admission time, so a Resolve call here on a valid ISVC
// will not lose data.
func Resolve(isvc *v1beta1.InferenceService) *ResolvedIntent {
	intent := &ResolvedIntent{
		PassthroughEnvoyGateway: map[string]string{},
		PassthroughIstio:        map[string]string{},
	}
	if isvc.Spec.Traffic != nil {
		intent.Traffic = isvc.Spec.Traffic
	}

	cb := resolveCircuitBreaker(isvc.Annotations)
	if cb != nil {
		intent.CircuitBreaker = cb
	}
	retry := resolveRetry(isvc.Annotations)
	if retry != nil {
		intent.Retry = retry
	}
	timeout := resolveTimeout(isvc.Annotations)
	if timeout != nil {
		intent.Timeout = timeout
	}

	for k, v := range isvc.Annotations {
		if path, ok := strings.CutPrefix(k, constants.PassthroughEnvoyGatewayPrefix); ok {
			intent.PassthroughEnvoyGateway[path] = v
			continue
		}
		if path, ok := strings.CutPrefix(k, constants.PassthroughIstioPrefix); ok {
			intent.PassthroughIstio[path] = v
		}
	}

	return intent
}

func resolveCircuitBreaker(annotations map[string]string) *CircuitBreakerIntent {
	if annotations == nil {
		return nil
	}
	keys := []string{
		constants.CircuitBreakerMaxConnectionsAnnotation,
		constants.CircuitBreakerMaxParallelRequestsAnnotation,
		constants.CircuitBreakerMaxPendingRequestsAnnotation,
		constants.CircuitBreakerMaxParallelRetriesAnnotation,
		constants.CircuitBreakerPerEndpointMaxConnectionsAnnotation,
	}
	if !anyKeySet(annotations, keys) {
		return nil
	}
	out := &CircuitBreakerIntent{}
	out.MaxConnections = parseInt32(annotations[constants.CircuitBreakerMaxConnectionsAnnotation])
	out.MaxParallelRequests = parseInt32(annotations[constants.CircuitBreakerMaxParallelRequestsAnnotation])
	out.MaxPendingRequests = parseInt32(annotations[constants.CircuitBreakerMaxPendingRequestsAnnotation])
	out.MaxParallelRetries = parseInt32(annotations[constants.CircuitBreakerMaxParallelRetriesAnnotation])
	out.PerEndpointMaxConnections = parseInt32(annotations[constants.CircuitBreakerPerEndpointMaxConnectionsAnnotation])
	return out
}

func resolveRetry(annotations map[string]string) *RetryIntent {
	if annotations == nil {
		return nil
	}
	keys := []string{
		constants.RetryAttemptsAnnotation,
		constants.RetryOnAnnotation,
		constants.RetryPerTryTimeoutAnnotation,
	}
	if !anyKeySet(annotations, keys) {
		return nil
	}
	out := &RetryIntent{
		Attempts:      parseInt32(annotations[constants.RetryAttemptsAnnotation]),
		PerTryTimeout: parseDuration(annotations[constants.RetryPerTryTimeoutAnnotation]),
	}
	if raw, ok := annotations[constants.RetryOnAnnotation]; ok && raw != "" {
		for _, tok := range strings.Split(raw, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				out.RetryOn = append(out.RetryOn, tok)
			}
		}
	}
	return out
}

func resolveTimeout(annotations map[string]string) *TimeoutIntent {
	if annotations == nil {
		return nil
	}
	keys := []string{
		constants.TimeoutIdleAnnotation,
		constants.TimeoutMaxConnectionDurationAnnotation,
		constants.TimeoutTCPConnectAnnotation,
	}
	if !anyKeySet(annotations, keys) {
		return nil
	}
	return &TimeoutIntent{
		Idle:                  parseDuration(annotations[constants.TimeoutIdleAnnotation]),
		MaxConnectionDuration: parseDuration(annotations[constants.TimeoutMaxConnectionDurationAnnotation]),
		TCPConnect:            parseDuration(annotations[constants.TimeoutTCPConnectAnnotation]),
	}
}

func anyKeySet(annotations map[string]string, keys []string) bool {
	for _, k := range keys {
		if _, ok := annotations[k]; ok {
			return true
		}
	}
	return false
}

func parseInt32(value string) *int32 {
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	out := int32(n)
	return &out
}

func parseDuration(value string) *time.Duration {
	if value == "" {
		return nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return nil
	}
	return &d
}
