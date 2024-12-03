package v1beta1

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"context"
	"encoding/json"
	goerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrainingJob is the Schema for the TrainingJobs API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
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
	// +required Specific ClusterBaseModel/BaseModel name to use for hosting the model.
	BaseModel *string `json:"baseModel,omitempty"`

	// Specific training framework to use for the training job.
	TrainingFramework *TrainingFramework `json:"trainingFramework,omitempty"`

	// Hyperparameters for training job
	Hyperparameters runtime.RawExtension `json:"hyperparameters,omitempty"`

	// Data for training and validation
	Datasets map[constants.DatasetType]*Storage `json:"datasetsSpecs,omitempty"`

	// OutputLocation: define the location where training output stores. Checkpointing etc.
	OutputLocation Storage `json:"outputLocation,omitempty"`

	// The compartment ID to use for the training job
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`
}

type TrainingJobStatus struct {
	// JobReplicaStatus contains maps from `ReplicaType` to `ReplicaStatus` that specify
	//  the replica current status condition
	JobReplicaStatus map[ReplicaType]*ReplicaStatus `json:"jobReplicaStatus,omitempty"`

	// Conditions is an array of current observed job conditions.
	Conditions []JobCondition `json:"conditions,omitempty"`

	// Details represent any information about the training job
	Details string `json:"details,omitempty"`

	// RetryCount represents the number of retries the training job has performed
	RetryCount int `json:"retryCount,omitempty"`

	// Represents time when the training job is acknowledged by the controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the training job is completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Represents last time when the job was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// FinetunedWeight reference to the finetuned model being produced
	FinetunedWeightRef ObjectReference `json:"finetunedWeightRef,omitempty"`
}

// TrainingJobList contains a list of TrainingJob
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type TrainingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingJob `json:"items"`
}

func (tjs *TrainingJobStatus) GetLatestTrainingJobConditionType() JobConditionType {
	return tjs.Conditions[len(tjs.Conditions)-1].Type
}

func (tjs *TrainingJobStatus) IsTrainingJobConditionEmpty() bool {
	return len(tjs.Conditions) == 0
}

func (tjs *TrainingJobStatus) IncrementRetry() {
	tjs.RetryCount = tjs.RetryCount + 1
}

func (tjs *TrainingJobStatus) UpdateJobStatus(conditionType JobConditionType, details string) {
	jobCondition := JobCondition{
		Type:   conditionType,
		Status: v1.ConditionTrue,
	}
	tjs.Conditions = append(tjs.Conditions, jobCondition)
	tjs.Details = details
}

// GetBaseModel Get the base model from the given model name.
func (tjs *TrainingJobSpec) GetBaseModel(cl client.Client, name string, namespace string) (*BaseModel, error) {
	baseModel := &BaseModel{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, baseModel)
	if err == nil {
		return baseModel, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No BaseModel with the name: " + name)
}

func (tjs *TrainingJobSpec) GetDatasets() *map[constants.DatasetType]*Storage {
	return &tjs.Datasets
}

func (tjs *TrainingJobSpec) GetModelStorage() *Storage {
	return &tjs.OutputLocation
}

func (tjs *TrainingJobSpec) GetHyperparameters() *runtime.RawExtension {
	return &tjs.Hyperparameters
}

func GetHyperparameterValueByKey(hyperparameters *runtime.RawExtension, targetKey string) string {
	data, err := json.Marshal(hyperparameters)
	if err != nil {
		return ""
	}

	kvmap := make(map[string]json.RawMessage)

	e := json.Unmarshal(data, &kvmap)
	if e != nil {
		return ""
	}

	val, ok := kvmap[targetKey]
	if !ok {
		return ""
	}

	valStr, err := json.Marshal(&val)
	if err != nil {
		return ""
	}

	return string(valStr)
}
