package v1beta1

import (
	"reflect"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/utils"
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
}

var _ Component = &PredictorSpec{}

// PredictorExtensionSpec defines configuration shared across all predictor frameworks
type PredictorExtensionSpec struct {
	// This field points to the location of the model which is mounted onto the pod.
	// +optional
	StorageURI *string `json:"storageUri,omitempty"`
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

// GetImplementations returns the implementations for the component
func (s *PredictorSpec) GetImplementations() []ComponentImplementation {
	implementations := NonNilComponents([]ComponentImplementation{
		s.Model,
	})
	// This struct is not a pointer, so it will never be nil; include if containers are specified
	if len(s.PodSpec.Containers) != 0 {
		implementations = append(implementations, NewCustomPredictor(&s.PodSpec))
	}

	return implementations
}

// GetImplementation returns the implementation for the component
func (s *PredictorSpec) GetImplementation() ComponentImplementation {
	return s.GetImplementations()[0]
}

// GetExtensions returns the extensions for the component
func (s *PredictorSpec) GetExtensions() *ComponentExtensionSpec {
	return &s.ComponentExtensionSpec
}

// Validate returns an error if invalid
func (p *PredictorExtensionSpec) Validate() error {
	return utils.FirstNonNilError([]error{
		// TODO: Re-enable storage spec validation once azure/gcs are supported.
		// Enabling this currently prevents those storage types from working with ModelMesh.
		// validateStorageSpec(p.GetStorageSpec(), p.GetStorageUri()),
	})
}

// GetStorageUri returns the predictor storage Uri
func (p *PredictorExtensionSpec) GetStorageUri() *string {
	return p.StorageURI
}

// GetPredictorImplementations GetPredictor returns the implementation for the predictor
func (s *PredictorSpec) GetPredictorImplementations() []ComponentImplementation {
	implementations := NonNilPredictors([]ComponentImplementation{
		s.Model,
	})
	// This struct is not a pointer, so it will never be nil; include if containers are specified
	if len(s.PodSpec.Containers) != 0 {
		implementations = append(implementations, NewCustomPredictor(&s.PodSpec))
	}
	return implementations
}

func (s *PredictorSpec) GetPredictorImplementation() *ComponentImplementation {
	predictors := s.GetPredictorImplementations()
	if len(predictors) == 0 {
		return nil
	}
	return &predictors[0]
}

func NonNilPredictors(objects []ComponentImplementation) (results []ComponentImplementation) {
	for _, object := range objects {
		if !reflect.ValueOf(object).IsNil() {
			results = append(results, object)
		}
	}
	return results
}
