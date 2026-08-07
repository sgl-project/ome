// Package v1beta1 — traffic-management status types.
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrafficStatus reflects the resolved backend traffic policy and the
// emitted policy resource (e.g. Envoy Gateway BackendTrafficPolicy)
// for an InferenceService. Lives at InferenceServiceStatus.Traffic.
// Populated only when traffic intent is declared via spec.traffic or
// any ome.io/* traffic annotation; otherwise nil so older clients see
// nothing.
type TrafficStatus struct {
	// Algorithm reflects the resolved load-balancing algorithm. One
	// of RoundRobin | LeastRequest | Random | ConsistentHash | Default.
	// "Default" means no Algorithm was set on spec.traffic and the
	// Gateway implementation default applies.
	// +optional
	Algorithm string `json:"algorithm,omitempty"`

	// BackendPolicyResource references the OME-emitted backend policy
	// (typically a gateway.envoyproxy.io/v1alpha1.BackendTrafficPolicy).
	// Empty when no policy was emitted (e.g. translator deferred to a
	// conflicting hand-authored policy).
	// +optional
	BackendPolicyResource *BackendPolicyRef `json:"backendPolicyResource,omitempty"`

	// TargetedHTTPRoutes lists the HTTPRoute names the emitted policy
	// targets. Operators read this to understand which routes the
	// policy applies to without inspecting the emitted resource.
	// Empty when no policy was emitted.
	// +optional
	// +listType=atomic
	TargetedHTTPRoutes []string `json:"targetedHTTPRoutes,omitempty"`

	// Conditions surface translation, conflict, and gateway-acceptance
	// state. The well-known condition type is BackendPolicyReady.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// BackendPolicyRef identifies the OME-emitted backend policy resource
// sibling to the InferenceService.
type BackendPolicyRef struct {
	// APIVersion of the resource (e.g. "gateway.envoyproxy.io/v1alpha1").
	APIVersion string `json:"apiVersion"`
	// Kind of the resource (e.g. "BackendTrafficPolicy").
	Kind string `json:"kind"`
	// Name of the resource, in the same namespace as the
	// InferenceService.
	Name string `json:"name"`
}

// Well-known TrafficStatus condition types.
const (
	// TrafficConditionBackendPolicyReady is True when the emitted
	// backend policy resource has been acknowledged by the gateway
	// controller; False with a Reason explaining the situation
	// otherwise; Unknown while emission is in flight.
	TrafficConditionBackendPolicyReady = "BackendPolicyReady"

	// TrafficConditionBackendPolicyUnsupportedFields is added (with
	// Status=True, Reason=UnsupportedField) when the operator
	// declared ome.io/* traffic annotations the active translator
	// does not honor — for example, an ome.io/dr.* (Istio
	// pass-through) annotation on an Envoy-Gateway cluster. The
	// message lists every dropped key so operators see exactly what
	// was ignored without inspecting the policy resource.
	//
	// The condition is omitted entirely when nothing was dropped
	// (positive-polarity convention: absence = nothing to worry
	// about). It is also omitted on noop / TranslationFailed
	// branches, where BackendPolicyReady already explains the
	// situation and listing dropped keys is redundant noise.
	TrafficConditionBackendPolicyUnsupportedFields = "BackendPolicyUnsupportedFields"
)

// Well-known TrafficConditionBackendPolicyReady reasons.
const (
	TrafficReasonAcceptedByGateway     = "AcceptedByGateway"
	TrafficReasonConflictingPolicy     = "ConflictingPolicy"
	TrafficReasonUnsupportedField      = "UnsupportedField"
	TrafficReasonNoTranslatorAvailable = "NoTranslatorAvailable"
	TrafficReasonGatewayRejected       = "GatewayRejected"
	TrafficReasonPending               = "Pending"
	// TrafficReasonTranslationFailed means the active translator
	// errored while turning the resolved intent into a backend policy
	// resource (e.g. an intent shape the implementation can't honor).
	// The condition message carries the translator's error string.
	TrafficReasonTranslationFailed = "TranslationFailed"
)

// RolloutPhase reflects the current rollout state for a Component.
// Operators read it to understand which rollout strategy is active and
// whether it has succeeded. Lives on ComponentStatusSpec.RolloutPhase.
type RolloutPhase string

const (
	// RolloutPhaseStable indicates one revision is live serving 100%
	// of traffic and no rollout is in flight.
	RolloutPhaseStable RolloutPhase = "Stable"
	// RolloutPhaseCanarying indicates two revisions are live, with the
	// canary at 1-99% traffic and canary pods Ready.
	RolloutPhaseCanarying RolloutPhase = "Canarying"
	// RolloutPhaseBlueGreenStandby indicates two revisions are live,
	// with the canary at 0% traffic and canary pods Ready (ready for
	// in-cluster validation before cutover).
	RolloutPhaseBlueGreenStandby RolloutPhase = "BlueGreenStandby"
	// RolloutPhasePending indicates a new revision is being
	// materialized; canary pods are not yet Ready. Transient.
	RolloutPhasePending RolloutPhase = "Pending"
	// RolloutPhasePaused indicates a step-based rollout is paused at
	// a step boundary, waiting for either the
	// ome.io/rollout-promote annotation (Manual policy) or for
	// the step's Pause.Duration to elapse (Auto policy).
	RolloutPhasePaused RolloutPhase = "Paused"
	// RolloutPhasePromoting indicates the canary is scaling up to
	// full replicas and the stable revision is draining. Transient.
	RolloutPhasePromoting RolloutPhase = "Promoting"
	// RolloutPhaseRollingBack indicates the canary is scaling down
	// and the stable revision is scaling back to full. Transient.
	RolloutPhaseRollingBack RolloutPhase = "RollingBack"
	// RolloutPhaseRolledBack indicates a rollback completed: the
	// component is fully back on the stable revision and is HELD there,
	// rejecting the rolled-back revision. The rollout re-arms only when a
	// new (different) target revision appears. Terminal until then.
	RolloutPhaseRolledBack RolloutPhase = "RolledBack"
	// RolloutPhaseFailed indicates the canary pods failed to reach
	// Ready within the configured timeout. The canary is preserved
	// for diagnosis; operator must roll back or recover explicitly.
	RolloutPhaseFailed RolloutPhase = "Failed"
)
