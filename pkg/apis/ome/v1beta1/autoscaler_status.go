package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoscalerManagedBy identifies who owns the active autoscaler for a
// Component. Mirrored onto status.components.<comp>.autoscaler.managedBy
// by the ISVC status writer so operators can tell whether OME is
// reconciling the HPA / ScaledObject themselves or whether an external
// operator is driving scale via the published scaleTargetRef.
//
// none and external are STATUS-FIELD TWINS — they share the same
// dispatch branch (delete any OME-managed HPA / SO and stay out) and
// are distinguished ONLY here in the ManagedBy field. The status writer
// inspects the resolved Class to emit "external" when the user opted
// into a non-OME scaler vs "none" when scaling is disabled entirely.
//
// +kubebuilder:validation:Enum=ome;external;none
type AutoscalerManagedBy string

const (
	// AutoscalerManagedByOME means OME emitted the HPA / ScaledObject
	// for this Component and reconciles it on every pass. Conditions /
	// CurrentReplicas / DesiredReplicas mirror the live scaler.
	AutoscalerManagedByOME AutoscalerManagedBy = "ome"

	// AutoscalerManagedByExternal means an operator-owned scaler drives
	// the Component. OME publishes the canonical scaleTargetRef on
	// status so the external scaler can target it, and stays out
	// otherwise. Conditions / replica counters are empty / zero — OME
	// has no way to read the external scaler's state.
	AutoscalerManagedByExternal AutoscalerManagedBy = "external"

	// AutoscalerManagedByNone means no OME-managed scaler is active for this
	// Component. Conditions empty; replica counters zero.
	AutoscalerManagedByNone AutoscalerManagedBy = "none"
)

