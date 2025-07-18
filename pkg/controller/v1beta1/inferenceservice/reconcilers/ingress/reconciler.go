package ingress

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/factory"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

var mainLog = logf.Log.WithName("IngressReconciler")

// IngressReconciler is the new main ingress reconciler using the refactored architecture
type IngressReconciler struct {
	client        client.Client
	clientset     kubernetes.Interface
	scheme        *runtime.Scheme
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	factory       interfaces.StrategyFactory
}

// NewIngressReconciler creates a new main ingress reconciler
func NewIngressReconciler(
	client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	ingressConfig *controllerconfig.IngressConfig,
	isvcConfig *controllerconfig.InferenceServicesConfig,
) interfaces.Reconciler {
	// Create factory
	strategyFactory := factory.NewStrategyFactory(clientset)

	return &IngressReconciler{
		client:        client,
		clientset:     clientset,
		scheme:        scheme,
		ingressConfig: ingressConfig,
		isvcConfig:    isvcConfig,
		factory:       strategyFactory,
	}
}

// ReconcileWithDeploymentMode reconciles ingress resources for a specific deployment mode
func (r *IngressReconciler) ReconcileWithDeploymentMode(ctx context.Context, isvc *v1beta1.InferenceService, deploymentMode constants.DeploymentModeType) error {
	mainLog := logf.FromContext(ctx)

	// Check if ingress creation is disabled
	if r.ingressConfig.DisableIngressCreation {
		mainLog.Info("Ingress creation disabled, skipping ingress reconciliation", "isvc", isvc.Name)

		// Set IngressReady to True since we're intentionally not creating ingress
		// External service will be created as fallback for cluster access
		mainLog.Info("Setting IngressReady condition to True with reason IngressDisabled", "isvc", isvc.Name)
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:    v1beta1.IngressReady,
			Status:  corev1.ConditionTrue,
			Reason:  "IngressDisabled",
			Message: "Ingress creation is disabled, using external service for access",
		})

		mainLog.Info("Returning early from ingress reconciler due to disabled ingress creation", "isvc", isvc.Name)
		return nil
	}

	// Resolve ingress config with annotation overrides
	resolvedIngressConfig := isvcutils.ResolveIngressConfig(r.ingressConfig, isvc.Annotations)

	// Create reconciler options
	opts := interfaces.ReconcilerOptions{
		Client:        r.client,
		Scheme:        r.scheme,
		IngressConfig: resolvedIngressConfig,
		IsvcConfig:    r.isvcConfig,
	}

	// Get the appropriate strategy
	strategy, err := r.getStrategy(deploymentMode, opts)
	if err != nil {
		return fmt.Errorf("failed to get ingress strategy for deployment mode %s: %w", deploymentMode, err)
	}

	mainLog.Info("Using ingress strategy",
		"strategy", strategy.GetName(),
		"isvc", isvc.Name)

	// Execute the strategy
	return strategy.Reconcile(ctx, isvc)
}

// Reconcile orchestrates the ingress reconciliation using the appropriate strategy
func (r *IngressReconciler) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) error {
	// Determine deployment mode for ingress strategy selection
	deploymentMode := r.getDeploymentMode(isvc, isvc.Spec.Engine, isvc.Spec.Decoder, isvc.Spec.Router)

	return r.ReconcileWithDeploymentMode(ctx, isvc, deploymentMode)
}

// getDeploymentMode determines deployment mode using new spec-based logic
func (r *IngressReconciler) getDeploymentMode(isvc *v1beta1.InferenceService, engine *v1beta1.EngineSpec, decoder *v1beta1.DecoderSpec, router *v1beta1.RouterSpec) constants.DeploymentModeType {
	// Determine entrypoint component for deployment mode selection
	entrypointComponent := isvcutils.DetermineEntrypointComponent(isvc)

	// Determine deployment modes for all components
	engineMode, decoderMode, routerMode, err := isvcutils.DetermineDeploymentModes(engine, decoder, router, nil)
	if err != nil {
		mainLog.Error(err, "Failed to determine deployment modes, falling back to RawDeployment", "isvc", isvc.Name)
		return constants.RawDeployment
	}

	// Return the deployment mode of the entrypoint component
	switch entrypointComponent {
	case v1beta1.RouterComponent:
		return routerMode
	case v1beta1.EngineComponent:
		return engineMode
	case v1beta1.DecoderComponent:
		return decoderMode
	default:
		return engineMode
	}
}

// getStrategy returns the appropriate strategy for the deployment mode
func (r *IngressReconciler) getStrategy(deploymentMode constants.DeploymentModeType, opts interfaces.ReconcilerOptions) (interfaces.IngressStrategy, error) {
	if r.factory == nil {
		return nil, fmt.Errorf("strategy factory is not initialized")
	}
	return r.factory.CreateStrategyWithOptions(string(deploymentMode), opts)
}
