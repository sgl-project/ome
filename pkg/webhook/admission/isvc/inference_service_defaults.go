package isvc

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

var (
	// logger for the mutating webhook.
	mutatorLogger = logf.Log.WithName("inferenceservice-v1beta1-mutating-webhook")
)

// InferenceServiceDefaulter is responsible for setting default values on the InferenceService
// when created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type InferenceServiceDefaulter struct {
	Client    client.Client
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
	if err = DefaultInferenceService(ctx, d.Client, isvc, deployConfig); err != nil {
		return err
	}
	return nil
}

// DefaultInferenceService sets default values on the InferenceService
func DefaultInferenceService(ctx context.Context, c client.Client, isvc *v1beta1.InferenceService, deployConfig *controllerconfig.DeployConfig) error {
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
