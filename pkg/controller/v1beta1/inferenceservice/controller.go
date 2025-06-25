package inferenceservice

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/status"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"

	v1beta2 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	multimodelconfig "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/modelconfig"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/components"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/external_service"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	isvcutils "github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"github.com/sgl-project/ome/pkg/utils"
	knapis "knative.dev/pkg/apis"
)

// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices;inferenceservices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=servingruntimes;servingruntimes/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=servingruntimes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=clusterservingruntimes;clusterservingruntimes/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=clusterservingruntimes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=basemodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=basemodels;basemodels/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=finetunedweights/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=finetunedweights;finetunedweights/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=clusterbasemodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=clusterbasemodels;basemodels/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=sidecars,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=ray.io,resources=rayclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ray.io,resources=rayclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ray.io,resources=rayclusters/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/finalizers,verbs=get;list;watch;create;update;patch;delete

// InferenceServiceState describes the Readiness of the InferenceService
type InferenceServiceState string

// Different InferenceServiceState an InferenceService may have.
const (
	InferenceServiceReadyState    InferenceServiceState = "InferenceServiceReady"
	InferenceServiceNotReadyState InferenceServiceState = "InferenceServiceNotReady"
)

// InferenceServiceReconciler reconciles an InferenceService object
type InferenceServiceReconciler struct {
	client.Client
	ClientConfig  *rest.Config
	Clientset     kubernetes.Interface
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	StatusManager *status.StatusReconciler
}

