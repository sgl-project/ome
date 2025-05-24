package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// TrainingJobKind is the Kind name for the TrainingJob.
	TrainingJobKind string = "TrainingJob"
)

// TrainingJob is the Schema for the TrainingJobs API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type TrainingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingJobSpec   `json:"spec,omitempty"`
	Status TrainingJobStatus `json:"status,omitempty"`
}

// TrainingJobSpec defines the base job spec which various training job specs implement.
// It defines the desired state of a training job
type TrainingJobSpec struct {
	// Reference to the training runtime.
	// The field is immutable.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="runtimeRef is immutable"
	RuntimeRef RuntimeRef `json:"runtimeRef"`

	// Trainer defines the trainer to use for the training job.
	// +required
	Trainer *TrainerSpec `json:"trainer,omitempty"`

	// ModelConfig defines the model configuration for the training job.
	// +required
	ModelConfig *ModelConfig `json:"modelConfig,omitempty"`

	// Datasets defines the datasets for the training job.
	// +required
	Datasets *StorageSpec `json:"datasets,omitempty"`

	// HyperParameterTuningConfig defines the hyperparameter configuration and tuning strategy
	HyperParameterTuningConfig *HyperparameterTuningConfig `json:"hyperParameterTuningConfig,omitempty"`

	// Whether the controller should suspend the running TrainJob.
	// Defaults to false.
	// +kubebuilder:default=false
	Suspend *bool `json:"suspend,omitempty"`

	// Labels to apply for the derivative JobSet and Jobs.
	// They will be merged with the TrainingRuntime values.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to apply for the derivative JobSet and Jobs.
	// They will be merged with the TrainingRuntime values.
	Annotations map[string]string `json:"annotations,omitempty"`

	// The compartment ID to use for the training job
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`
}

type TrainerSpec struct {

	// Docker image for the training container.
	Image *string `json:"image,omitempty"`

	// Entrypoint commands for the training container.
	// +listType=atomic
	Command []string `json:"command,omitempty"`

	// Arguments to the entrypoint for the training container.
	// +listType=atomic
	Args []string `json:"args,omitempty"`

	// List of environment variables to set in the training container.
	// These values will be merged with the TrainingRuntime's trainer environments.
	// +listType=map
	// +listMapKey=name
	Env []v1.EnvVar `json:"env,omitempty"`

	// Number of training nodes.
	NumNodes *int32 `json:"numNodes,omitempty"`

	// Compute resources for each training node.
	ResourcesPerNode *v1.ResourceRequirements `json:"resourcesPerNode,omitempty"`

	// Number of processes/workers/slots on every training node.
	// For the Torch runtime: `auto`, `cpu`, `gpu`, or int value can be set.
	// For the MPI runtime only int value can be set.
	NumProcPerNode *string `json:"numProcPerNode,omitempty"`
}

type HyperparameterTuningConfig struct {
	// Method specifies the search algorithm to use (grid, random, bayes)
	// +kubebuilder:validation:Enum=grid;random;bayes
	Method string `json:"method"`

	// Metric defines the objective metric to optimize
	Metric MetricConfig `json:"metric"`

	// Parameters defines the hyperparameters and their search spaces
	Parameters runtime.RawExtension `json:"parameters"`

	// MaxTrials specifies the maximum number of trials to run
	// +optional
	MaxTrials *int32 `json:"maxTrials,omitempty"`
}

// MetricConfig defines the metric to optimize during hyperparameter tuning
type MetricConfig struct {
	// Name of the metric
	Name string `json:"name"`

	// Goal indicates whether to minimize or maximize the metric
	// +kubebuilder:validation:Enum=minimize;maximize
	Goal string `json:"goal"`
}

type ModelConfig struct {
	// InputModel defines the base model name.
	InputModel *string `json:"inputModel,omitempty"`

	// OutputModel defines where the finetune weight (output model) stores.
	OutputModel *StorageSpec `json:"outputModel,omitempty"`
}

type TrainingJobStatus struct {
	// JobsStatus tracks the child Jobs in TrainJob.
	// +listType=map
	// +listMapKey=name
	JobsStatus []JobStatus `json:"jobsStatus,omitempty"`

	// Conditions is an array of current observed job conditions.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// RetryCount represents the number of retries the training job has performed
	RetryCount int `json:"retryCount,omitempty"`

	// StartTime represents time when the training job is acknowledged by the controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime represents time when the training job is completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// LastReconcileTime represents last time when the job was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

type JobStatus struct {
	// Name of the child Job.
	Name string `json:"name"`

	// Ready is the number of child Jobs where the number of ready pods and completed pods
	// is greater than or equal to the total expected pod count for the child Job.
	Ready int32 `json:"ready"`

	// Succeeded is the number of successfully completed child Jobs.
	Succeeded int32 `json:"succeeded"`

	// Failed is the number of failed child Jobs.
	Failed int32 `json:"failed"`

	// Active is the number of child Jobs with at least 1 pod in a running or pending state
	// which are not marked for deletion.
	Active int32 `json:"active"`

	// Suspended is the number of child Jobs which are in a suspended state.
	Suspended int32 `json:"suspended"`
}

// RuntimeRef represents the reference to the existing training runtime.
type RuntimeRef struct {
	// Name of the runtime being referenced.
	// When namespaced-scoped TrainingRuntime is used, the TrainJob must have
	// the same namespace as the deployed runtime.
	Name string `json:"name"`

	// APIGroup of the runtime being referenced.
	// Defaults to `ome.io`.
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`

	// Kind of the runtime being referenced.
	// Defaults to ClusterTrainingRuntime.
	// +kubebuilder:default="ClusterTrainingRuntime"
	Kind *string `json:"kind,omitempty"`
}

