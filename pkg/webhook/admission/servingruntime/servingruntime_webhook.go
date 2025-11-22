package servingruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

var log = logf.Log.WithName(constants.ServingRuntimeValidatorWebhookName)

const (
	InvalidPriorityError                        = "same priority assigned for the model format %s"
	InvalidPriorityServingRuntimeError          = "%s in the servingruntimes %s and %s in namespace %s"
	InvalidPriorityClusterServingRuntimeError   = "%s in the clusterservingruntimes %s and %s"
	PriorityIsNotSameError                      = "different priorities assigned for the model format %s"
	PriorityIsNotSameServingRuntimeError        = "%s under the servingruntime %s"
	PriorityIsNotSameClusterServingRuntimeError = "%s under the clusterservingruntime %s"
	ChainsawInjectAnnotationNotAllowError       = "chainsaw inject annotation is not allowed"
	InvalidConfigurationError                   = "invalid configuration: %s"
	MultiNodeConfigurationError                 = "for MultiNode deployment, both leader and worker must be defined and worker.size must be greater than 0"
	RawDeploymentConfigurationError             = "for RawDeployment, leader and worker must not be defined"
	UnknownAcceleratorClassError                = "unknown accelerator classes referenced in AcceleratorRequirements: %v"
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-clusterservingruntime,mutating=false,failurePolicy=fail,groups=ome.io,resources=clusterservingruntimes,versions=v1beta1,name=clusterservingruntime.ome-webhook-server.validator

type ClusterServingRuntimeValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-servingruntime,mutating=false,failurePolicy=fail,groups=ome.io,resources=servingruntimes,versions=v1beta1,name=servingruntime.ome-webhook-server.validator

type ServingRuntimeValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

func (sr *ServingRuntimeValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	servingRuntime := &v1beta1.ServingRuntime{}
	if err := sr.Decoder.Decode(req, servingRuntime); err != nil {
		log.Error(err, "Failed to decode serving runtime", "name", servingRuntime.Name, "namespace", servingRuntime.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	ExistingRuntimes := &v1beta1.ServingRuntimeList{}
	if err := sr.Client.List(context.TODO(), ExistingRuntimes, client.InNamespace(servingRuntime.Namespace)); err != nil {
		log.Error(err, "Failed to get serving runtime list", "namespace", servingRuntime.Namespace)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Only validate for priority if the new serving runtime is not disabled
	if servingRuntime.Spec.IsDisabled() {
		return admission.Allowed("")
	}

	// Validate the configuration based on engineConfig and decoderConfig
	if err := validateServingRuntimeConfiguration(&servingRuntime.Spec); err != nil {
		return admission.Denied(fmt.Sprintf(InvalidConfigurationError, err.Error()))
	}

	// Validate that all referenced accelerator classes exist
	if err := validateAcceleratorClasses(ctx, sr.Client, &servingRuntime.Spec); err != nil {
		log.Info("Accelerator class validation failed", "name", servingRuntime.Name, "namespace", servingRuntime.Namespace, "error", err)
		return admission.Denied(err.Error())
	}

	for i := range ExistingRuntimes.Items {
		if err := validateModelFormatPrioritySame(&servingRuntime.Spec); err != nil {
			return admission.Denied(fmt.Sprintf(PriorityIsNotSameServingRuntimeError, err.Error(), servingRuntime.Name))
		}

		if err := validateServingRuntimeAnnotations(&servingRuntime.Spec); err != nil {
			return admission.Denied(ChainsawInjectAnnotationNotAllowError)
		}

		if err := validateServingRuntimePriority(&servingRuntime.Spec, &ExistingRuntimes.Items[i].Spec, servingRuntime.Name, ExistingRuntimes.Items[i].Name); err != nil {
			return admission.Denied(fmt.Sprintf(InvalidPriorityServingRuntimeError, err.Error(), ExistingRuntimes.Items[i].Name, servingRuntime.Name, servingRuntime.Namespace))
		}
	}
	return admission.Allowed("")
}

// Handle validates the incoming request
func (csr *ClusterServingRuntimeValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	clusterServingRuntime := &v1beta1.ClusterServingRuntime{}
	if err := csr.Decoder.Decode(req, clusterServingRuntime); err != nil {
		log.Error(err, "Failed to decode cluster serving runtime", "name", clusterServingRuntime.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	ExistingRuntimes := &v1beta1.ClusterServingRuntimeList{}
	if err := csr.Client.List(context.TODO(), ExistingRuntimes); err != nil {
		log.Error(err, "Failed to get cluster serving runtime list")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Only validate for priority if the new cluster serving runtime is not disabled
	if clusterServingRuntime.Spec.IsDisabled() {
		return admission.Allowed("")
	}

	// Validate the configuration based on engineConfig and decoderConfig
	if err := validateServingRuntimeConfiguration(&clusterServingRuntime.Spec); err != nil {
		return admission.Denied(fmt.Sprintf(InvalidConfigurationError, err.Error()))
	}

	// Validate that all referenced accelerator classes exist
	if err := validateAcceleratorClasses(ctx, csr.Client, &clusterServingRuntime.Spec); err != nil {
		log.Info("Accelerator class validation failed", "name", clusterServingRuntime.Name, "error", err)
		return admission.Denied(err.Error())
	}

	for i := range ExistingRuntimes.Items {
		if err := validateModelFormatPrioritySame(&clusterServingRuntime.Spec); err != nil {
			return admission.Denied(fmt.Sprintf(PriorityIsNotSameClusterServingRuntimeError, err.Error(), clusterServingRuntime.Name))
		}

		if err := validateServingRuntimeAnnotations(&clusterServingRuntime.Spec); err != nil {
			return admission.Denied(ChainsawInjectAnnotationNotAllowError)
		}

		if err := validateServingRuntimePriority(&clusterServingRuntime.Spec, &ExistingRuntimes.Items[i].Spec, clusterServingRuntime.Name, ExistingRuntimes.Items[i].Name); err != nil {
			return admission.Denied(fmt.Sprintf(InvalidPriorityClusterServingRuntimeError, err.Error(), ExistingRuntimes.Items[i].Name, clusterServingRuntime.Name))
		}
	}
	return admission.Allowed("")
}

func areSupportedModelFormatsEqual(m1 v1beta1.SupportedModelFormat, m2 v1beta1.SupportedModelFormat) bool {
	if strings.EqualFold(m1.Name, m2.Name) &&
		((m1.Version == nil && m2.Version == nil) || (m1.Version != nil && m2.Version != nil && *m1.Version == *m2.Version)) &&
		((m1.Quantization == nil && m2.Quantization == nil) || (m1.Quantization != nil && m2.Quantization != nil && *m1.Quantization == *m2.Quantization)) &&
		((m1.ModelFramework == nil && m2.ModelFramework == nil) || (m1.ModelFramework != nil && m2.ModelFramework != nil && *m1.ModelFramework == *m2.ModelFramework)) &&
		((m1.ModelFormat == nil && m2.ModelFormat == nil) || (m1.ModelFormat != nil && m2.ModelFormat != nil && *m1.ModelFormat == *m2.ModelFormat)) &&
		((m1.ModelArchitecture == nil && m2.ModelArchitecture == nil) || (m1.ModelArchitecture != nil && m2.ModelArchitecture != nil && *m1.ModelArchitecture == *m2.ModelArchitecture)) {
		return true
	}
	return false
}

func areModelSizeRangesEqual(range1 *v1beta1.ModelSizeRangeSpec, range2 *v1beta1.ModelSizeRangeSpec) bool {
	if range1 == nil || range2 == nil {
		return range1 == range2
	}

	// Compare Min values
	if (range1.Min == nil) != (range2.Min == nil) {
		return false
	}
	if range1.Min != nil && range2.Min != nil && *range1.Min != *range2.Min {
		return false
	}

	// Compare Max values
	if (range1.Max == nil) != (range2.Max == nil) {
		return false
	}
	if range1.Max != nil && range2.Max != nil && *range1.Max != *range2.Max {
		return false
	}

	return true
}

func validateServingRuntimeAnnotations(servingRuntime *v1beta1.ServingRuntimeSpec) error {
	if servingRuntime.ServingRuntimePodSpec.Annotations == nil {
		return nil
	}
	return nil
}

func validateModelFormatPrioritySame(newSpec *v1beta1.ServingRuntimeSpec) error {
	nameToPriority := make(map[string]*int32)

	// Validate when same model format has same priority under same runtime.
	// If the same model format has different prority value then throws the error
	for _, newModelFormat := range newSpec.SupportedModelFormats {
		// Only validate priority if autoselect is ture
		if newModelFormat.IsAutoSelectEnabled() {
			if existingPriority, ok := nameToPriority[newModelFormat.Name]; ok {
				if existingPriority != nil && newModelFormat.Priority != nil && (*existingPriority != *newModelFormat.Priority) {
					return fmt.Errorf(PriorityIsNotSameError, newModelFormat.Name)
				}
			} else {
				nameToPriority[newModelFormat.Name] = newModelFormat.Priority
			}
		}
	}
	return nil
}

func validateServingRuntimePriority(newSpec *v1beta1.ServingRuntimeSpec, existingSpec *v1beta1.ServingRuntimeSpec, existingRuntimeName string, newRuntimeName string) error {
	// Skip the runtime if it is disabled or both are not multi model runtime and in update scenario skip the existing runtime if it is same as the new runtime
	if (existingSpec.IsDisabled()) || (existingRuntimeName == newRuntimeName) {
		return nil
	}
	// Only validate for priority if both servingruntimes supports the same protocol version
	isTheProtocolSame := false
	for _, protocolVersion := range existingSpec.ProtocolVersions {
		if contains(newSpec.ProtocolVersions, protocolVersion) {
			isTheProtocolSame = true
			break
		}
	}
	if isTheProtocolSame {
		for _, existingModelFormat := range existingSpec.SupportedModelFormats {
			for _, newModelFormat := range newSpec.SupportedModelFormats {
				// Only validate priority if auto select is true and model formats and size ranges are equal
				if existingModelFormat.IsAutoSelectEnabled() && newModelFormat.IsAutoSelectEnabled() &&
					areSupportedModelFormatsEqual(existingModelFormat, newModelFormat) &&
					areModelSizeRangesEqual(existingSpec.ModelSizeRange, newSpec.ModelSizeRange) {
					if existingModelFormat.Priority != nil && newModelFormat.Priority != nil && *existingModelFormat.Priority == *newModelFormat.Priority {
						return fmt.Errorf(InvalidPriorityError, newModelFormat.Name)
					}
				}
			}
		}
	}
	return nil
}

func validateServingRuntimeConfiguration(spec *v1beta1.ServingRuntimeSpec) error {
	// Check if both engineConfig and decoderConfig are specified
	hasEngineConfig := spec.EngineConfig != nil
	hasDecoderConfig := spec.DecoderConfig != nil

	// If both engineConfig and decoderConfig are specified, this is a PDDisaggregated deployment
	// No additional validation needed for this case
	if hasEngineConfig && hasDecoderConfig {
		// This is PDDisaggregated mode
		return nil
	}

	// First, validate any worker configuration regardless of deployment mode
	// If worker size is specified as 0 or negative, that's always invalid
	if spec.WorkerPodSpec != nil && spec.WorkerPodSpec.Size != nil && *spec.WorkerPodSpec.Size <= 0 {
		return errors.New(MultiNodeConfigurationError)
	}

	// Check for explicit deployment mode in environment variables
	isExplicitMultiNode := false
	isExplicitRawDeployment := false
	for _, container := range spec.Containers {
		for _, env := range container.Env {
			if env.Name == "DEPLOYMENT_MODE" {
				if env.Value == string(constants.MultiNode) {
					isExplicitMultiNode = true
				} else if env.Value == string(constants.RawDeployment) {
					isExplicitRawDeployment = true
				}
			}
		}
	}

	// If only engineConfig is specified, check for the deployment mode configuration
	if hasEngineConfig && !hasDecoderConfig {
		// Check if this is a MultiNode configuration
		isMultiNode := isExplicitMultiNode

		// If no explicit mode is set, check worker configuration to determine mode
		if !isExplicitMultiNode && !isExplicitRawDeployment {
			// Check if there are worker pods defined with size > 0
			if spec.WorkerPodSpec != nil && spec.WorkerPodSpec.Size != nil && *spec.WorkerPodSpec.Size > 0 {
				isMultiNode = true
			}
		}

		// For MultiNode configuration, validate according to requirements:
		// - Must have both leader and worker defined
		// - Worker size must be greater than 0
		if isMultiNode {
			// Check if worker is properly defined
			if spec.WorkerPodSpec == nil || spec.WorkerPodSpec.Size == nil || *spec.WorkerPodSpec.Size <= 0 {
				return errors.New(MultiNodeConfigurationError)
			}

			// In the future, we can add leader node validation here when the API supports it
		}

		// For RawDeployment, validate that workers are not defined
		if isExplicitRawDeployment {
			// If explicit RawDeployment mode is set and there are workers, that's invalid
			if spec.WorkerPodSpec != nil && spec.WorkerPodSpec.Size != nil && *spec.WorkerPodSpec.Size > 0 {
				return errors.New(RawDeploymentConfigurationError)
			}
		} else if !isMultiNode {
			// Default case is RawDeployment without explicit mode set
			// Check that no worker configuration is present for default RawDeployment
			if spec.WorkerPodSpec != nil && spec.WorkerPodSpec.Size != nil && *spec.WorkerPodSpec.Size > 0 {
				return errors.New(RawDeploymentConfigurationError)
			}
		}
	}

	return nil
}

func contains[T comparable](slice []T, element T) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

// validateAcceleratorClasses checks that all accelerator classes referenced in the runtime spec exist.
// This is a strict validation to ensure that:
// 1. Typos in accelerator class names are caught early
// 2. Runtime scheduling won't fail due to missing accelerator definitions
// 3. Cluster operators can safely create runtimes knowing their accelerator dependencies are met
func validateAcceleratorClasses(ctx context.Context, c client.Client, spec *v1beta1.ServingRuntimeSpec) error {
	if spec.AcceleratorRequirements == nil || len(spec.AcceleratorRequirements.AcceleratorClasses) == 0 {
		return nil
	}

	// Fetch all AcceleratorClasses in a single API call for better performance
	allClasses := &v1beta1.AcceleratorClassList{}
	if err := c.List(ctx, allClasses); err != nil {
		return fmt.Errorf("failed to list accelerator classes: %w", err)
	}

	// Build a set for O(1) lookup
	existingClasses := make(map[string]bool, len(allClasses.Items))
	for _, ac := range allClasses.Items {
		existingClasses[ac.Name] = true
	}

	// Collect all missing classes to report them together
	var missingClasses []string
	for _, className := range spec.AcceleratorRequirements.AcceleratorClasses {
		if !existingClasses[className] {
			missingClasses = append(missingClasses, className)
		}
	}

	if len(missingClasses) > 0 {
		return fmt.Errorf(UnknownAcceleratorClassError, missingClasses)
	}

	return nil
}
