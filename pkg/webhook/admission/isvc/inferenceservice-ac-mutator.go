package isvc

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	opensourcev1beta1 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"

	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:webhook:path=/inferenceservice-ac-mutator,mutating=true,failurePolicy=fail,groups=ome.io,resources=inferenceservices,verbs=update,create,versions=v1beta1,name=inferenceservice.ome-webhook-server.inferenceservice-ac-mutator
var (
	acMutatorLogger = logf.Log.WithName("inferenceservice-ac-mutator")

	// Supported DAC profile patterns for AcceleratorClass mapping
	// Format: GPU_TYPE[-MEMORY]-x<count>
	// Examples: a10-x1, a100-40g-x2, H100-x4
	supportedDACProfilePattern = regexp.MustCompile(`^(a10|a100-40g|a100-80g|h100|h200|b200)-x([1248])$`)
)

// InferenceServiceACMutator is responsible for setting AcceleratorSelector.AcceleratorClass
// based on the DedicatedAICluster profile associated with the InferenceService.
type InferenceServiceACMutator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// NewInferenceServiceACMutator creates a new InferenceServiceACMutator
func NewInferenceServiceACMutator(c client.Client, decoder admission.Decoder) *InferenceServiceACMutator {
	return &InferenceServiceACMutator{
		Client:  c,
		Decoder: decoder,
	}
}

// Handle implements admission.Handler interface
func (m *InferenceServiceACMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	isvc := &opensourcev1beta1.InferenceService{}
	if err := m.Decoder.Decode(req, isvc); err != nil {
		acMutatorLogger.Error(err, "Failed to decode InferenceService")
		return admission.Errored(400, err)
	}

	acMutatorLogger.Info("Processing InferenceService for AcceleratorClass mutation",
		"namespace", isvc.Namespace, "name", isvc.Name)

	// Check if InferenceService has DAC annotation
	dacName, hasDACAnnotation := isvc.Annotations[constants.DedicatedAICluster]
	if !hasDACAnnotation || dacName == "" {
		acMutatorLogger.Info("No DAC annotation found, skipping AcceleratorClass mutation",
			"namespace", isvc.Namespace, "name", isvc.Name)
		return admission.Allowed("No DAC annotation, skipping")
	}

	// Get the DedicatedAICluster
	dac := &v1beta1.DedicatedAICluster{}
	if err := m.Client.Get(ctx, types.NamespacedName{Name: dacName, Namespace: dacName}, dac); err != nil {
		acMutatorLogger.Error(err, "Failed to get DedicatedAICluster",
			"dacName", dacName)
		// Allow the request to proceed - DAC not found is not a blocking error
		return admission.Allowed("DAC not found, skipping AcceleratorClass mutation")
	}

	// Get the DAC profile name
	profileName := dac.Spec.Profile
	if profileName == "" {
		acMutatorLogger.Info("DAC has no profile specified, skipping AcceleratorClass mutation",
			"dacName", dacName)
		return admission.Allowed("No DAC profile, skipping")
	}

	// Check if the profile matches supported patterns
	acName := mapDACProfileToAcceleratorClass(profileName)
	if acName == "" {
		acMutatorLogger.Info("DAC profile does not match supported patterns, skipping",
			"dacName", dacName, "profile", profileName)
		return admission.Allowed("Profile not in supported list, skipping")
	}

	// Set the AcceleratorClass only the profile matches supported patterns
	if err := m.setAcceleratorClass(isvc, acName); err != nil {
		acMutatorLogger.Error(err, "Failed to set AcceleratorClass",
			"namespace", isvc.Namespace, "name", isvc.Name, "acceleratorClass", acName)
		return admission.Errored(500, err)
	}

	acMutatorLogger.Info("Successfully set AcceleratorClass",
		"namespace", isvc.Namespace, "name", isvc.Name,
		"profile", profileName, "acceleratorClass", acName)

	// Create patch response from the modified InferenceService
	return admission.PatchResponseFromRaw(req.Object.Raw, mustMarshal(isvc))
}

// mapDACProfileToAcceleratorClass maps a DAC profile name to an AcceleratorClass name
// using the normalized format: nvidia-<model>-<memory>-<count>
//
// Mapping rules:
//   - a10-x1      -> nvidia-a10-1
//   - a10-x2      -> nvidia-a10-2
//   - a100-40g-x1 -> nvidia-a100-40gb-1
//   - a100-80g-x4 -> nvidia-a100-80gb-4
//   - h100-x1     -> nvidia-h100-1
//   - h200-x8     -> nvidia-h200-8
//   - b200-x1     -> nvidia-b200-1
//   - b200-x2     -> nvidia-b200-2
//   - b200-x4     -> nvidia-b200-4
//   - b200-x8     -> nvidia-b200-8
func mapDACProfileToAcceleratorClass(profileName string) string {

	matches := supportedDACProfilePattern.FindStringSubmatch(profileName)
	if matches == nil {
		return ""
	}

	gpuType := matches[1] // e.g., "a10", "a100-40g", "h100"
	count := matches[2]   // e.g., "1", "2", "4", "8"

	// Normalize GPU type to AcceleratorClass format
	var acName string
	switch gpuType {
	case "a10":
		acName = "nvidia-a10-" + count
	case "a100-40g":
		acName = "nvidia-a100-40gb-" + count
	case "a100-80g":
		acName = "nvidia-a100-80gb-" + count
	case "h100":
		acName = "nvidia-h100-" + count
	case "h200":
		acName = "nvidia-h200-" + count
	case "b200":
		acName = "nvidia-b200-" + count
	default:
		return ""
	}

	return strings.ToLower(acName)
}

// setAcceleratorClass sets the AcceleratorSelector.AcceleratorClass field on the InferenceService
func (m *InferenceServiceACMutator) setAcceleratorClass(isvc *opensourcev1beta1.InferenceService, acName string) error {
	// Initialize AcceleratorSelector if nil
	if isvc.Spec.AcceleratorSelector == nil {
		isvc.Spec.AcceleratorSelector = &opensourcev1beta1.AcceleratorSelector{}
	}

	// Only set if not already set (don't override user-specified value)
	if isvc.Spec.AcceleratorSelector.AcceleratorClass == nil || *isvc.Spec.AcceleratorSelector.AcceleratorClass == "" {
		isvc.Spec.AcceleratorSelector.AcceleratorClass = &acName
	}

	return nil
}

// mustMarshal marshals the object to JSON, panicking on error
func mustMarshal(obj interface{}) []byte {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return data
}

// GetSupportedDACProfiles returns the list of supported DAC profile patterns
// for AcceleratorClass mapping. Useful for documentation and validation.
func GetSupportedDACProfiles() []string {
	return []string{
		"a10-x1", "a10-x2", "a10-x4",
		"a100-40g-x1", "a100-40g-x2", "a100-40g-x4", "a100-40g-x8",
		"a100-80g-x1", "a100-80g-x2", "a100-80g-x4", "a100-80g-x8",
		"h100-x1", "h100-x2", "h100-x4", "h100-x8",
		"h200-x1", "h200-x2", "h200-x4", "h200-x8",
		"b200-x1", "b200-x2", "b200-x4", "b200-x8",
	}
}