const (
	// TrainJobSuspended means that TrainJob is suspended.
	TrainJobSuspended string = "Suspended"

	// TrainJobComplete means that the TrainJob has completed its execution.
	TrainJobComplete string = "Complete"

	// TrainJobFailed means that the actual jobs have failed its execution.
	TrainJobFailed string = "Failed"

	// TrainJobCreated means that the actual jobs creation has succeeded.
	TrainJobCreated string = "Created"
)

const (
	// TrainJobJobsCreationSucceededMessage is status condition message for the
	// {"type": "Created", "status": "True", "reason": "JobsCreationSucceeded"} condition.
	TrainJobJobsCreationSucceededMessage = "Succeeded to create Jobs"

	// TrainJobJobsBuildFailedMessage is status condition message for the
	// {"type": "Created", "status": "True", "reason": "JobsBuildFailed"} condition.
	TrainJobJobsBuildFailedMessage = "Failed to build Jobs"

	// TrainJobJobsCreationFailedMessage is status condition message for the
	// {"type": "Created", "status": "True", "reason": "JobsCreationFailed"} condition.
	TrainJobJobsCreationFailedMessage = "Failed to create Jobs"

	// TrainJobSuspendedMessage is status condition message for the
	// {"type": "Suspended", "status": "True", "reason": "Suspended"} condition.
	TrainJobSuspendedMessage = "TrainJob is suspended"

	// TrainJobResumedMessage is status condition message for the
	// {"type": "Suspended", "status": "True", "reason": "Resumed"} condition.
	TrainJobResumedMessage = "TrainJob is resumed"

	// TrainJobSuspendedReason is the "Suspended" condition reason.
	// When the TrainJob is suspended, this is added.
	TrainJobSuspendedReason string = "Suspended"

	// TrainJobResumedReason is the "Suspended" condition reason.
	// When the TrainJob suspension is changed from True to False, this is added.
	TrainJobResumedReason string = "Resumed"

	// TrainJobJobsCreationSucceededReason is the "Created" condition reason.
	// When the creating objects succeeded after building succeeded, this is added.
	TrainJobJobsCreationSucceededReason string = "JobsCreationSucceeded"

	// TrainJobJobsBuildFailedReason is the "Created" condition reason.
	// When the building objects based on the TrainJob and the specified runtime failed,
	// this is added.
	TrainJobJobsBuildFailedReason string = "JobsBuildFailed"

	// TrainJobJobsCreationFailedReason is the "Created" condition reason.
	// When the creating objects failed even though building succeeded, this is added.
	TrainJobJobsCreationFailedReason string = "JobsCreationFailed"
)

// TrainingJobList contains a list of TrainingJob
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type TrainingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TrainingJob{}, &TrainingJobList{})
}
