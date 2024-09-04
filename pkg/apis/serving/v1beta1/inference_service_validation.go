package v1beta1

import (
	"fmt"
	"reflect"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"regexp"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/serving/pkg/apis/autoscaling"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// regular expressions for validation of isvc name
const (
	IsvcNameFmt                         string = "[a-z]([-a-z0-9]*[a-z0-9])?"
	StorageUriPresentInTransformerError string = "storage uri should not be specified in transformer container"
)

var (
	// logger for the validation webhook.
	validatorLogger = logf.Log.WithName("inferenceservice-v1beta1-validation-webhook")
	// regular expressions for validation of isvc name
	IsvcRegexp = regexp.MustCompile("^" + IsvcNameFmt + "$")
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-inferenceservice,mutating=false,failurePolicy=fail,groups=ome.io,resources=inferenceservices,versions=v1beta1,name=inferenceservice.ome-webhook-server.validator
var _ webhook.Validator = &InferenceService{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (isvc *InferenceService) ValidateCreate() (admission.Warnings, error) {
	validatorLogger.Info("validate create", "name", isvc.Name)
	var allWarnings admission.Warnings
	annotations := isvc.Annotations

	if err := validateInferenceServiceName(isvc); err != nil {
		return allWarnings, err
	}

	if err := validateInferenceServiceAutoscaler(isvc); err != nil {
		return allWarnings, err
	}

	if err := validateAutoscalerTargetUtilizationPercentage(isvc); err != nil {
		return allWarnings, err
	}

	if err := validateCollocationStorageURI(isvc.Spec.Predictor); err != nil {
		return allWarnings, err
	}

	for _, component := range []Component{
		&isvc.Spec.Predictor,
	} {
		if !reflect.ValueOf(component).IsNil() {
			if err := validateExactlyOneImplementation(component); err != nil {
				return allWarnings, err
			}
			if err := utils.FirstNonNilError([]error{
				component.GetImplementation().Validate(),
				component.GetExtensions().Validate(),
				validateAutoScalingCompExtension(annotations, component.GetExtensions()),
			}); err != nil {
				return allWarnings, err
			}
		}
	}
	return allWarnings, nil
}

// Validate scaling options component extensions
func validateAutoScalingCompExtension(annotations map[string]string, compExtSpec *ComponentExtensionSpec) error {
	deploymentMode := annotations["ome.io/deploymentMode"]
	annotationClass := annotations[autoscaling.ClassAnnotationKey]
	if deploymentMode == string(constants.RawDeployment) || annotationClass == string(autoscaling.HPA) {
		return validateScalingHPACompExtension(compExtSpec)
	}

	return validateScalingKPACompExtension(compExtSpec)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (isvc *InferenceService) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	validatorLogger.Info("validate update", "name", isvc.Name)

	return isvc.ValidateCreate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (isvc *InferenceService) ValidateDelete() (admission.Warnings, error) {
	validatorLogger.Info("validate delete", "name", isvc.Name)
	return nil, nil
}

// GetIntReference returns the pointer for the integer input
func GetIntReference(number int) *int {
	num := number
	return &num
}

// Validation of isvc name
func validateInferenceServiceName(isvc *InferenceService) error {
	if !IsvcRegexp.MatchString(isvc.Name) {
		return fmt.Errorf(InvalidISVCNameFormatError, isvc.Name, IsvcNameFmt)
	}
	return nil
}

// Validation of isvc autoscaler class
func validateInferenceServiceAutoscaler(isvc *InferenceService) error {
	annotations := isvc.ObjectMeta.Annotations
	value, ok := annotations[constants.AutoscalerClass]
	class := constants.AutoscalerClassType(value)
	if ok {
		for _, item := range constants.AutoscalerAllowedClassList {
			if class == item {
				switch class {
				case constants.AutoscalerClassHPA:
					if metric, ok := annotations[constants.AutoscalerMetrics]; ok {
						return validateHPAMetrics(ScaleMetric(metric))
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
func validateHPAMetrics(metric ScaleMetric) error {
	for _, item := range constants.AutoscalerAllowedMetricsList {
		if item == constants.AutoscalerMetricsType(metric) {
			return nil
		}
	}
	return fmt.Errorf("[%s] is not a supported metric", metric)
}

// Validate of autoscaler targetUtilizationPercentage
func validateAutoscalerTargetUtilizationPercentage(isvc *InferenceService) error {
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

func validateScalingHPACompExtension(compExtSpec *ComponentExtensionSpec) error {
	metric := MetricCPU
	if compExtSpec.ScaleMetric != nil {
		metric = *compExtSpec.ScaleMetric
	}

	err := validateHPAMetrics(metric)

	if err != nil {
		return err
	}

	if compExtSpec.ScaleTarget != nil {
		target := *compExtSpec.ScaleTarget
		if metric == MetricCPU && target < 1 || target > 100 {
			return fmt.Errorf("the target utilization percentage should be a [1-100] integer")
		}

		if metric == MetricMemory && target < 1 {
			return fmt.Errorf("the target memory should be greater than 1 MiB")
		}
	}

	return nil
}

func validateKPAMetrics(metric ScaleMetric) error {
	for _, item := range constants.AutoScalerKPAMetricsAllowedList {
		if item == constants.AutoScalerKPAMetricsType(metric) {
			return nil
		}
	}
	return fmt.Errorf("[%s] is not a supported metric", metric)
}

func validateScalingKPACompExtension(compExtSpec *ComponentExtensionSpec) error {
	if compExtSpec.DeploymentStrategy != nil {
		return fmt.Errorf("customizing deploymentStrategy is only supported for raw deployment mode")
	}
	metric := MetricConcurrency
	if compExtSpec.ScaleMetric != nil {
		metric = *compExtSpec.ScaleMetric
	}

	err := validateKPAMetrics(metric)

	if err != nil {
		return err
	}

	if compExtSpec.ScaleTarget != nil {
		target := *compExtSpec.ScaleTarget

		if metric == MetricRPS && target < 1 {
			return fmt.Errorf("the target for rps should be greater than 1")
		}
	}

	return nil
}

// validates if transformer container has storage uri or not in collocation of predictor and transformer scenario
func validateCollocationStorageURI(predictorSpec PredictorSpec) error {
	for _, container := range predictorSpec.Containers {
		if container.Name == constants.TransformerContainerName {
			for _, env := range container.Env {
				if env.Name == constants.CustomSpecStorageUriEnvVarKey {
					return fmt.Errorf(StorageUriPresentInTransformerError)
				}
			}
			break
		}
	}
	return nil
}
