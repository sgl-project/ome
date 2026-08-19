package status

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils"
)

// TopLevelComponentReadyFromLifecycle derives the standard top-level
// component-ready condition (EngineReady / DecoderReady / RouterReady) from
// OMENative counters, honoring the Component's disruption budget. Returns nil for
// unknown Component types so callers leave the condition surface untouched.
//
// Ready is availability-based, not strict: True when the number of serving
// Instances is at or above the Component's availability floor (see
// utils.AvailabilityFloor) — the same MinAvailable/MaxUnavailable that drive the
// Component's PodDisruptionBudget, falling back to its rolling-update
// MaxUnavailable (defaulted 25% for OMENative). This mirrors how RawDeployment
// derives readiness from a Deployment's maxUnavailable-thresholded Available
// condition, so a Component serving 9/10 Instances (within budget) reports Ready
// instead of flapping to NotReady on every single-Instance disruption or
// in-budget rollout. A genuine outage below the floor still reports NotReady.
// The floor is never below 1 for a Component with desired Instances, so a budget
// that permits every Instance to be down cannot report a zero-serving Component
// as Ready.
//
// The reason distinguishes fully-converged ("Ready") from serving-at-or-above-
// floor but not fully rolled out ("MinimumAvailable"), so rollout progress stays
// visible. `ext` may be nil (e.g. a hand-crafted object bypassing the defaulter),
// in which case the floor is strict (all Instances must serve).
//
// Lives in the neutral inferenceservice/status package — the single source of
// truth for the Ready predicate that the IR-managed projector (irprojector
// status.go) calls. The IR's own Ready condition derives the same way via
// utils.AvailabilityFloor.
func TopLevelComponentReadyFromLifecycle(component v1beta1.ComponentType, cs *v1beta1.LifecycleStatus, ext *v1beta1.ComponentExtensionSpec) *apis.Condition {
	var readyType apis.ConditionType
	switch component {
	case v1beta1.EngineComponent:
		readyType = v1beta1.EngineReady
	case v1beta1.DecoderComponent:
		readyType = v1beta1.DecoderReady
	case v1beta1.RouterComponent:
		readyType = v1beta1.RouterReady
	default:
		return nil
	}
	cond := &apis.Condition{Type: readyType}
	if cs == nil || cs.Replicas == 0 {
		cond.Status = corev1.ConditionFalse
		cond.Reason = "NoReplicas"
		cond.Message = "Component has no desired Instances"
		return cond
	}
	floor := utils.AvailabilityFloor(cs.Replicas, extMinAvailable(ext), extMaxUnavailable(ext), rolloutMaxUnavailable(ext))
	// A budget permitting every Instance to be down (MaxUnavailable "100%", or
	// an integer at or above Replicas) yields a zero floor, which would report
	// Ready for a Component serving no traffic. Replicas > 0 here, so require
	// at least one serving Instance.
	if floor < 1 {
		floor = 1
	}
	switch {
	case cs.ReadyReplicas >= cs.Replicas:
		cond.Status = corev1.ConditionTrue
		cond.Reason = "Ready"
		cond.Message = "All Instances are Ready"
	case cs.ServingReplicas >= floor:
		cond.Status = corev1.ConditionTrue
		cond.Reason = "MinimumAvailable"
		cond.Message = fmt.Sprintf("%d/%d Instances serving (min %d)", cs.ServingReplicas, cs.Replicas, floor)
	default:
		cond.Status = corev1.ConditionFalse
		cond.Reason = "InsufficientAvailable"
		cond.Message = fmt.Sprintf("%d/%d Instances serving, need %d", cs.ServingReplicas, cs.Replicas, floor)
	}
	return cond
}

// extMinAvailable / extMaxUnavailable / rolloutMaxUnavailable pull the disruption
// budget off a (possibly nil) ComponentExtensionSpec: the PDB MinAvailable /
// MaxUnavailable, and the rolling-update MaxUnavailable fallback.
func extMinAvailable(ext *v1beta1.ComponentExtensionSpec) *intstr.IntOrString {
	if ext == nil {
		return nil
	}
	return ext.MinAvailable
}

func extMaxUnavailable(ext *v1beta1.ComponentExtensionSpec) *intstr.IntOrString {
	if ext == nil {
		return nil
	}
	return ext.MaxUnavailable
}

func rolloutMaxUnavailable(ext *v1beta1.ComponentExtensionSpec) *intstr.IntOrString {
	if ext == nil || ext.Lifecycle == nil || ext.Lifecycle.UpdateStrategy == nil || ext.Lifecycle.UpdateStrategy.RollingUpdate == nil {
		return nil
	}
	return ext.Lifecycle.UpdateStrategy.RollingUpdate.MaxUnavailable
}
