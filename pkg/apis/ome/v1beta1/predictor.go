package v1beta1

import (
	"github.com/sgl-project/sgl-ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
)

// PredictorImplementation defines common functions for all predictors e.g Tensorflow, Triton, etc
// +kubebuilder:object:generate=false
type PredictorImplementation interface {
}

// PredictorSpec defines the configuration for a predictor,
// The following fields follow a "1-of" semantic. Users must specify exactly one spec.
type PredictorSpec struct {
	// Model spec for any arbitrary framework.
	Model *ModelSpec `json:"model,omitempty"`

	// This spec is dual purpose. <br />
	// 1) Provide a full PodSpec for custom predictor.
	// The field PodSpec.Containers is mutually exclusive with other predictors (i.e. TFServing). <br />
	// 2) Provide a predictor (i.e. TFServing) and specify PodSpec
	// overrides, you must not provide PodSpec.Containers in this case. <br />
	PodSpec `json:",inline"`
	// Component extension defines the deployment configurations for a predictor
	ComponentExtensionSpec `json:",inline"`

	// WorkerSpec for the predictor, this is used for multi-node serving without Ray Cluster
	// +optional
	Worker *WorkerSpec `json:"workerSpec,omitempty"`
}

// PredictorExtensionSpec defines configuration shared across all predictor frameworks
type PredictorExtensionSpec struct {
	// This field points to the location of the model which is mounted onto the pod.
	// +optional
	StorageUri *string `json:"storageUri,omitempty"`
	// Runtime version of the predictor docker image
	// +optional
	RuntimeVersion *string `json:"runtimeVersion,omitempty"`
	// Protocol version to use by the predictor (i.e. v1 or v2 or grpc-v1 or grpc-v2)
	// +optional
	ProtocolVersion *constants.InferenceServiceProtocol `json:"protocolVersion,omitempty"`
	// Container enables overrides for the predictor.
	// Each framework will have different defaults that are populated in the underlying container spec.
	// +optional
	v1.Container `json:",inline"`
}

type ModelSpec struct {
	// Specific ClusterServingRuntime/ServingRuntime name to use for deployment.
	// +optional
	Runtime *string `json:"runtime,omitempty"`

	PredictorExtensionSpec `json:",inline"`

	// +required Specific ClusterBaseModel/BaseModel name to use for hosting the model.
	BaseModel *string `json:"baseModel,omitempty"`

	// +optional Specific FineTunedWeight name to use for hosting the additional weights.
	// +listType=atomic
	FineTunedWeights []string `json:"fineTunedWeights,omitempty"`
}
