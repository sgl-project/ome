package isvc

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"

	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/sgl-project/ome/pkg/constants"
)

var (
	// logger for the mutating webhook.
	mutatorLogger = logf.Log.WithName("inferenceservice-v1beta1-mutating-webhook")

	// DeprecationWarningPredictor is the warning message for using the deprecated Predictor field
	DeprecationWarningPredictor = "The Predictor field is deprecated and will be removed in a future release. Please use Engine and Model fields instead."
	// Environment variable to control whether Predictor migration is enabled
	EnablePredictorMigrationEnvVar = "ENABLE_PREDICTOR_MIGRATION"
)

// InferenceServiceDefaulter is responsible for setting default values on the InferenceService
// when created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceDefaulter struct {
	ClientSet kubernetes.Interface
}

// +kubebuilder:webhook:path=/mutate-ome-io-v1beta1-inferenceservice,mutating=true,failurePolicy=fail,groups=ome.io,resources=inferenceservices,verbs=create;update,versions=v1beta1,name=inferenceservice.ome-webhook-server.defaulter
var _ webhook.CustomDefaulter = &InferenceServiceDefaulter{}

func (d *InferenceServiceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	isvc, err := convertToInferenceService(obj)
	if err != nil {
		mutatorLogger.Error(err, "Unable to convert object to InferenceService")
		return err
	}
	mutatorLogger.Info("Defaulting InferenceService", "namespace", isvc.Namespace, "name", isvc.Name)
	deployConfig, err := controllerconfig.NewDeployConfig(d.ClientSet)
	if err != nil {
		mutatorLogger.Error(err, "Failed to get deploy config")
		return err
	}
	DefaultInferenceService(isvc, deployConfig)
	return nil
}

// DefaultInferenceService sets default values on the InferenceService
func DefaultInferenceService(isvc *v1beta1.InferenceService, deployConfig *controllerconfig.DeployConfig) {
	// Create annotations map if it doesn't exist
	if isvc.ObjectMeta.Annotations == nil {
		isvc.ObjectMeta.Annotations = map[string]string{}
	}

	// Determine deployment mode based on components
	_, modeExists := isvc.ObjectMeta.Annotations[constants.DeploymentMode]
	if !modeExists {
		// If both Engine and Decoder are specified, set the mode for PD disaggregated deployment
		if isvc.Spec.Engine != nil && isvc.Spec.Decoder != nil {
			// Use the PDDisaggregated deployment mode for PD disaggregated deployments
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = string(constants.PDDisaggregated)
		} else if isvc.Spec.Engine != nil {
			// Check for MultiNode mode: leader and worker with worker.size > 0
			if isvc.Spec.Engine.Leader != nil &&
				isvc.Spec.Engine.Worker != nil &&
				isvc.Spec.Engine.Worker.Size != nil &&
				*isvc.Spec.Engine.Worker.Size > 0 {
				isvc.ObjectMeta.Annotations[constants.DeploymentMode] = string(constants.MultiNode)
			} else if deployConfig != nil && deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
				// Default to RawDeployment mode if not MultiNode
				isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
			}
		} else if deployConfig != nil && deployConfig.DefaultDeploymentMode == string(constants.RawDeployment) {
			// Apply default deployment mode from config if provided
			isvc.ObjectMeta.Annotations[constants.DeploymentMode] = deployConfig.DefaultDeploymentMode
		}
	}

	// Add deprecated warning annotation for Predictor usage
	if isPredictorUsed(isvc) {
		if isvc.ObjectMeta.Annotations == nil {
			isvc.ObjectMeta.Annotations = map[string]string{}
		}
		// Only add the warning if it's not already there
		if _, exists := isvc.ObjectMeta.Annotations[constants.DeprecationWarning]; !exists {
			isvc.ObjectMeta.Annotations[constants.DeprecationWarning] = DeprecationWarningPredictor
		}

		// Check if migration is enabled via environment variable
		enableMigration := shouldEnableMigration()
		if enableMigration {
			// Migrate Predictor fields to Engine and top-level Model/Runtime
			migrateFromPredictorToNewArchitecture(isvc)
		}
	}

	// Set default values for Engine component if present
	if isvc.Spec.Engine != nil {
		defaultEngine(isvc.Spec.Engine)
	}

	// Set default values for Decoder component if present
	if isvc.Spec.Decoder != nil {
		defaultDecoder(isvc.Spec.Decoder)
	}

	// Set default values for Router component if present
	if isvc.Spec.Router != nil {
		defaultRouter(isvc.Spec.Router)
	}
}

// isPredictorUsed checks if the Predictor field is used in the InferenceService
// shouldEnableMigration checks if Predictor migration is enabled via environment variable
func shouldEnableMigration() bool {
	// Check the environment variable - default to true for backward compatibility
	value := os.Getenv(EnablePredictorMigrationEnvVar)
	// If the variable is not set or set to anything other than "false", enable migration
	return value != "false"
}

func isPredictorUsed(isvc *v1beta1.InferenceService) bool {
	// Check if the Predictor has any fields set
	predictor := isvc.Spec.Predictor

	// Check if Model is defined in Predictor
	if predictor.Model != nil && predictor.Model.BaseModel != nil {
		return true
	}

	// Check if MinReplicas or MaxReplicas are set
	if predictor.MinReplicas != nil {
		return true
	}

	// Check other significant fields
	if predictor.ServiceAccountName != "" ||
		len(predictor.Containers) > 0 ||
		len(predictor.Volumes) > 0 ||
		len(predictor.NodeSelector) > 0 ||
		len(predictor.Tolerations) > 0 ||
		predictor.Affinity != nil {
		return true
	}

	return false
}

