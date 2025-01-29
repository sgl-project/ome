package v1beta1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

const (
	// TrainingRuntimeKind is the Kind name for the TrainingRuntime.
	TrainingRuntimeKind string = "TrainingRuntime"
	// ClusterTrainingRuntimeKind is the Kind name for the ClusterTrainingRuntime.
	ClusterTrainingRuntimeKind string = "ClusterTrainingRuntime"
)

// TrainingRuntime is the Schema for the TrainingRuntimes API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="TrainingFrameworks",type="string",JSONPath=".spec.supportedTrainingFrameworks[*].name"
// +kubebuilder:printcolumn:name="TrainingReplicaType",type="string",JSONPath=".spec.replicaType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TrainingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TrainingRuntimeSpec `json:"spec,omitempty"`
}

// TrainingRuntimeSpec defines the desired state of TrainingRuntime
// +k8s:openapi-gen=true
type TrainingRuntimeSpec struct {
	// Configuration for the model training with ML-specific parameters.
	MLPolicy *MLPolicy `json:"mlPolicy,omitempty"`

	// Configuration for the PodGroup to enable gang-scheduling via supported plugins.
	PodGroupPolicy *PodGroupPolicy `json:"podGroupPolicy,omitempty"`

	// JobSet template which will be used by TrainJob.
	Template JobSetTemplateSpec `json:"template"`

	// Labels that will be added to the runtime spec.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations that will be added to the runtime spec.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// The compartment ID to use for the training runtime
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`
}

// JobSetTemplateSpec represents a template of the desired JobSet.
type JobSetTemplateSpec struct {
	// Metadata for custom JobSet's labels and annotations.
	// JobSet name and namespace is equal to the TrainJob's name and namespace.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired JobSet which will be created from TrainJob.
	Spec jobsetv1alpha2.JobSetSpec `json:"spec,omitempty"`
}

// PodGroupPolicy represents a PodGroup configuration for gang-scheduling.
type PodGroupPolicy struct {
	// Coscheduling plugin from the Kubernetes scheduler-plugins for gang-scheduling.
	CoschedulingPodGroupPolicyConfig *CoschedulingPodGroupPolicyConfig `json:"coscheduling,omitempty"`

	// Todo: Add support for Volcano gang-scheduler if necessary.
}

// CoschedulingPodGroupPolicyConfig represents configuration for co-scheduling plugin.
// The number of min members in the PodGroupSpec is always equal to the number of nodes.
type CoschedulingPodGroupPolicyConfig struct {
	// Time threshold to schedule PodGroup for gang-scheduling.
	// If the scheduling timeout is equal to 0, the default value is used.
	// Defaults to 60 seconds.
	ScheduleTimeoutSeconds *int32 `json:"scheduleTimeoutSeconds,omitempty"`
}

// MLPolicy represents configuration for the model training with ML-specific parameters.
type MLPolicy struct {
	// Number of training nodes.
	// Defaults to 1.
	NumNodes *int32 `json:"numNodes,omitempty"`

	// Configuration for the runtime-specific parameters, such as Torch or MPI.
	// Only one of its members may be specified.
	MLPolicyConfig `json:",inline"`
}

// MLPolicyConfig represents the runtime-specific configuration for various technologies.
// One of the following specs can be set.
type MLPolicyConfig struct {
	// Configuration for the PyTorch runtime.
	Torch *TorchMLPolicyConfig `json:"torch,omitempty"`

	// Configuration for the MPI Runtime.
	MPI *MPIMLPolicyConfig `json:"mpi,omitempty"`
}

// TorchMLPolicyConfig represents a PyTorch runtime configuration.
type TorchMLPolicyConfig struct {
	// Number of processes per node.
	// This value is inserted into the `--nproc-per-node` argument of the `torchrun` CLI.
	// Supported values: `auto`, `cpu`, `gpu`, or int value.
	// TODO (andreyvelich): Add kubebuilder validation.
	// Defaults to `auto`.
	NumProcPerNode *string `json:"numProcPerNode,omitempty"`

	// Elastic policy for the PyTorch training.
	ElasticPolicy *TorchElasticPolicy `json:"elasticPolicy,omitempty"`
}

// TorchElasticPolicy represents a configuration for the PyTorch elastic training.
// If this policy is set, the `.spec.numNodes` parameter must be omitted, since min and max node
// is used to configure the `torchrun` CLI argument: `--nnodes=minNodes:maxNodes`.
// Only `c10d` backend is supported for the Rendezvous communication.
type TorchElasticPolicy struct {
	// How many times the training job can be restarted.
	// This value is inserted into the `--max-restarts` argument of the `torchrun` CLI and
	// the `.spec.failurePolicy.maxRestarts` parameter of the training Job.
	MaxRestarts *int32 `json:"maxRestarts,omitempty"`

	// Lower limit for the number of nodes to which training job can scale down.
	MinNodes *int32 `json:"minNodes,omitempty"`

	// Upper limit for the number of nodes to which training job can scale up.
	MaxNodes *int32 `json:"maxNodes,omitempty"`

	// Specification which are used to calculate the desired number of nodes. See the individual
	// metric source types for more information about how each type of metric must respond.
	// The HPA will be created to perform auto-scaling.
	// +listType=atomic
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`
}

// MPIMLPolicyConfig represents a MPI runtime configuration.
type MPIMLPolicyConfig struct {
	// Number of processes per node.
	// This value is equal to the number of slots for each node in the hostfile.
	NumProcPerNode *int32 `json:"numProcPerNode,omitempty"`

	// Implementation name for the MPI to create the appropriate hostfile.
	// Defaults to OpenMPI.
	MPIImplementation *MPIImplementation `json:"mpiImplementation,omitempty"`

	// Directory where SSH keys are mounted.
	SSHAuthMountPath *string `json:"sshAuthMountPath,omitempty"`

	// Whether to run training process on the launcher Job.
	// Defaults to false.
	RunLauncherAsNode *bool `json:"runLauncherAsNode,omitempty"`
}

// MPIImplementation represents one of the supported MPI implementations.
type MPIImplementation string

const (
	MPIImplementationOpenMPI MPIImplementation = "OpenMPI"
	MPIImplementationIntel   MPIImplementation = "Intel"
	MPIImplementationMPICH   MPIImplementation = "MPICH"
)

// TrainingRuntimeList contains a list of TrainingRuntime
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type TrainingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingRuntime `json:"items"`
}

// ClusterTrainingRuntime is the Schema for the TrainingRuntimes API in cluster scope
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="TrainingFrameworks",type="string",JSONPath=".spec.supportedTrainingFrameworks[*].name"
// +kubebuilder:printcolumn:name="TrainingReplicaType",type="string",JSONPath=".spec.replicaType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterTrainingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TrainingRuntimeSpec `json:"spec,omitempty"`
}

// ClusterTrainingRuntimeList contains a list of ClusterTrainingRuntime
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ClusterTrainingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTrainingRuntime `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterTrainingRuntime{}, &ClusterTrainingRuntimeList{}, &TrainingRuntime{}, &TrainingRuntimeList{})
}