// ComponentAutoscalerStatus reports per-Component autoscaler state on
// the InferenceService. Mirrors the resolved ComponentAutoscaler block
// plus the live HPA / ScaledObject metrics so an operator can read the
// authoritative scaler shape, who owns it, and what it's currently
// doing — all from the parent ISVC without chasing IRs and scalers
// separately.
//
// Populated by the ISVC status writer for RawDeployment- and
// OMENative-managed Components.
// The writer degrades gracefully when a Component has no live scaler —
// ManagedBy reports "none" and counters stay zero without crashing.
//
// Alpha. The shape may change without notice.
type ComponentAutoscalerStatus struct {
	// Class echoes the resolved autoscaler class as picked by the
	// Component's deployment-mode resolver. One of HPA | KEDA | External | None.
	// Mirrors the input that drove dispatch so the operator can confirm
	// the inheritance chain landed on the expected class.
	// +optional
	Class AutoscalerClass `json:"class,omitempty"`

	// ManagedBy reports who owns the underlying scaler object.
	// Distinguished from Class because none and external are
	// dispatch-equivalent (both => delete any OME-managed HPA / SO);
	// the surface-level distinction lives here.
	//   - "ome": OME emitted the HPA / ScaledObject; conditions and
	//     replica counters below mirror its live status.
	//   - "external": operator-owned scaler; OME publishes
	//     scaleTargetRef and stays out. Conditions empty.
	//   - "none": no OME-managed scaler is active. Conditions empty;
	//     counters zero.
	// +kubebuilder:validation:Enum=ome;external;none
	// +optional
	ManagedBy AutoscalerManagedBy `json:"managedBy,omitempty"`

	// SpecSource reports which layer of the ISVC -> policy -> runtime ->
	// legacy -> default inheritance chain produced the resolved Autoscaler
	// block. One of "isvc" | "policy" | "runtime" | "legacy" | "default".
	// Mirrors autoscaler.SpecSource so the operator can debug which layer is
	// contributing the live config.
	// +optional
	SpecSource string `json:"specSource,omitempty"`

	// Policy identifies the AutoscalerPolicy that rendered the live block.
	// Set only when SpecSource is "policy".
	// +optional
	Policy *AutoscalerPolicyProvenance `json:"policy,omitempty"`

	// ShadowedPolicyRef reports a policy ref outranked by an inline block
	// (SpecSource "isvc" with spec.<component>.autoscalerPolicyRef set). Its
	// wouldRenderDigest is the in-cluster preview of what the policy would
	// produce if the inline block were removed.
	// +optional
	ShadowedPolicyRef *ShadowedAutoscalerPolicy `json:"shadowedPolicyRef,omitempty"`

	// CurrentReplicas is mirrored from the HPA / ScaledObject status.
	// Zero when ManagedBy != "ome" or when the scaler has not yet
	// reported.
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is mirrored from the HPA / ScaledObject status.
	// Zero when ManagedBy != "ome" or when the scaler has not yet
	// reported.
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// LastScaleTime mirrors the HPA / ScaledObject's last scale event.
	// Nil when ManagedBy != "ome", when the scaler has not yet
	// reported, or when no scaling has occurred since the scaler was
	// created.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// Conditions mirrors the HPA's or ScaledObject's conditions
	// verbatim (HPA: ScalingActive / AbleToScale / ScalingLimited;
	// ScaledObject: Ready / Active / Fallback / Paused). OME adds its
	// own conditions next to them: AutoscalerSwitched when the
	// Component moves between an OME HPA and a KEDA ScaledObject, and
	// AutoscalerResolved when it carries an autoscalerPolicyRef. The
	// scaler conditions are empty when ManagedBy != "ome" —
	// operator-owned scalers are surfaced via scaleTargetRef instead.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AutoscalerSwitchedCondition reports, per Component, whether OME finished
// moving the Component between its own HPA and a KEDA ScaledObject.
//
// Moving to KEDA: OME creates a ScaledObject that names the HPA it already
// runs, and KEDA takes that HPA over in place, keeping the same object.
// The condition turns True once the ScaledObject controls the HPA and KEDA
// reports the ScaledObject Ready. It stays while the ScaledObject keeps
// naming that HPA.
//
// Moving back to an HPA: OME deletes the ScaledObject and the HPAs KEDA
// created for it, waits until they are gone, and then creates its own HPA.
// The condition turns True once nothing KEDA created remains.
const (
	AutoscalerSwitchedCondition = "AutoscalerSwitched"

	// AutoscalerSwitchedReasonHPAControlledByKEDA: KEDA controls the HPA
	// through the ScaledObject and reports the ScaledObject Ready.
	AutoscalerSwitchedReasonHPAControlledByKEDA = "HPAControlledByKEDA"
	// AutoscalerSwitchedReasonScaledObjectRemoved: the ScaledObject and its
	// HPAs are gone and OME runs its own HPA.
	AutoscalerSwitchedReasonScaledObjectRemoved = "ScaledObjectRemoved"
	// AutoscalerSwitchedReasonWaitingForKEDA: KEDA has not taken the HPA
	// over yet.
	AutoscalerSwitchedReasonWaitingForKEDA = "WaitingForKEDA"
	// AutoscalerSwitchedReasonScaledObjectNotReady: KEDA controls the HPA
	// but does not report the ScaledObject Ready.
	AutoscalerSwitchedReasonScaledObjectNotReady = "ScaledObjectNotReady"
	// AutoscalerSwitchedReasonWaitingForScaledObjectDeletion: the
	// ScaledObject still exists, usually while KEDA finishes deleting it.
	AutoscalerSwitchedReasonWaitingForScaledObjectDeletion = "WaitingForScaledObjectDeletion"
	// AutoscalerSwitchedReasonWaitingForHPADeletion: an HPA that KEDA
	// created still exists.
	AutoscalerSwitchedReasonWaitingForHPADeletion = "WaitingForHPADeletion"
)

// ScaleTargetRef is a stripped-down reference to the canonical scale
// target for a Component — the InferenceReplica (for OMENative) or the
// underlying Deployment (for RawDeployment). Published on
// status.components.<comp>.scaleTargetRef so an external scaler knows
// which object to point at (the same GroupKind the user would target
// with kubectl scale <kind>/<name>).
//
// Published for every reconciled RawDeployment or OMENative Component,
// including when no OME-managed scaler is active.
//
// Alpha. The shape may change without notice.
type ScaleTargetRef struct {
	// APIVersion of the scale target — e.g. "ome.io/v1beta1" for the
	// IR-managed OMENative path; "apps/v1" for RawDeployment.
	APIVersion string `json:"apiVersion"`

	// Kind of the scale target — e.g. "InferenceReplica" for the
	// IR-managed OMENative path; "Deployment" for RawDeployment.
	Kind string `json:"kind"`

	// Name of the scale target. Namespace is always the same as the
	// owning ISVC.
	Name string `json:"name"`
}