// migrateFromPredictorToNewArchitecture moves fields from Predictor to Engine and top-level Model/Runtime
func migrateFromPredictorToNewArchitecture(isvc *v1beta1.InferenceService) {
	// Skip migration if Engine is already configured
	if isvc.Spec.Engine != nil {
		return
	}

	// Create Engine component from Predictor
	engine := &v1beta1.EngineSpec{}

	// First migrate the embedded structs: ComponentExtensionSpec and PodSpec fields
	// We use JSON marshaling for safer migration of complex nested structures
	if err := migrateSpecViaJSON(&isvc.Spec.Predictor, engine); err != nil {
		// Log error but continue with migration
		mutatorLogger.Error(err, "Error migrating Predictor to Engine via JSON")
	}

	// Process containers carefully - they could be nil
	if len(isvc.Spec.Predictor.Containers) > 0 {
		// Look for ome-container or containers with 'ome' in the name and map to Runner
		omeContainerFound := false
		var otherContainers []v1.Container

		for _, container := range isvc.Spec.Predictor.Containers {
			// If we find a container that looks like an OME container and we haven't already
			// assigned a Runner, use this container as the Runner
			if !omeContainerFound && (container.Name == "ome-container" || strings.Contains(strings.ToLower(container.Name), "ome")) {
				engine.Runner = &v1beta1.RunnerSpec{
					Container: container,
				}
				omeContainerFound = true
			} else {
				// Keep other containers
				otherContainers = append(otherContainers, container)
			}
		}

		// Only set Containers if we have remaining containers after moving one to Runner
		if len(otherContainers) > 0 {
			engine.Containers = otherContainers
		} else {
			engine.Containers = nil
		}

		// If no OME container was found but we have containers, use the first one as Runner
		if !omeContainerFound && len(isvc.Spec.Predictor.Containers) > 0 {
			// Use the first container as the Runner
			engine.Runner = &v1beta1.RunnerSpec{
				Container: isvc.Spec.Predictor.Containers[0],
			}

			// If there's only one container, set Containers to nil
			if len(isvc.Spec.Predictor.Containers) == 1 {
				engine.Containers = nil
			} else if len(isvc.Spec.Predictor.Containers) > 1 {
				// Otherwise, keep all containers except the first one
				engine.Containers = isvc.Spec.Predictor.Containers[1:]
			}
		}
	}

	// If the predictor has a Worker, migrate it to the Engine
	if isvc.Spec.Predictor.Worker != nil {
		engine.Worker = isvc.Spec.Predictor.Worker
	}

	// Set the Engine
	isvc.Spec.Engine = engine

	// Move Model to top-level if not already there
	if isvc.Spec.Model == nil && isvc.Spec.Predictor.Model != nil {
		// Create ModelRef from BaseModel if it exists
		if isvc.Spec.Predictor.Model.BaseModel != nil {
			clusterBaseModel := "ClusterBaseModel"
			isvc.Spec.Model = &v1beta1.ModelRef{
				Name: *isvc.Spec.Predictor.Model.BaseModel,
				Kind: &clusterBaseModel, // Kind is a *string, needs to be a pointer
			}

			// Copy any fine-tuned weights if present
			if len(isvc.Spec.Predictor.Model.FineTunedWeights) > 0 {
				isvc.Spec.Model.FineTunedWeights = isvc.Spec.Predictor.Model.FineTunedWeights
			}
		}
	}

	// Move Runtime reference if present
	if isvc.Spec.Runtime == nil && isvc.Spec.Predictor.Model != nil && isvc.Spec.Predictor.Model.Runtime != nil {
		clusterServingRuntime := "ClusterServingRuntime"
		isvc.Spec.Runtime = &v1beta1.ServingRuntimeRef{
			Name: *isvc.Spec.Predictor.Model.Runtime,
			Kind: &clusterServingRuntime, // Kind is a *string, needs to be a pointer
		}
	}

}

// migrateSpecViaJSON uses JSON marshaling/unmarshaling to safely migrate from one spec to another
func migrateSpecViaJSON(source, target interface{}) error {
	// Marshal the source spec
	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return err
	}

	// Unmarshal into the target spec
	if err := json.Unmarshal(sourceJSON, target); err != nil {
		return err
	}

	return nil
}

// defaultEngine sets default values for the Engine component
func defaultEngine(engine *v1beta1.EngineSpec) {
	// Set default replica values if not set
	if engine.MinReplicas == nil {
		minReplicas := 1 // MinReplicas is *int, not *int32
		engine.MinReplicas = &minReplicas
	}

	// MaxReplicas is not a pointer type, so check if it's 0 (default value)
	if engine.MaxReplicas == 0 {
		engine.MaxReplicas = 3
	}
}

// defaultDecoder sets default values for the Decoder component
func defaultDecoder(decoder *v1beta1.DecoderSpec) {
	// Set default replica values if not set
	if decoder.MinReplicas == nil {
		minReplicas := 1 // MinReplicas is *int, not *int32
		decoder.MinReplicas = &minReplicas
	}

	// MaxReplicas is not a pointer type, so check if it's 0 (default value)
	if decoder.MaxReplicas == 0 {
		decoder.MaxReplicas = 3
	}
}

// defaultRouter sets default values for the Router component
func defaultRouter(router *v1beta1.RouterSpec) {
	// Set default replica values if not set
	if router.MinReplicas == nil {
		minReplicas := 1 // MinReplicas is *int, not *int32
		router.MinReplicas = &minReplicas
	}

	// MaxReplicas is not a pointer type, so check if it's 0 (default value)
	if router.MaxReplicas == 0 {
		router.MaxReplicas = 2
	}
}
