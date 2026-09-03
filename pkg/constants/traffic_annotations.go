// Traffic-management annotation keys.
//
// All annotations live under the OME group prefix (ome.io/). Operators
// set them on the InferenceService metadata; the traffic translator
// reads them and threads the values into the emitted backend policy
// resource.
package constants

// Backend-policy annotation keys.
//
// Long-tail load-balancing knobs that did not graduate to the typed
// core (TrafficSpec) in alpha. Each key is independently validated by
// the admission webhook; unknown keys with the `ome.io/` prefix that
// look like one of these (Levenshtein 2) are rejected with a
// Did-you-mean suggestion.
var (
	// Circuit breaker.
	CircuitBreakerMaxConnectionsAnnotation            = OMEAPIGroupName + "/circuit-breaker-max-connections"
	CircuitBreakerMaxParallelRequestsAnnotation       = OMEAPIGroupName + "/circuit-breaker-max-parallel-requests"
	CircuitBreakerMaxPendingRequestsAnnotation        = OMEAPIGroupName + "/circuit-breaker-max-pending-requests"
	CircuitBreakerMaxParallelRetriesAnnotation        = OMEAPIGroupName + "/circuit-breaker-max-parallel-retries"
	CircuitBreakerPerEndpointMaxConnectionsAnnotation = OMEAPIGroupName + "/circuit-breaker-per-endpoint-max-connections"

	// Retries.
	RetryAttemptsAnnotation      = OMEAPIGroupName + "/retry-attempts"
	RetryOnAnnotation            = OMEAPIGroupName + "/retry-on"
	RetryPerTryTimeoutAnnotation = OMEAPIGroupName + "/retry-per-try-timeout"

	// Connection / sub-second timeouts. Request-level timeout stays on
	// ComponentExtensionSpec.TimeoutSeconds; these cover the rest.
	TimeoutIdleAnnotation                  = OMEAPIGroupName + "/timeout-idle"
	TimeoutMaxConnectionDurationAnnotation = OMEAPIGroupName + "/timeout-max-connection-duration"
	TimeoutTCPConnectAnnotation            = OMEAPIGroupName + "/timeout-tcp-connect"
)

// Operational annotation keys.
var (
	// ManagedByConflictAckedAnnotation, when set to "true" on the
	// InferenceService, acknowledges a hand-authored backend policy
	// with the conflicting name OME would use. Suppresses the GA
	// admission rejection while OME continues to defer to the
	// manually-authored resource.
	ManagedByConflictAckedAnnotation = OMEAPIGroupName + "/managed-by-conflict-acked"

	// RolloutReadyTimeoutAnnotation overrides the default timeout for
	// new-revision pods to reach Ready before the rollout is marked
	// Failed. Duration string (e.g. "15m"). Default: 15m.
	RolloutReadyTimeoutAnnotation = OMEAPIGroupName + "/rollout-ready-timeout"

	// RevisionHistoryLimitAnnotation caps the number of non-live
	// ControllerRevisions retained per Component before garbage
	// collection. Must be a positive integer (rejected at admission
	// otherwise). Live revisions — current, target, and every
	// per-Instance running/target revision — are never deleted
	// regardless of the limit. Overrides the operator-level
	// lifecycle.revisionHistoryLimit value from the
	// inferenceservice-config ConfigMap; when neither is set, no
	// revisions are pruned.
	RevisionHistoryLimitAnnotation = OMEAPIGroupName + "/revision-history-limit"

	// RolloutPromoteAnnotation is the operator verb that advances a paused
	// canary step under Manual promotion (canary with neither auto nor
	// analysis set). Value: the canary revision hash (copied from
	// status.canary.canaryRevisionHash) — the hash guards against promoting
	// a stale revision, and admission rejects any other value. The
	// controller clears the annotation after consuming it. No-op when
	// spec.rollout.groups[].canary is unset.
	RolloutPromoteAnnotation = OMEAPIGroupName + "/rollout-promote"

	// RolloutRollbackAnnotation is the operator verb for instant
	// rollback. Value: "true" ("false" is identical to absence). Traffic
	// shifts to 100% old, new revision drains after scaleDownDelaySeconds.
	// Controller clears the annotation after consuming it.
	RolloutRollbackAnnotation = OMEAPIGroupName + "/rollout-rollback"

	// PausedRolloutAnnotation, set on the InferenceService, holds every
	// coordination group at its current phase (and the canary step
	// machine) and suspends fleet changes until the operator clears it.
	// Two accepted values:
	//   "true"   — pause: no Update / Create / Migration work starts or
	//              advances, but each component's RestartPolicy keeps
	//              repairing existing Instances at their current
	//              revision.
	//   "freeze" — full stop: repair is suspended too; only kubelet
	//              container restarts and deliberate scale-down remain.
	// Status stays truthful in either depth: an Instance that lost every
	// pod stops reporting Ready even though its recovery stays parked.
	// Any other value is treated as not paused.
	PausedRolloutAnnotation = OMEAPIGroupName + "/rollout-paused"

	// PausedRolloutFreezeValue is the PausedRolloutAnnotation value that
	// additionally suspends Instance repair.
	PausedRolloutFreezeValue = "freeze"
)

// RolloutPauseState interprets PausedRolloutAnnotation on the given
// annotations map. paused reports whether any pause is in effect;
// freeze reports the full-stop variant. Unknown values are not paused —
// every consumer must share this parse so a pause registers uniformly
// across the coordination, canary, projection, and workload layers.
func RolloutPauseState(annotations map[string]string) (paused, freeze bool) {
	switch annotations[PausedRolloutAnnotation] {
	case "true":
		return true, false
	case PausedRolloutFreezeValue:
		return true, true
	default:
		return false, false
	}
}

// Pass-through annotation prefixes.
//
// Operators can reach implementation-specific fields OME does not
// type-model by setting annotations under these prefixes. The
// translator stitches the value verbatim into the named path on the
// emitted resource. OME performs no schema validation; the gateway
// controller catches errors and OME surfaces GatewayRejected.
//
// Each prefix is per-translator. Using a prefix whose translator is
// not active surfaces UnsupportedPassthrough at admission.
//
// These are var rather than const because OMEAPIGroupName itself is
// declared as var in this package; same pattern as the existing
// annotation block in constants.go.
var (
	// PassthroughEnvoyGatewayPrefix maps to
	// gateway.envoyproxy.io/v1alpha1.BackendTrafficPolicy.spec.<path>
	// when the Envoy Gateway translator is active.
	PassthroughEnvoyGatewayPrefix = OMEAPIGroupName + "/btp."

	// PassthroughIstioPrefix maps to
	// networking.istio.io/.../DestinationRule.spec.<path>
	// when the Istio translator is active.
	PassthroughIstioPrefix = OMEAPIGroupName + "/dr."
)
