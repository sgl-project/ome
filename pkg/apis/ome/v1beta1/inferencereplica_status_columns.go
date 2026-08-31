package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// InstanceStatusEncoding identifies the stored per-Instance status
// representation.
// +kubebuilder:validation:Enum=ColumnarV2
type InstanceStatusEncoding string

const (
	// InstanceStatusEncodingColumnarV2 stores per-Instance status in grouped
	// columns and sparse exceptional entries.
	InstanceStatusEncodingColumnarV2 InstanceStatusEncoding = "ColumnarV2"
)

// InstanceStatusColumns is the ColumnarV2 representation of per-Instance
// status. Members defines the complete row set; the remaining fields describe
// values for subsets of those members.
type InstanceStatusColumns struct {
	// Members is the canonical index set containing every logical row.
	// +kubebuilder:validation:MinLength=1
	Members string `json:"members"`

	// RowOrder preserves a nonascending dense row order. Its absence means
	// ascending member order.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	RowOrder []int32 `json:"rowOrder,omitempty"`

	// Phases provides exactly one phase for every member.
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	Phases []InstanceStatusPhaseGroup `json:"phases"`

	// RunningRevisions groups members by nonempty running revision.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	RunningRevisions []InstanceStatusStringGroup `json:"runningRevisions,omitempty"`

	// TargetRevisions groups members by nonempty target revision.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	TargetRevisions []InstanceStatusStringGroup `json:"targetRevisions,omitempty"`

	// Incarnations groups members by nonzero incarnation.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	Incarnations []InstanceStatusInt64Group `json:"incarnations,omitempty"`

	// PodCounts groups members by positive pod count.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	PodCounts []InstanceStatusCountGroup `json:"podCounts,omitempty"`

	// ServingPodCounts groups members by positive serving pod count.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	ServingPodCounts []InstanceStatusCountGroup `json:"servingPodCounts,omitempty"`

	// AvailablePodCounts groups members by positive available pod count.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	AvailablePodCounts []InstanceStatusCountGroup `json:"availablePodCounts,omitempty"`

	// Admitted contains the members whose admitted value is true.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Admitted *string `json:"admitted,omitempty"`

	// ActiveOrdinalOne contains the members whose active ordinal is one.
	// +optional
	// +kubebuilder:validation:MinLength=1
	ActiveOrdinalOne *string `json:"activeOrdinalOne,omitempty"`

	// Entries stores fields that are sparse or cannot be usefully grouped.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=atomic
	Entries []InstanceStatusColumnEntry `json:"entries,omitempty"`
}

// InstanceStatusPhaseGroup assigns one lifecycle phase to an index set.
type InstanceStatusPhaseGroup struct {
	Value OMENativeInstancePhase `json:"value"`

	// +kubebuilder:validation:MinLength=1
	Indexes string `json:"indexes"`
}

// InstanceStatusStringGroup assigns one nonempty string to an index set.
type InstanceStatusStringGroup struct {
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value"`

	// +kubebuilder:validation:MinLength=1
	Indexes string `json:"indexes"`
}

// InstanceStatusInt64Group assigns one signed integer to an index set.
// +kubebuilder:validation:XValidation:rule="self.value != 0",message="value must be nonzero"
type InstanceStatusInt64Group struct {
	Value int64 `json:"value"`

	// +kubebuilder:validation:MinLength=1
	Indexes string `json:"indexes"`
}

// InstanceStatusCountGroup assigns one positive count to an index set.
type InstanceStatusCountGroup struct {
	// +kubebuilder:validation:Minimum=1
	Value int32 `json:"value"`

	// +kubebuilder:validation:MinLength=1
	Indexes string `json:"indexes"`
}

// InstanceStatusColumnEntry stores exceptional fields for one member.
// +kubebuilder:validation:XValidation:rule="has(self.conditions) || has(self.operation) || has(self.lastFailure)",message="an entry must contain at least one exceptional field"
type InstanceStatusColumnEntry struct {
	// Index identifies the member described by this entry.
	// +kubebuilder:validation:Minimum=0
	Index int32 `json:"index"`

	// Conditions preserves the member's condition records.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Operation is the member's in-flight durable operation.
	// +optional
	Operation *InstanceOperation `json:"operation,omitempty"`

	// LastFailure is the member's retained failure diagnostic.
	// +optional
	LastFailure *InstanceTermination `json:"lastFailure,omitempty"`
}
