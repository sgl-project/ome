package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageSource represents the different types of storage
type StorageSource string

const (
	ObjectStorage StorageSource = "OBJECT_STORAGE"
	PVC           StorageSource = "PVC"
)

// Storage defines the storage source/location of the data/model
// The fields follow a "1-of" semantic. Users must specify exactly one URL based on given source.
type Storage struct {
	// Represents the type of the storage
	StorageType StorageSource `json:"storageType,omitempty"`

	// ObjectStorageSpec for data/model stored in ObjectStorage
	OSStorageSpec *OSStorage `json:"oSStorageSpec,omitempty"`

	// PVCStorageSpec for data/model stored as PVC
	PVCStorageSpec *PVCStorage `json:"pVCStorageSpec,omitempty"`
}

// OSStorage defines the arguments for object storage
type OSStorage struct {
	BucketName string `json:"bucketName"`
	Namespace  string `json:"namespace"`
	ObjectName string `json:"objectName,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	OboToken   string `json:"oboToken,omitempty"`
}

// PVCStorage defines the arguments for pvc storage
type PVCStorage struct {
	// This field points to the location of the data/model which is mounted onto the pod.
	Path string `json:"path"`
}

// CleanPodPolicy describes how to deal with pods when the job is finished.
type CleanPodPolicy string

const (
	CleanPodPolicyUndefined CleanPodPolicy = ""
	CleanPodPolicyAll       CleanPodPolicy = "All"
	CleanPodPolicyRunning   CleanPodPolicy = "Running"
	CleanPodPolicyNone      CleanPodPolicy = "None"
)

// RestartPolicy describes how the replicas should be restarted.
// Only one of the following restart policies may be specified.
// If none of the following policies is specified, the default one
// is RestartPolicyAlways.
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"

	// RestartPolicyExitCode policy means that user should add exit code by themselves,
	// The job operator will check these exit codes to
	// determine the behavior when an error occurs:
	// - 1-127: permanent error, do not restart.
	// - 128-255: retryable error, will restart the pod.
	RestartPolicyExitCode RestartPolicy = "ExitCode"
)

// RunPolicy encapsulates various runtime policies of the distributed training
// job, for example how to clean up resources and how long the job can stay
// active.
type RunPolicy struct {
	// CleanPodPolicy defines the policy to kill pods after the job completes.
	// Default to None.
	CleanPodPolicy *CleanPodPolicy `json:"cleanPodPolicy,omitempty"`

	// TTLSecondsAfterFinished is the TTL to clean up jobs.
	// It may take extra ReconcilePeriod seconds for the cleanup, since
	// reconcile gets called periodically.
	// Default to infinite.
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// Specifies the duration in seconds relative to the startTime that the job may be active
	// before the system tries to terminate it; value must be positive integer.
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

	// Optional number of retries before marking this job failed.
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// SchedulingPolicy defines the policy related to scheduling, e.g. gang-scheduling
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// suspend specifies whether the Job controller should create Pods or not.
	// If a Job is created with suspend set to true, no Pods are created by
	// the Job controller. If a Job is suspended after creation (i.e. the
	// flag goes from false to true), the Job controller will delete all
	// active Pods and PodGroups associated with this Job.
	// Users must design their workload to gracefully handle this.
	// Suspending a Job will reset the StartTime field of the Job.
	//
	// Defaults to false.
	// +kubebuilder:default:=false
	// +optional
	Suspend *bool `json:"suspend,omitempty"`
}

// SchedulingPolicy encapsulates various scheduling policies of the distributed training
// job, for example `minAvailable` for gang-scheduling.
type SchedulingPolicy struct {
	MinAvailable           *int32                                 `json:"minAvailable,omitempty"`
	Queue                  string                                 `json:"queue,omitempty"`
	MinResources           *map[v1.ResourceName]resource.Quantity `json:"minResources,omitempty"`
	PriorityClass          string                                 `json:"priorityClass,omitempty"`
	ScheduleTimeoutSeconds *int32                                 `json:"scheduleTimeoutSeconds,omitempty"`
}

// JobCondition describes the state of the job at a certain point.
type JobCondition struct {
	// Type of job condition.
	Type JobConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human-readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// JobConditionType defines all kinds of types of JobStatus.
type JobConditionType string

const (
	// JobCreated means the job has been accepted by the system,
	// but one or more of the pods/services has not been started.
	// This includes time before pods being scheduled and launched.
	JobCreated JobConditionType = "Created"

	// JobRunning means all sub-resources (e.g. services/pods) of this job
	// have been successfully scheduled and launched.
	// The training is running without error.
	JobRunning JobConditionType = "Running"

	// JobRestarting means one or more sub-resources (e.g. services/pods) of this job
	// reached phase failed but maybe restarted according to it's restart policy
	// which specified by user in v1.PodTemplateSpec.
	// The training is freezing/pending.
	JobRestarting JobConditionType = "Restarting"

	// JobSucceeded means all sub-resources (e.g. services/pods) of this job
	// reached phase have terminated in success.
	// The training is complete without error.
	JobSucceeded JobConditionType = "Succeeded"

	// JobSuspended means the job has been suspended.
	JobSuspended JobConditionType = "Suspended"

	// JobFailed means one or more sub-resources (e.g. services/pods) of this job
	// reached phase failed with no restarting.
	// The training has failed its execution.
	JobFailed JobConditionType = "Failed"
)

func IsTerminalJobCondition(conditionType JobConditionType) bool {
	return conditionType == JobSucceeded || conditionType == JobFailed
}
