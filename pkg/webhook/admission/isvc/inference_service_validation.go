package isvc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/runtimeselector"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"regexp"

	"github.com/sgl-project/ome/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// regular expressions for validation of isvc name
const (
	IsvcNameFmt                string = "[a-z]([-a-z0-9]*[a-z0-9])?"
	InvalidISVCNameFormatError string = "invalid InferenceService name %q, must match %q"
)

var (
	// logger for the validation webhook.
	inferenceServiceValidatorLogger = logf.Log.WithName("inferenceservice-v1beta1-validation-webhook")
	// IsvcRegexp regular expressions for validation of isvc name
	IsvcRegexp = regexp.MustCompile("^" + IsvcNameFmt + "$")
)

// InferenceServiceValidator is responsible for validating the InferenceService resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceValidator struct {
	Client          client.Client
	RuntimeSelector runtimeselector.Selector
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-inferenceservice,mutating=false,failurePolicy=fail,groups=ome.io,resources=inferenceservices,versions=v1beta1,name=inferenceservice.ome-webhook-server.validator
var _ webhook.CustomValidator = &InferenceServiceValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *InferenceServiceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		inferenceServiceValidatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	inferenceServiceValidatorLogger.Info("validate create", "name", isvc.Name)
	return v.validateInferenceService(ctx, isvc)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *InferenceServiceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	isvc, err := convertToInferenceService(newObj)
	if err != nil {
		inferenceServiceValidatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	inferenceServiceValidatorLogger.Info("validate update", "name", isvc.Name)
	return v.validateInferenceService(ctx, isvc)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *InferenceServiceValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		inferenceServiceValidatorLogger.Error(err, "Unable to convert object to InferenceService")
		return nil, err
	}
	inferenceServiceValidatorLogger.Info("validate delete", "name", isvc.Name)
	return nil, nil
}

// GetIntReference returns the pointer for the integer input
func GetIntReference(number int) *int {
	num := number
	return &num
}

func (v *InferenceServiceValidator) validateInferenceService(ctx context.Context, isvc *v1beta1.InferenceService) (admission.Warnings, error) {
	var allWarnings admission.Warnings

	if err := validateInferenceServiceName(isvc); err != nil {
		return allWarnings, err
	}

	if err := validateInferenceServiceAutoscaler(isvc); err != nil {
		return allWarnings, err
	}

	if err := validateAutoscalerTargetUtilizationPercentage(isvc); err != nil {
		return allWarnings, err
	}

	// New validation logic for Engine/Decoder architecture
	if err := validateEngineDecoderConfiguration(isvc); err != nil {
		return allWarnings, err
	}

	// Validate runtime and model resolution for new architecture
	if isvc.Spec.Engine != nil {
		warnings, err := v.validateRuntimeAndModelResolution(ctx, isvc)
		if err != nil {
			return allWarnings, err
		}
		allWarnings = append(allWarnings, warnings...)
	}
	return allWarnings, nil
}

// Validation of isvc name
func validateInferenceServiceName(isvc *v1beta1.InferenceService) error {
	if !IsvcRegexp.MatchString(isvc.Name) {
		return fmt.Errorf(InvalidISVCNameFormatError, isvc.Name, IsvcNameFmt)
	}
	return nil
}

// Validation of isvc autoscaler class
func validateInferenceServiceAutoscaler(isvc *v1beta1.InferenceService) error {
	annotations := isvc.ObjectMeta.Annotations
	value, ok := annotations[constants.AutoscalerClass]
	class := constants.AutoscalerClassType(value)
	if ok {
		for _, item := range constants.AutoscalerAllowedClassList {
			if class == item {
				switch class {
				case constants.AutoscalerClassHPA:
					if metric, ok := annotations[constants.AutoscalerMetrics]; ok {
						return validateHPAMetrics(v1beta1.ScaleMetric(metric))
					} else {
						return nil
					}
				case constants.AutoscalerClassExternal:
					return nil
				default:
					return fmt.Errorf("unknown autoscaler class [%s]", class)
				}
			}
		}
		return fmt.Errorf("[%s] is not a supported autoscaler class type", value)
	}

	return nil
}

// Validate of autoscaler HPA metrics
func validateHPAMetrics(metric v1beta1.ScaleMetric) error {
	for _, item := range constants.AutoscalerAllowedMetricsList {
		if item == constants.AutoscalerMetricsType(metric) {
			return nil
		}
	}
	return fmt.Errorf("[%s] is not a supported metric", metric)
}

