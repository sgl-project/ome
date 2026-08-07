package constants

import (
	"strings"
	"testing"
)

func TestTrafficAnnotations_Prefix(t *testing.T) {
	// All traffic annotation keys must live under the
	// ome.io/ namespace. The webhook's Did-you-mean detection relies
	// on this prefix.
	keys := []struct {
		name  string
		value string
	}{
		// Circuit breaker.
		{"CircuitBreakerMaxConnectionsAnnotation", CircuitBreakerMaxConnectionsAnnotation},
		{"CircuitBreakerMaxParallelRequestsAnnotation", CircuitBreakerMaxParallelRequestsAnnotation},
		{"CircuitBreakerMaxPendingRequestsAnnotation", CircuitBreakerMaxPendingRequestsAnnotation},
		{"CircuitBreakerMaxParallelRetriesAnnotation", CircuitBreakerMaxParallelRetriesAnnotation},
		{"CircuitBreakerPerEndpointMaxConnectionsAnnotation", CircuitBreakerPerEndpointMaxConnectionsAnnotation},
		// Retries.
		{"RetryAttemptsAnnotation", RetryAttemptsAnnotation},
		{"RetryOnAnnotation", RetryOnAnnotation},
		{"RetryPerTryTimeoutAnnotation", RetryPerTryTimeoutAnnotation},
		// Timeouts.
		{"TimeoutIdleAnnotation", TimeoutIdleAnnotation},
		{"TimeoutMaxConnectionDurationAnnotation", TimeoutMaxConnectionDurationAnnotation},
		{"TimeoutTCPConnectAnnotation", TimeoutTCPConnectAnnotation},
		// Operational.
		{"ManagedByConflictAckedAnnotation", ManagedByConflictAckedAnnotation},
		{"RolloutReadyTimeoutAnnotation", RolloutReadyTimeoutAnnotation},
		{"RevisionHistoryLimitAnnotation", RevisionHistoryLimitAnnotation},
		{"RolloutPromoteAnnotation", RolloutPromoteAnnotation},
		{"RolloutRollbackAnnotation", RolloutRollbackAnnotation},
	}
	wantPrefix := OMEAPIGroupName + "/"
	for _, k := range keys {
		if !strings.HasPrefix(k.value, wantPrefix) {
			t.Errorf("%s = %q does not start with %q", k.name, k.value, wantPrefix)
		}
	}
}

func TestTrafficAnnotations_Unique(t *testing.T) {
	// No two keys may collide — otherwise admission Did-you-mean and
	// the parser would behave non-deterministically.
	all := []string{
		CircuitBreakerMaxConnectionsAnnotation,
		CircuitBreakerMaxParallelRequestsAnnotation,
		CircuitBreakerMaxPendingRequestsAnnotation,
		CircuitBreakerMaxParallelRetriesAnnotation,
		CircuitBreakerPerEndpointMaxConnectionsAnnotation,
		RetryAttemptsAnnotation,
		RetryOnAnnotation,
		RetryPerTryTimeoutAnnotation,
		TimeoutIdleAnnotation,
		TimeoutMaxConnectionDurationAnnotation,
		TimeoutTCPConnectAnnotation,
		ManagedByConflictAckedAnnotation,
		RolloutReadyTimeoutAnnotation,
		RevisionHistoryLimitAnnotation,
		RolloutPromoteAnnotation,
		RolloutRollbackAnnotation,
	}
	seen := make(map[string]string, len(all))
	for _, k := range all {
		if prev, ok := seen[k]; ok {
			t.Errorf("duplicate annotation key %q (also: %q)", k, prev)
		}
		seen[k] = k
	}
}

func TestPassthroughPrefixes(t *testing.T) {
	// Pass-through prefixes must end with "." so a value like
	// "ome.io/btp.loadBalancer.slowStart" can be cleanly stripped to
	// "loadBalancer.slowStart". They must not collide with the
	// known-key namespace (otherwise Did-you-mean would false-positive).
	if want := OMEAPIGroupName + "/btp."; PassthroughEnvoyGatewayPrefix != want {
		t.Errorf("PassthroughEnvoyGatewayPrefix = %q, want %q", PassthroughEnvoyGatewayPrefix, want)
	}
	if want := OMEAPIGroupName + "/dr."; PassthroughIstioPrefix != want {
		t.Errorf("PassthroughIstioPrefix = %q, want %q", PassthroughIstioPrefix, want)
	}
	if !strings.HasSuffix(PassthroughEnvoyGatewayPrefix, ".") {
		t.Errorf("PassthroughEnvoyGatewayPrefix must end with .: %q", PassthroughEnvoyGatewayPrefix)
	}
	if !strings.HasSuffix(PassthroughIstioPrefix, ".") {
		t.Errorf("PassthroughIstioPrefix must end with .: %q", PassthroughIstioPrefix)
	}
}

func TestKnownAnnotation_NoCollisionWithPassthrough(t *testing.T) {
	// A known annotation key MUST NOT match a pass-through prefix —
	// otherwise the webhook can't distinguish them when classifying
	// a key as known vs pass-through.
	known := []string{
		CircuitBreakerMaxConnectionsAnnotation,
		CircuitBreakerMaxParallelRequestsAnnotation,
		CircuitBreakerMaxPendingRequestsAnnotation,
		CircuitBreakerMaxParallelRetriesAnnotation,
		CircuitBreakerPerEndpointMaxConnectionsAnnotation,
		RetryAttemptsAnnotation,
		RetryOnAnnotation,
		RetryPerTryTimeoutAnnotation,
		TimeoutIdleAnnotation,
		TimeoutMaxConnectionDurationAnnotation,
		TimeoutTCPConnectAnnotation,
		ManagedByConflictAckedAnnotation,
		RolloutReadyTimeoutAnnotation,
		RevisionHistoryLimitAnnotation,
		RolloutPromoteAnnotation,
		RolloutRollbackAnnotation,
	}
	for _, k := range known {
		if strings.HasPrefix(k, PassthroughEnvoyGatewayPrefix) {
			t.Errorf("known key %q collides with Envoy Gateway passthrough prefix", k)
		}
		if strings.HasPrefix(k, PassthroughIstioPrefix) {
			t.Errorf("known key %q collides with Istio passthrough prefix", k)
		}
	}
}
