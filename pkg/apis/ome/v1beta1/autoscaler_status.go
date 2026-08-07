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

	// AutoscalerManagedByNone means no scaler is active for this
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
// Populated by the ISVC status writer for OMENative-managed Components.
// The writer degrades gracefully when a Component has no live scaler —
// ManagedBy reports "none" and counters stay zero without crashing.
//
// Alpha. The shape may change without notice.
type ComponentAutoscalerStatus struct {
	// Class echoes the resolved autoscaler class as picked by
	// ResolveComponentAutoscaler. One of HPA | KEDA | External | None.
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
	//   - "none": no scaler at all. Conditions empty; counters zero.
	// +kubebuilder:validation:Enum=ome;external;none
	// +optional
	ManagedBy AutoscalerManagedBy `json:"managedBy,omitempty"`

	// SpecSource reports which layer of the ISVC -> runtime -> default
	// inheritance chain produced the resolved Autoscaler block. One of
	// "isvc" | "runtime" | "default". Mirrors autoscaler.SpecSource so
	// the operator can debug which layer is contributing the live config.
	// +optional
	SpecSource string `json:"specSource,omitempty"`

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
	// ScaledObject: Ready / Active / Fallback / Paused). Empty when
	// ManagedBy != "ome" — operator-owned scalers are surfaced via
	// scaleTargetRef instead.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ScaleTargetRef is a stripped-down reference to the canonical scale
// target for a Component — the InferenceReplica (for OMENative) or the
// underlying Deployment (for RawDeployment). Published on
// status.components.<comp>.scaleTargetRef so an external scaler knows
// which object to point at (the same GroupKind the user would target
// with kubectl scale <kind>/<name>).
//
// Empty when the Component has no active scaler (e.g., legacy
// RawDeployment Components that have not migrated to the new Autoscaler
// block).
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