// Validate of autoscaler targetUtilizationPercentage
func validateAutoscalerTargetUtilizationPercentage(isvc *v1beta1.InferenceService) error {
	annotations := isvc.ObjectMeta.Annotations
	if value, ok := annotations[constants.TargetUtilizationPercentage]; ok {
		t, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("the target utilization percentage should be a [1-100] integer")
		} else if t < 1 || t > 100 {
			return fmt.Errorf("the target utilization percentage should be a [1-100] integer")
		}
	}

	return nil
}

// Convert runtime.Object into InferenceService
func convertToInferenceService(obj runtime.Object) (*v1beta1.InferenceService, error) {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil, fmt.Errorf("expected an InferenceService object but got %T", obj)
	}
	return isvc, nil
}

// validateEngineDecoderConfiguration validates Engine/Decoder configuration rules
func validateEngineDecoderConfiguration(isvc *v1beta1.InferenceService) error {
	// Rule 1: If inference service has decoder defined, but not engine, fail
	if isvc.Spec.Decoder != nil && isvc.Spec.Engine == nil {
		return fmt.Errorf("decoder cannot be specified without engine")
	}

	return nil
}

// validateRuntimeAndModelResolution validates runtime and model resolution for new architecture
func (v *InferenceServiceValidator) validateRuntimeAndModelResolution(ctx context.Context, isvc *v1beta1.InferenceService) (admission.Warnings, error) {
	var warnings admission.Warnings

	// Only validate new architecture if Engine is specified (focusing on new spec)
	if isvc.Spec.Engine == nil {
		return warnings, nil
	}

	// Rule 2: If inference service does not have runtime defined in isvc.runtime
	if isvc.Spec.Runtime == nil {
		// Check if engine has full runner config
		if !hasFullRunnerConfig(isvc.Spec.Engine) {
			// Model reference is required when runtime is not specified and engine doesn't have full config
			if isvc.Spec.Model == nil {
				return warnings, fmt.Errorf("model reference is required when runtime is not specified and engine does not have complete runner configuration")
			}

			return v.resolveModelAndRuntime(ctx, isvc, warnings)
		}
	}

	return warnings, nil
}

// resolveModelAndRuntime performs actual model and runtime resolution
func (v *InferenceServiceValidator) resolveModelAndRuntime(ctx context.Context, isvc *v1beta1.InferenceService, warnings admission.Warnings) (admission.Warnings, error) {
	// Resolve model using the new architecture approach
	baseModel, _, err := isvcutils.GetBaseModel(v.Client, isvc.Spec.Model.Name, isvc.Namespace)
	if err != nil {
		return warnings, fmt.Errorf("failed to resolve model %s: %w", isvc.Spec.Model.Name, err)
	}

	// Validate model is not disabled
	if baseModel.Disabled != nil && *baseModel.Disabled {
		return warnings, fmt.Errorf("model %s is disabled", isvc.Spec.Model.Name)
	}

	// Check runtime selection/validation
	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		// Validate specified runtime
		if err := v.RuntimeSelector.ValidateRuntime(ctx, isvc.Spec.Runtime.Name, baseModel, isvc.Namespace); err != nil {
			return warnings, fmt.Errorf("runtime %s does not support model %s: %w",
				isvc.Spec.Runtime.Name, isvc.Spec.Model.Name, err)
		}
		warnings = append(warnings, fmt.Sprintf("Runtime %s is valid for model %s",
			isvc.Spec.Runtime.Name, isvc.Spec.Model.Name))
	} else {
		// Check if runtime can be auto-selected
		selection, err := v.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc.Namespace)
		if err != nil {
			return warnings, fmt.Errorf("no supporting runtime found for model %s and engine does not have complete runner configuration", isvc.Spec.Model.Name)
		}
		// Success - runtime will be auto-selected
		warnings = append(warnings, fmt.Sprintf("Runtime %s will be auto-selected for model %s",
			selection.Name, isvc.Spec.Model.Name))
	}
	return warnings, nil
}

// hasFullRunnerConfig checks if the engine has complete runner configuration
func hasFullRunnerConfig(engine *v1beta1.EngineSpec) bool {
	if engine == nil {
		return false
	}

	// Check if main runner is defined with required fields
	if engine.Runner != nil && engine.Runner.Image != "" {
		return true
	}

	// Check if both leader and worker runners are defined for multi-node
	if engine.Leader != nil && engine.Worker != nil {
		leaderHasRunner := engine.Leader.Runner != nil && engine.Leader.Runner.Image != ""
		workerHasRunner := engine.Worker.Runner != nil && engine.Worker.Runner.Image != ""
		return leaderHasRunner && workerHasRunner
	}

	return false
}
