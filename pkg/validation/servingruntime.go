package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const (
	priorityIsNotSameError = "different priorities assigned for the model format %s"
)

func ValidateSupportedModelFormats(formats []v1beta1.SupportedModelFormat) error {
	for i, f := range formats {
		if f.ModelFormat == nil || f.ModelFormat.Name == "" {
			return fmt.Errorf("spec.supportedModelFormats[%d]: modelFormat.name is required", i)
		}
		if f.ModelFramework == nil || f.ModelFramework.Name == "" {
			return fmt.Errorf("spec.supportedModelFormats[%d]: modelFramework.name is required", i)
		}
	}
	return nil
}

func ValidateModelFormatPrioritySame(spec *v1beta1.ServingRuntimeSpec) error {
	nameToPriority := make(map[string]*int32)

	for _, f := range spec.SupportedModelFormats {
		if f.IsAutoSelectEnabled() {
			if existingPriority, ok := nameToPriority[f.Name]; ok {
				if existingPriority != nil && f.Priority != nil && (*existingPriority != *f.Priority) {
					return fmt.Errorf(priorityIsNotSameError, f.Name)
				}
			} else {
				nameToPriority[f.Name] = f.Priority
			}
		}
	}
	return nil
}
