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

// ValidateRuntimeAutoscalerPolicyRefs rejects an AutoscalerPolicyRef on any
// runtime component config. The embedded ComponentExtensionSpec admits the
// field structurally, but no controller reads it off a runtime — policy refs
// attach on the InferenceService only — so admitting one here would store
// dead configuration an operator reasonably expects to act.
func ValidateRuntimeAutoscalerPolicyRefs(spec *v1beta1.ServingRuntimeSpec) error {
	if spec == nil {
		return nil
	}
	type componentRef struct {
		name string
		ref  *v1beta1.AutoscalerPolicyRef
	}
	var refs []componentRef
	if spec.EngineConfig != nil {
		refs = append(refs, componentRef{"engineConfig", spec.EngineConfig.AutoscalerPolicyRef})
	}
	if spec.DecoderConfig != nil {
		refs = append(refs, componentRef{"decoderConfig", spec.DecoderConfig.AutoscalerPolicyRef})
	}
	if spec.RouterConfig != nil {
		refs = append(refs, componentRef{"routerConfig", spec.RouterConfig.AutoscalerPolicyRef})
	}
	for _, c := range refs {
		if c.ref != nil {
			return fmt.Errorf("%s: autoscalerPolicyRef is not supported on serving runtimes; policy refs attach on the InferenceService only", c.name)
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
