package isvc

import (
	"context"
	"fmt"

	opensourcev1beta1 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/runtimeselector"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/inferenceservice-router-mutator,mutating=true,failurePolicy=fail,groups=ome.io,resources=inferenceservices,verbs=create,update,versions=v1beta1,name=inferenceservice.ome-webhook-server.inferenceservice-router-mutator

var routerMutatorLogger = logf.Log.WithName("inferenceservice-router-mutator")

type InferenceServiceRouterMutator struct {
	Client          client.Client
	Decoder         admission.Decoder
	RuntimeSelector runtimeselector.Selector
}

// Handle implements admission.Handler interface
func (m *InferenceServiceRouterMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	isvc := &opensourcev1beta1.InferenceService{}
	if err := m.Decoder.Decode(req, isvc); err != nil {
		routerMutatorLogger.Error(err, "Failed to decode InferenceService")
		return admission.Errored(400, err)
	}

	routerMutatorLogger.Info("Processing InferenceService for router injection",
		"namespace", isvc.Namespace, "name", isvc.Name,
		"hasPredictor", isPredictorUsed(isvc),
		"hasRouter", isvc.Spec.Router != nil,
		"hasRuntime", isvc.Spec.Runtime != nil,
		"hasModel", isvc.Spec.Model != nil)

	// Temp skip for router injection if the inference service still use predictor
	// it is rare case, add this function for backward compatible.
	if isPredictorUsed(isvc) {
		routerMutatorLogger.Info("InferenceService still use predictor, won't inject router with it",
			"namespace", isvc.Namespace, "name", isvc.Name)
		return admission.Allowed("InferenceService still use predictor, won't inject router with it, skipping")
	}

	// Skip if router is already explicitly set
	if isvc.Spec.Router != nil {
		routerMutatorLogger.Info("Router already set, skipping",
			"namespace", isvc.Namespace, "name", isvc.Name)
		return admission.Allowed("Router already set, skipping")
	}

	var runtimeSpec *opensourcev1beta1.ServingRuntimeSpec
	var err error
	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		routerMutatorLogger.Info("Resolving explicit runtime",
			"namespace", isvc.Namespace, "name", isvc.Name, "runtime", isvc.Spec.Runtime.Name)
		runtimeSpec, _, err = m.RuntimeSelector.GetRuntime(ctx, isvc.Spec.Runtime.Name, isvc.Namespace)
	} else {
		// Model reference is required when runtime is not specified
		if isvc.Spec.Model == nil || isvc.Spec.Model.Name == "" {
			routerMutatorLogger.Info("No runtime or model reference specified",
				"namespace", isvc.Namespace, "name", isvc.Name)
			return admission.Denied("No runtime or model reference specified, invalid inference service")
		}
		routerMutatorLogger.Info("Auto-selecting runtime from model",
			"namespace", isvc.Namespace, "name", isvc.Name, "model", isvc.Spec.Model.Name)
		runtimeSpec, err = m.resolveRuntimeSpec(ctx, isvc)
	}

	if err != nil {
		routerMutatorLogger.Error(err, "Failed to resolve runtime",
			"namespace", isvc.Namespace, "name", isvc.Name)
		return admission.Denied(fmt.Sprintf("Runtime resolution failed: %v", err))
	}

	// Check if the runtime has router config
	if runtimeSpec.RouterConfig == nil {
		routerMutatorLogger.Info("Runtime has no router config, skipping",
			"namespace", isvc.Namespace, "name", isvc.Name)
		return admission.Allowed("Runtime has no router config, skipping")
	}

	// Inject router with default replicas, the replica of router always be 1
	minReplicas := 1
	isvc.Spec.Router = &opensourcev1beta1.RouterSpec{}
	isvc.Spec.Router.MinReplicas = &minReplicas
	isvc.Spec.Router.MaxReplicas = 1

	routerMutatorLogger.Info("Injected router into InferenceService",
		"namespace", isvc.Namespace, "name", isvc.Name)

	return admission.PatchResponseFromRaw(req.Object.Raw, mustMarshal(isvc))
}

// resolveRuntimeSpec resolves the ServingRuntimeSpec via auto-selection based on the model.
func (m *InferenceServiceRouterMutator) resolveRuntimeSpec(ctx context.Context, isvc *opensourcev1beta1.InferenceService) (*opensourcev1beta1.ServingRuntimeSpec, error) {
	baseModel, _, err := isvcutils.GetBaseModel(m.Client, isvc.Spec.Model.Name, isvc.Namespace)
	if err != nil {
		routerMutatorLogger.Error(err, "Failed to get base model",
			"model", isvc.Spec.Model.Name, "namespace", isvc.Namespace)
		return nil, err
	}

	routerMutatorLogger.Info("Resolved base model, selecting runtime",
		"model", isvc.Spec.Model.Name, "namespace", isvc.Namespace,
		"modelFormat", baseModel.ModelFormat.Name)

	selection, err := m.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc)
	if err != nil {
		routerMutatorLogger.Error(err, "Failed to select runtime",
			"model", isvc.Spec.Model.Name, "namespace", isvc.Namespace)
		return nil, err
	}

	routerMutatorLogger.Info("Selected runtime",
		"runtimeName", selection.Name, "isCluster", selection.IsCluster,
		"hasRouterConfig", selection.Spec.RouterConfig != nil)

	return selection.Spec, nil
}

// isPredictorUsed checks if the Predictor field is existed in the InferenceService
func isPredictorUsed(isvc *opensourcev1beta1.InferenceService) bool {
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
