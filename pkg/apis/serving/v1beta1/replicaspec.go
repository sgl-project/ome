package v1beta1

import (
	v1 "k8s.io/api/core/v1"
)

// ReplicaType represents the type of the replica. Each training runtime needs to define its
// own set of ReplicaTypes.
type ReplicaType string

const (
	// CohereLauncher is the type of the master replica.
	CohereLauncher ReplicaType = "Launcher"

	// CohereWorker is the type of the worker replica.
	CohereWorker ReplicaType = "Worker"

	// PytorchMaster is the type of the master replica.
	PytorchMaster ReplicaType = "Master"

	// PytorchWorker is the type of the worker replica.
	PytorchWorker ReplicaType = "Worker"

	// TensorFlowPS is the type of the launcher replica.
	TensorFlowPS ReplicaType = "PS"

	// TensorFlowWorker is the type of the worker replica.
	TensorFlowWorker ReplicaType = "Worker"

	// MPILauncher is the type of the launcher replica.
	MPILauncher ReplicaType = "Launcher"

	// MPIWorker is the type of the worker replica.
	MPIWorker ReplicaType = "Worker"

	// PeftFinetuningReplicaTypeLauncher is the type of the master replica.
	PeftFinetuningReplicaTypeLauncher ReplicaType = "Launcher"

	// PeftFinetuningWorker is the type of the worker replica.
	PeftFinetuningWorker ReplicaType = "Worker"
)

// ReplicaSpec is a description of the replica
type ReplicaSpec struct {
	// Replicas is the desired number of replicas of the given template.
	// If unspecified, defaults to 1.
	ReplicaCount *int32 `json:"replicaCount,omitempty"`

	// Template is the object that describes the pod that
	// will be created for this replica. RestartPolicy in PodTemplateSpec
	// will be overridden by RestartPolicy in ReplicaSpec
	Template v1.PodTemplateSpec `json:"template,omitempty"`

	// Restart policy for all replicas within the job.
	// One of Always, OnFailure, Never and ExitCode.
	// Default to Never.
	RestartPolicy v1.RestartPolicy `json:"restartPolicy,omitempty"`
}

// ReplicaStatusType enum
type ReplicaStatusType string

const (
	NotStarted     ReplicaStatusType = "NotStarted"
	In_Progress    ReplicaStatusType = "InProgress"
	Failed         ReplicaStatusType = "Failed"
	PartialSuccess ReplicaStatusType = "PartialSuccess"
	Succeeded      ReplicaStatusType = "Succeeded"
)

// ReplicaStatus represents the current observed state of the replica.
type ReplicaStatus struct {
	// The number of actively running pods.
	ActivePodCount int32 `json:"activePodCount,omitempty"`

	// The number of pods which reached phase Succeeded.
	SucceededPodCount int32 `json:"succeededPodCount,omitempty"`

	// The number of pods which reached phase Failed.
	FailedPodCount int32 `json:"failedPodCount,omitempty"`

	// Represents current status state of the replica
	StatusType *ReplicaStatusType `json:"replicaStatusType,omitempty"`

	// A Selector is a label query over a set of resources. The result of matchLabels and
	// matchExpressions are ANDed. An empty Selector matches all objects. A null
	// Selector matches no objects.
	Selector string `json:"selector,omitempty"`
}
