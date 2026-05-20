package workload

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/components"
)

// WorkloadReconcileRequest encapsulates parameters for workload reconciliation.
// It contains all information required to execute workload reconciliation.
type WorkloadReconcileRequest struct {
	// InferenceService instance
	InferenceService *v1beta1.InferenceService

	// Base model information
	BaseModel     *v1beta1.BaseModelSpec
	BaseModelMeta *metav1.ObjectMeta

	// Runtime information
	Runtime     *v1beta1.ServingRuntimeSpec
	RuntimeName string

	// Merged component specifications
	MergedEngine  *v1beta1.EngineSpec
	MergedDecoder *v1beta1.DecoderSpec
	MergedRouter  *v1beta1.RouterSpec

	// Deployment modes configuration
	DeploymentModes *ComponentDeploymentModes

	// Component builder factory
	ComponentBuilderFactory *components.ComponentBuilderFactory

	// Whether runtime is user-specified
	UserSpecifiedRuntime bool

	// AcceleratorClass information
	EngineAcceleratorClass      *v1beta1.AcceleratorClassSpec
	EngineAcceleratorClassName  string
	DecoderAcceleratorClass     *v1beta1.AcceleratorClassSpec
	DecoderAcceleratorClassName string

	// SupportedModelFormat information
	EngineSupportedModelFormat  *v1beta1.SupportedModelFormat
	DecoderSupportedModelFormat *v1beta1.SupportedModelFormat
}

// ComponentDeploymentModes encapsulates deployment modes for each component.
type ComponentDeploymentModes struct {
	Engine  constants.DeploymentModeType
	Decoder constants.DeploymentModeType
	Router  constants.DeploymentModeType
}