func (r *InferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the InferenceService instance
	isvc := &v1beta2.InferenceService{}
	if err := r.Get(ctx, req.NamespacedName, isvc); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// get annotations from isvc
	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	deployConfig, err := controllerconfig.NewDeployConfig(r.Clientset)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to create DeployConfig")
	}

	// For backward compatibility with predictor-based architecture
	deploymentMode := isvcutils.GetDeploymentMode(annotations, deployConfig)
	r.Log.Info("Inference service deployment mode ", "namespace", isvc.Namespace, "inference service", isvc.Name, "deployment mode", deploymentMode)

	// name of our custom finalizer
	finalizerName := "inferenceservice.finalizers"

	// examine DeletionTimestamp to determine if object is under deletion
	if isvc.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(isvc, finalizerName) {
			controllerutil.AddFinalizer(isvc, finalizerName)
			if err := r.Update(context.Background(), isvc); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(isvc, finalizerName) {
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(isvc, finalizerName)
			if err := r.Update(context.Background(), isvc); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Handle VirtualDeployment without actual reconciliation
	if deploymentMode == constants.VirtualDeployment {
		return r.handleVirtualDeployment(isvc)
	}

	// Abort early if the resolved deployment mode is Serverless, but Knative Services are not available
	if deploymentMode == constants.Serverless {
		if result, err := r.handleServerlessPrerequisites(isvc); err != nil {
			return result, err
		}
	}

	// Initialize status if not already initialized
	if isvc.Status.Components == nil {
		isvc.Status.Components = make(map[v1beta2.ComponentType]v1beta2.ComponentStatusSpec)
	}

	// Setup reconcilers
	r.Log.Info("Reconciling inference service", "apiVersion", isvc.APIVersion, "namespace", isvc.Namespace, "isvc", isvc.Name)
	isvcConfig, err := controllerconfig.NewInferenceServicesConfig(r.Clientset)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to create InferenceServicesConfig")
	}

	modelConfigReconciler := multimodelconfig.NewModelConfigReconciler(r.Client, r.Clientset, r.Scheme)
	result, err := modelConfigReconciler.Reconcile(ctx, isvc) // Added ctx
	if err != nil {
		return result, err
	}

	// Initialize ComponentBuilderFactory
	// Note: isvcConfig is created a few lines above inside the Reconcile function
	// for NewInferenceServicesConfig. We will use that existing isvcConfig.
	componentBuilderFactory := components.NewComponentBuilderFactory(r.Client, r.Clientset, r.Scheme, isvcConfig)

	// Determine which components to reconcile based on the spec
	var reconcilers []components.Component
	// TODO: covert predictor spec to engine spec, and remove predictor spec

	// Check if we should use the new architecture
	hasNewArchitectureConfig := isvc.Spec.Model != nil && (isvc.Spec.Engine != nil || isvc.Spec.Decoder != nil || isvc.Spec.Runtime != nil || isvc.Spec.Router != nil)
	var ingressDeploymentMode constants.DeploymentModeType
	if hasNewArchitectureConfig {
		// New architecture path
		r.Log.Info("Using new engine/decoder architecture", "namespace", isvc.Namespace, "inferenceService", isvc.Name)

		// Step 1: Reconcile model first
		baseModel, baseModelMeta, err := isvcutils.ReconcileBaseModel(r.Client, isvc)
		if err != nil {
			r.Log.Error(err, "Failed to reconcile base model", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "ModelReconcileError", err.Error())
			return reconcile.Result{}, err
		}

		// TODO, instead of failing, we should use isvc spec to create deployment
		// Step 2: Get rt spec (either specified or auto-selected based on model)
		rt, rtName, err := isvcutils.GetRuntimeForNewArchitecture(r.Client, isvc, baseModel)
		if err != nil {
			r.Log.Error(err, "Failed to get rt", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimeReconcileError", err.Error())
			return reconcile.Result{}, err
		}

		// Step 3: Merge rt and isvc specs to get final engine, decoder, and router specs
		mergedEngine, mergedDecoder, mergedRouter, err := isvcutils.MergeRuntimeSpecs(isvc, rt)
		if err != nil {
			r.Log.Error(err, "Failed to merge specs", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "MergeSpecsError", err.Error())
			return reconcile.Result{}, err
		}

		// Step 4: Determine deployment modes based on merged specs
		engineDeploymentMode, decoderDeploymentMode, routerDeploymentMode, err := isvcutils.DetermineDeploymentModes(mergedEngine, mergedDecoder, mergedRouter, rt)
		if err != nil {
			r.Log.Error(err, "Failed to determine deployment modes", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "DeploymentModeError", err.Error())
			return reconcile.Result{}, err
		}

		r.Log.Info("Determined deployment modes",
			"engine", engineDeploymentMode,
			"decoder", decoderDeploymentMode,
			"router", routerDeploymentMode,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)

		// If both engine and decoder exist, it's PD-disaggregated
		if mergedEngine != nil && mergedDecoder != nil {
			r.Log.Info("PD-disaggregated deployment detected", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
		}

		// Step 5a: Check for existing deployed components and handle deletions
		existingComponents, err := r.checkExistingComponents(ctx, isvc)
		if err != nil {
			r.Log.Error(err, "Failed to check existing components", "Name", isvc.Name)
			return reconcile.Result{}, err
		}

		// Create deletion reconcilers for components that exist but are not in current spec
		r.Log.Info("Checking for components to delete",
			"existing", existingComponents,
			"hasEngine", mergedEngine != nil,
			"hasDecoder", mergedDecoder != nil,
			"hasRouter", mergedRouter != nil,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)

		// Delete engine if it exists but is not in current spec
		if existingComponents.Engine && mergedEngine == nil {
			r.Log.Info("Creating engine deletion reconciler", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
			// Use RawDeployment mode for deletion since we just need to clean up resources
			engineDeletionReconciler := componentBuilderFactory.CreateEngineComponent(
				constants.RawDeployment, // Use consistent deployment mode for deletion
				baseModel,
				baseModelMeta,
				nil, // nil engine spec for deletion
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, engineDeletionReconciler)
		}

		// Delete decoder if it exists but is not in current spec
		if existingComponents.Decoder && mergedDecoder == nil {
			r.Log.Info("Creating decoder deletion reconciler", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
			decoderDeletionReconciler := componentBuilderFactory.CreateDecoderComponent(
				constants.RawDeployment, // Use consistent deployment mode for deletion
				baseModel,
				baseModelMeta,
				nil, // nil decoder spec for deletion
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, decoderDeletionReconciler)
		}

		// Delete router if it exists but is not in current spec
		if existingComponents.Router && mergedRouter == nil {
			r.Log.Info("Creating router deletion reconciler", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
			routerDeletionReconciler := componentBuilderFactory.CreateRouterComponent(
				constants.RawDeployment, // Use consistent deployment mode for deletion
				baseModel,
				baseModelMeta,
				nil, // nil router spec for deletion
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, routerDeletionReconciler)
		}

		// Step 5b: Create reconcilers based on merged specs
		if mergedEngine != nil {
			r.Log.Info("Creating engine reconciler",
				"deploymentMode", engineDeploymentMode,
				"namespace", isvc.Namespace,
				"inferenceService", isvc.Name)

			engineReconciler := componentBuilderFactory.CreateEngineComponent(
				engineDeploymentMode,
				baseModel,
				baseModelMeta,
				mergedEngine,
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, engineReconciler)
		}

		if mergedDecoder != nil {
			r.Log.Info("Creating decoder reconciler",
				"deploymentMode", decoderDeploymentMode,
				"namespace", isvc.Namespace,
				"inferenceService", isvc.Name)

			decoderReconciler := componentBuilderFactory.CreateDecoderComponent(
				decoderDeploymentMode,
				baseModel,
				baseModelMeta,
				mergedDecoder,
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, decoderReconciler)
		}

		// Add Router reconciler if merged router spec exists (using new v2 Router)
		if mergedRouter != nil {
			r.Log.Info("Creating router reconciler",
				"deploymentMode", routerDeploymentMode, // Using the determined router deployment mode
				"namespace", isvc.Namespace,
				"inferenceService", isvc.Name)

			routerReconciler := componentBuilderFactory.CreateRouterComponent(
				routerDeploymentMode, // Using the determined router deployment mode
				baseModel,
				baseModelMeta,
				mergedRouter, // Using the merged router spec instead of isvc.Spec.Router
				rt,
				rtName,
			)
			reconcilers = append(reconcilers, routerReconciler)
		}

		// Determine the correct ingress deployment mode using the same logic as ingress reconciler
		// but with the already-determined deployment modes to avoid inconsistency
		if mergedRouter != nil {
			ingressDeploymentMode = routerDeploymentMode
		} else if mergedDecoder != nil {
			ingressDeploymentMode = decoderDeploymentMode
		} else {
			ingressDeploymentMode = engineDeploymentMode
		}

		r.Log.Info("Determined ingress deployment mode",
			"ingressDeploymentMode", ingressDeploymentMode,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)
	} else {
		// Legacy architecture: use predictor with deployment mode from annotations/configmap
		r.Log.Info("Using legacy predictor architecture",
			"deploymentMode", deploymentMode,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)
		// TODO: change this to v2 predictor
		reconcilers = append(reconcilers, components.NewPredictor(r.Client, r.Clientset, r.Scheme, isvcConfig, deploymentMode))

		// For legacy architecture, ingress deployment mode is the same as the overall deployment mode
		ingressDeploymentMode = deploymentMode
	}

	// Step 6: Run all reconcilers (both regular and deletion reconcilers)
	for _, reconciler := range reconcilers {
		// Check if this component should exist based on current spec
		if !reconciler.ShouldExist(isvc) {
			// Component should be deleted
			r.Log.Info("Calling Delete on component that should not exist", 
				"component", fmt.Sprintf("%T", reconciler),
				"namespace", isvc.Namespace, 
				"inferenceService", isvc.Name)
			result, err := reconciler.Delete(isvc)
			if err != nil {
				r.Log.Error(err, "Failed to delete component", 
					"component", fmt.Sprintf("%T", reconciler),
					"namespace", isvc.Namespace, 
					"inferenceService", isvc.Name)
				return result, err
			}
			if result.Requeue || result.RequeueAfter > 0 {
				return result, nil
			}
		} else {
			// Component should exist, do normal reconciliation
			result, err := reconciler.Reconcile(isvc)
			if err != nil {
				r.Log.Error(err, "Failed to reconcile component", 
					"component", fmt.Sprintf("%T", reconciler),
					"namespace", isvc.Namespace, 
					"inferenceService", isvc.Name)
				return result, err
			}
			if result.Requeue || result.RequeueAfter > 0 {
				return result, nil
			}
		}
	}

	// Now reconcile ingress and external service after components have created their services
	ingressConfig, err := controllerconfig.NewIngressConfig(r.Clientset)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to create IngressConfig")
	}

	if hasNewArchitectureConfig {
		// New architecture: ingress uses the determined ingress deployment mode
		ingressReconciler := ingress.NewIngressReconciler(r.Client, r.Clientset, r.Scheme, ingressConfig, isvcConfig)
		r.Log.Info("Reconciling ingress for inference service", "isvc", isvc.Name)
		if err := ingressReconciler.(*ingress.IngressReconciler).ReconcileWithDeploymentMode(ctx, isvc, ingressDeploymentMode); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile ingress")
		}
	} else {
		// Legacy architecture: ingress uses the legacy deployment mode
		ingressReconciler := ingress.NewIngressReconciler(r.Client, r.Clientset, r.Scheme, ingressConfig, isvcConfig)
		r.Log.Info("Reconciling ingress for inference service", "isvc", isvc.Name)
		if err := ingressReconciler.(*ingress.IngressReconciler).ReconcileWithDeploymentMode(ctx, isvc, deploymentMode); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile ingress")
		}
	}

	// Reconcile external service - creates a service with the inference service name
	// when ingress is disabled to provide a stable endpoint
	externalServiceReconciler := external_service.NewExternalServiceReconciler(r.Client, r.Clientset, r.Scheme, ingressConfig)
	r.Log.Info("Reconciling external service for inference service", "isvc", isvc.Name)
	if err := externalServiceReconciler.Reconcile(ctx, isvc); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile external service")
	}

	// Set Status.Address for external service when ingress is disabled
	if ingressConfig.DisableIngressCreation {
		if err := r.setExternalServiceURL(ctx, isvc, ingressConfig); err != nil {
			r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
			return reconcile.Result{}, errors.Wrapf(err, "fails to set external service URL")
		}
	}

	if deploymentMode == constants.Serverless {
		componentList := []v1beta2.ComponentType{v1beta2.EngineComponent}
		r.StatusManager.PropagateCrossComponentStatus(&isvc.Status, componentList, v1beta2.RoutesReady)
		r.StatusManager.PropagateCrossComponentStatus(&isvc.Status, componentList, v1beta2.LatestDeploymentReady)
	}

	if err = r.updateStatus(isvc, deploymentMode); err != nil {
		r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceServiceReconciler) handleVirtualDeployment(isvc *v1beta2.InferenceService) (ctrl.Result, error) {
	// We directly set URL and inference service status to Ready in VirtualDeployment mode

	// Set URL across all Status components
	host := network.GetServiceHostname(isvc.Name, isvc.Namespace)
	openAIURL := knapis.HTTP(host)
	addressURL := &duckv1.Addressable{
		URL: &knapis.URL{
			Host:   host,
			Scheme: "http",
		},
	}
	isvc.Status.URL = openAIURL
	isvc.Status.Address = addressURL
	isvc.Status.Components = map[v1beta2.ComponentType]v1beta2.ComponentStatusSpec{
		v1beta2.PredictorComponent: {
			URL: openAIURL,
		},
	}

	isvc.Status.SetConditions(knapis.Conditions{{
		Type:               knapis.ConditionReady,
		Status:             v1.ConditionTrue,
		LastTransitionTime: knapis.VolatileTime{Inner: metav1.Now()},
		Reason:             "VirtualDeployment",
		Message:            "InferenceService is in VirtualDeployment mode",
	}})

	if err := r.updateStatus(isvc, constants.VirtualDeployment); err != nil {
		r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceServiceReconciler) handleServerlessPrerequisites(isvc *v1beta2.InferenceService) (ctrl.Result, error) {
	// Abort early if the resolved deployment mode is Serverless, but Knative Services are not available
	ksvcAvailable, err := utils.IsCrdAvailable(r.ClientConfig, knservingv1.SchemeGroupVersion.String(), constants.KnativeServiceKind)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ksvcAvailable {
		r.Recorder.Event(isvc, v1.EventTypeWarning, "ServerlessModeRejected",
			"It is not possible to use Serverless deployment mode when Knative Services are not available")
		return ctrl.Result{Requeue: false},
			reconcile.TerminalError(fmt.Errorf("the resolved deployment mode of InferenceService '%s' is Serverless, but Knative Serving is not available", isvc.Name))
	}

	return ctrl.Result{}, nil
}

func (r *InferenceServiceReconciler) updateStatus(desiredService *v1beta2.InferenceService, deploymentMode constants.DeploymentModeType) error {
	existingService := &v1beta2.InferenceService{}
	namespacedName := types.NamespacedName{Name: desiredService.Name, Namespace: desiredService.Namespace}
	if err := r.Get(context.TODO(), namespacedName, existingService); err != nil {
		return err
	}
	wasReady := inferenceServiceReadiness(existingService.Status)
	if inferenceServiceStatusEqual(existingService.Status, desiredService.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale, and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if err := r.Status().Update(context.TODO(), desiredService); err != nil {
		r.Log.Error(err, "Failed to update InferenceService status", "InferenceService", desiredService.Name)
		r.Recorder.Eventf(desiredService, v1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for InferenceService %q: %v", desiredService.Name, err)
		return errors.Wrapf(err, "fails to update InferenceService status")
	} else {
		// If there was a difference and there was no error.
		isReady := inferenceServiceReadiness(desiredService.Status)
		if wasReady && !isReady { // Moved to NotReady State
			r.Recorder.Eventf(desiredService, v1.EventTypeWarning, string(InferenceServiceNotReadyState),
				fmt.Sprintf("InferenceService [%v] is no longer Ready", desiredService.GetName()))
		} else if !wasReady && isReady { // Moved to Ready State
			r.Recorder.Eventf(desiredService, v1.EventTypeNormal, string(InferenceServiceReadyState),
				fmt.Sprintf("InferenceService [%v] is Ready", desiredService.GetName()))
		}
	}
	return nil
}

func inferenceServiceReadiness(status v1beta2.InferenceServiceStatus) bool {
	return status.Conditions != nil &&
		status.GetCondition(knapis.ConditionReady) != nil &&
		status.GetCondition(knapis.ConditionReady).Status == v1.ConditionTrue
}

func inferenceServiceStatusEqual(s1, s2 v1beta2.InferenceServiceStatus) bool {
	return equality.Semantic.DeepEqual(s1, s2)
}

func (r *InferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager, deployConfig *controllerconfig.DeployConfig, ingressConfig *controllerconfig.IngressConfig) error {
	r.ClientConfig = mgr.GetConfig()

	// NEW: Initialize StatusReconciler
	r.StatusManager = status.NewStatusReconciler()

	ksvcFound, err := utils.IsCrdAvailable(r.ClientConfig, knservingv1.SchemeGroupVersion.String(), constants.KnativeServiceKind)
	if err != nil {
		return err
	}

	vsFound, err := utils.IsCrdAvailable(r.ClientConfig, istioclientv1beta1.SchemeGroupVersion.String(), constants.IstioVirtualServiceKind)
	if err != nil {
		return err
	}

	rayFound, err := utils.IsCrdAvailable(r.ClientConfig, ray.SchemeGroupVersion.String(), constants.RayClusterKind)
	if err != nil {
		return err
	}

	lwsFound, err := utils.IsCrdAvailable(r.ClientConfig, lws.SchemeGroupVersion.String(), constants.LWSKind)
	if err != nil {
		return err
	}

	kedaFound, err := utils.IsCrdAvailable(r.ClientConfig, kedav1.SchemeGroupVersion.String(), constants.KEDAScaledObjectKind)
	if err != nil {
		return err
	}

	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.InferenceService{}).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Owns(&v1.PersistentVolume{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{})

	if ksvcFound {
		ctrlBuilder = ctrlBuilder.Owns(&knservingv1.Service{})
	} else {
		r.Log.Info("The InferenceService controller won't watch serving.knative.dev/v1/Service resources because the CRD is not available.")
	}

	if rayFound {
		ctrlBuilder = ctrlBuilder.Owns(&ray.RayCluster{})
	} else {
		r.Log.Info("The InferenceService controller won't watch ray.io/v1/RayCluster resources because the CRD is not available.")
	}

	if kedaFound {
		ctrlBuilder = ctrlBuilder.Owns(&kedav1.ScaledObject{})
	} else {
		r.Log.Info("The InferenceService controller won't watch keda.sh/v1/ScaledObject resources because the CRD is not available.")
	}

	if lwsFound {
		ctrlBuilder = ctrlBuilder.Owns(&lws.LeaderWorkerSet{})
	} else {
		r.Log.Info("The InferenceService controller won't watch leaderworkerset.x-k8s.io/v1/LeaderWorkerSet resources because the CRD is not available.")
	}

	if vsFound && !ingressConfig.DisableIstioVirtualHost {
		ctrlBuilder = ctrlBuilder.Owns(&istioclientv1beta1.VirtualService{})
	} else {
		r.Log.Info("The InferenceService controller won't watch networking.istio.io/v1beta1/VirtualService resources because the CRD is not available.")
	}

	return ctrlBuilder.Complete(r)
}

func (r *InferenceServiceReconciler) setExternalServiceURL(ctx context.Context, isvc *v1beta2.InferenceService, ingressConfig *controllerconfig.IngressConfig) error {
	// Get the external service
	externalService := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, externalService); err != nil {
		return err
	}

	// Set the URL and Address of the external service
	host := network.GetServiceHostname(externalService.Name, externalService.Namespace)
	openAIURL := knapis.HTTP(host)
	addressURL := &duckv1.Addressable{
		URL: &knapis.URL{
			Host:   host,
			Scheme: "http",
		},
	}
	isvc.Status.URL = openAIURL
	isvc.Status.Address = addressURL

	return nil
}

type existingComponents struct {
	Engine  bool
	Decoder bool
	Router  bool
}

func (r *InferenceServiceReconciler) checkExistingComponents(ctx context.Context, isvc *v1beta2.InferenceService) (existingComponents, error) {
	existing := existingComponents{}

	// Check status for existing components - this is more reliable than querying deployments
	if isvc.Status.Components != nil {
		// Check if engine component exists in status
		if _, hasEngine := isvc.Status.Components[v1beta2.EngineComponent]; hasEngine {
			existing.Engine = true
		}

		// Check if decoder component exists in status
		if _, hasDecoder := isvc.Status.Components[v1beta2.DecoderComponent]; hasDecoder {
			existing.Decoder = true
		}

		// Check if router component exists in status
		if _, hasRouter := isvc.Status.Components[v1beta2.RouterComponent]; hasRouter {
			existing.Router = true
		}
	}

	return existing, nil
}
