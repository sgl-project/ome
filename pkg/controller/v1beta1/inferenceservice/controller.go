package inferenceservice

import (
	"context"
	"fmt"

	"sigs.k8s.io/ome/pkg/acceleratorclassselector"

	policyv1 "k8s.io/api/policy/v1"

	"github.com/go-logr/logr"
	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/pkg/errors"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	knapis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"

	v1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/components"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/external_service"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	multimodelconfig "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/modelconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/traffic"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/status"
	isvcutils "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"sigs.k8s.io/ome/pkg/runtimeselector"
	"sigs.k8s.io/ome/pkg/utils"
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
// +kubebuilder:rbac:groups=apps,resources=controllerrevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.istio.io,resources=sidecars,verbs=get;list;watch;create;update;patch;delete
// Deprecated: this controller no longer reconciles or watches Knative Services. These rules are
// retained so that a controller image predating the Serverless removal - which still watches
// serving.knative.dev/v1 Services when that CRD is installed - keeps starting against a newer
// ClusterRole. Without them its informer never syncs and the manager aborts.
//
// TODO: remove these two rbac markers once no controller image that watches Knative Services is
// still running. Removing them is a breaking change for that image whenever the
// serving.knative.dev CRD is present in the cluster. Re-run `make manifests` afterwards to drop
// the rules from config/rbac/role.yaml and the generated Helm ClusterRole.
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services;services/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services/status,verbs=get;update;patch
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
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

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
	ClientConfig             *rest.Config
	Clientset                kubernetes.Interface
	Log                      logr.Logger
	Scheme                   *runtime.Scheme
	Recorder                 record.EventRecorder
	StatusManager            *status.StatusReconciler
	RuntimeSelector          runtimeselector.Selector
	AcceleratorClassSelector acceleratorclassselector.Selector
	// TrafficReconciler is the backend-policy reconciler.
	// Built from the active translator at controller startup; the
	// active translator is selected by traffic/factory.New based on
	// installed Gateway-implementation CRDs.
	TrafficReconciler *traffic.Reconciler
}

func (r *InferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the InferenceService instance
	isvc := &v1beta1.InferenceService{}
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

	// Determine the deployment mode for this inference service
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

	// Initialize status if not already initialized
	if isvc.Status.Components == nil {
		isvc.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
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

	var ingressDeploymentMode constants.DeploymentModeType

	// Step 1: Reconcile model first
	baseModel, baseModelMeta, err := isvcutils.ReconcileBaseModel(r.Client, isvc)
	if err != nil {
		r.Log.Error(err, "Failed to reconcile base model", "Name", isvc.Name)
		r.Recorder.Eventf(isvc, v1.EventTypeWarning, "ModelReconcileError", err.Error())
		return reconcile.Result{}, err
	}

	// Step 2: Get runtime spec (either specified or auto-selected based on model)
	var rt *v1beta1.ServingRuntimeSpec
	var rtName string
	userSpecifiedRuntime := false

	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		// Validate specified runtime
		rtName = isvc.Spec.Runtime.Name
		userSpecifiedRuntime = true

		if isvc.Spec.Runtime.AutoSync != nil && !*isvc.Spec.Runtime.AutoSync {
			// Pinning opt-in (autoSync=false): resolve the runtime spec from the
			// pinned ControllerRevision snapshot instead of the live runtime. The
			// pin helper handles first-reconcile create, drift detection, and the
			// ome.io/runtime-sync ack, and persists status itself when children
			// must be skipped.
			pin, perr := r.resolvePinnedRuntime(ctx, isvc)
			if perr != nil {
				r.Log.Error(perr, "Pin resolution failed", "runtime", rtName)
				r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimePinError", perr.Error())
				return reconcile.Result{}, perr
			}
			if pin.skipChildren {
				return reconcile.Result{}, nil
			}
			rt = pin.spec
		} else {
			// A lean InferenceService (no spec.model) can still name a
			// runtime explicitly; there is no model to validate against.
			if baseModel != nil {
				if err := r.RuntimeSelector.ValidateRuntime(ctx, rtName, baseModel, isvc); err != nil {
					// The operator named this runtime explicitly, so OME should not
					// block on the runtime's *declared* supportedModelFormats: a
					// generic runtime (e.g. sglang) can serve many architectures it
					// never enumerates. Downgrade a pure compatibility mismatch
					// (format / architecture / framework) to an advisory event and
					// proceed — the deliberate choice wins over the declaration.
					//
					// This mirrors the admission webhook (explicit-runtime compat
					// is advisory). Without the same downgrade here, the webhook
					// admits the ISVC but this reconcile hard-fails, so it never
					// gets pods.
					//
					// Everything else stays a hard error: a not-found / disabled
					// runtime or a malformed model genuinely cannot run.
					if runtimeselector.IsRuntimeCompatibilityError(err) {
						r.Log.Info("Runtime named explicitly; proceeding despite declared-format mismatch",
							"runtime", rtName, "model", isvc.Spec.Model.Name, "details", err.Error())
						r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimeCompatibilityAdvisory",
							"Runtime %s does not declare support for model %s (%v); proceeding because the runtime was named explicitly",
							rtName, isvc.Spec.Model.Name, err)
					} else {
						r.Log.Error(err, "Runtime validation failed", "runtime", rtName, "model", isvc.Spec.Model.Name)
						r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimeValidationError",
							"Runtime %s does not support model %s: %v", rtName, isvc.Spec.Model.Name, err)
						return reconcile.Result{}, err
					}
				}
			}

			// Get the runtime spec using selector
			rtSpec, _, err := r.RuntimeSelector.GetRuntime(ctx, rtName, isvc.Namespace, runtimeselector.RefKind(isvc.Spec.Runtime))
			if err != nil {
				if runtimeselector.IsRuntimeNotFoundError(err) {
					// A named-but-missing runtime is a permanent user-config error,
					// not a transient failure. Don't error+requeue (which hot-loops
					// and spams ERROR logs) and don't touch a currently-serving
					// ISVC — surface it as an advisory condition + event and wait
					// for the ServingRuntime watch to re-trigger once the runtime
					// exists.
					return r.markRuntimeUnresolved(isvc, deploymentMode, rtName, err)
				}
				r.Log.Error(err, "Failed to get runtime spec", "runtime", rtName)
				r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimeFetchError", err.Error())
				return reconcile.Result{}, err
			}
			r.clearRuntimeUnresolved(isvc)
			rt = rtSpec
		}
	} else if baseModel != nil {
		// Auto-select runtime
		selection, err := r.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc)
		if err != nil {
			if runtimeselector.IsNoRuntimeFoundError(err) || runtimeselector.IsRuntimeNotFoundError(err) {
				// No compatible runtime exists for the model yet — same
				// permanent-config, non-destructive treatment as the explicit
				// path: advisory condition + event, no error requeue, self-heals
				// when a matching runtime is created (watch re-triggers).
				return r.markRuntimeUnresolved(isvc, deploymentMode, isvc.Spec.Model.Name, err)
			}
			r.Log.Error(err, "Failed to auto-select runtime", "model", isvc.Spec.Model.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "RuntimeSelectionError",
				"Failed to find runtime for model %s: %v", isvc.Spec.Model.Name, err)
			return reconcile.Result{}, err
		}
		r.clearRuntimeUnresolved(isvc)
		rt = selection.Spec
		rtName = selection.Name
		r.Log.Info("Auto-selected runtime", "runtime", rtName, "model", isvc.Spec.Model.Name)
	} else {
		// No model and no runtime: nothing to select from. The admission
		// webhook rejects the shapes that cannot work; this is a defensive
		// guard for direct writes that bypass admission.
		err := fmt.Errorf("InferenceService must specify spec.runtime when spec.model is omitted")
		r.Log.Error(err, "Cannot reconcile", "Name", isvc.Name)
		r.Recorder.Event(isvc, v1.EventTypeWarning, "RuntimeSelectionError", err.Error())
		return reconcile.Result{}, err
	}

	// Step 3: Merge rt and isvc specs to get final engine, decoder, and router specs
	mergedEngine, mergedDecoder, mergedRouter, err := isvcutils.MergeRuntimeSpecs(isvc, rt, r.Log)
	if err != nil {
		r.Log.Error(err, "Failed to merge specs", "Name", isvc.Name)
		r.Recorder.Eventf(isvc, v1.EventTypeWarning, "MergeSpecsError", err.Error())
		return reconcile.Result{}, err
	}

	// Step 4: Determine deployment modes based on merged specs
	engineDeploymentMode, decoderDeploymentMode, routerDeploymentMode, err := isvcutils.DetermineDeploymentModes(mergedEngine, mergedDecoder, mergedRouter, rt, isvc.Spec.DeploymentMode)
	if err != nil {
		r.Log.Error(err, "Failed to determine deployment modes", "Name", isvc.Name)
		r.Recorder.Eventf(isvc, v1.EventTypeWarning, "DeploymentModeError", err.Error())
		return reconcile.Result{}, err
	}

	// If both engine and decoder exist, it's PD-disaggregated
	if mergedEngine != nil && mergedDecoder != nil {
		r.Log.Info("PD-disaggregated deployment detected", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
	}

	// Step 5: Create reconcilers based on merged specs
	if mergedEngine != nil {
		engineACObj, engineAcName, err := r.AcceleratorClassSelector.GetAcceleratorClass(ctx, isvc, rt, v1beta1.EngineComponent)
		if err != nil {
			r.Log.Error(err, "Failed to get accelerator class for engine component", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "AcceleratorClassError", "Failed to get accelerator class for engine: %v", err)
			return reconcile.Result{}, err
		}
		var engineAC *v1beta1.AcceleratorClassSpec
		if engineACObj == nil {
			r.Log.Info("Accelerator class not specified for engine component", "inferenceService", isvc.Name)
		} else {
			engineAC = &engineACObj.Spec
		}
		engineSupportedModelFormats := r.RuntimeSelector.GetSupportedModelFormat(ctx, rt, baseModel, userSpecifiedRuntime)
		r.Log.Info("Creating engine reconciler",
			"deploymentMode", engineDeploymentMode,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name,
			"acceleratorClass", engineAcName)

		engineReconciler := componentBuilderFactory.CreateEngineComponent(
			engineDeploymentMode,
			baseModel,
			baseModelMeta,
			mergedEngine,
			rt,
			rtName,
			engineSupportedModelFormats,
			engineAC,
			engineAcName,
		)
		reconcilers = append(reconcilers, engineReconciler)
	}

	if mergedDecoder != nil {
		decoderACObj, decoderAcName, err := r.AcceleratorClassSelector.GetAcceleratorClass(ctx, isvc, rt, v1beta1.DecoderComponent)
		if err != nil {
			r.Log.Error(err, "Failed to get accelerator class for decoder component", "Name", isvc.Name)
			r.Recorder.Eventf(isvc, v1.EventTypeWarning, "AcceleratorClassError", "Failed to get accelerator class for decoder: %v", err)
			return reconcile.Result{}, err
		}
		var decoderAC *v1beta1.AcceleratorClassSpec
		if decoderACObj == nil {
			r.Log.Info("Accelerator class not specified for decoder component", "inferenceService", isvc.Name)
		} else {
			decoderAC = &decoderACObj.Spec
		}
		decoderSupportedModelFormats := r.RuntimeSelector.GetSupportedModelFormat(ctx, rt, baseModel, userSpecifiedRuntime)
		r.Log.Info("Creating decoder reconciler",
			"deploymentMode", decoderDeploymentMode,
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name,
			"acceleratorClass", decoderAcName)

		decoderReconciler := componentBuilderFactory.CreateDecoderComponent(
			decoderDeploymentMode,
			baseModel,
			baseModelMeta,
			mergedDecoder,
			rt,
			rtName,
			decoderSupportedModelFormats,
			decoderAC,
			decoderAcName,
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

	// Step 6: Run all reconcilers
	for _, reconciler := range reconcilers {
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

	// Now reconcile ingress and external service after components have created their services
	ingressConfig, err := controllerconfig.NewIngressConfig(r.Clientset)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to create IngressConfig")
	}

	// Resolve ingress config with annotation overrides
	resolvedIngressConfig := isvcutils.ResolveIngressConfig(ingressConfig, isvc.Annotations)

	// New architecture: ingress uses the determined ingress deployment mode
	ingressReconciler := ingress.NewIngressReconciler(r.Client, r.Clientset, r.Scheme, resolvedIngressConfig, isvcConfig)
	r.Log.Info("Reconciling ingress for inference service", "isvc", isvc.Name)
	if err := ingressReconciler.(*ingress.IngressReconciler).ReconcileWithDeploymentMode(ctx, isvc, ingressDeploymentMode); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile ingress")
	}

	// Reconcile external service - creates a service with the inference service name
	// when ingress is disabled to provide a stable endpoint
	externalServiceReconciler := external_service.NewExternalServiceReconciler(r.Client, r.Clientset, r.Scheme, resolvedIngressConfig)
	r.Log.Info("Reconciling external service for inference service", "isvc", isvc.Name)
	if err := externalServiceReconciler.Reconcile(ctx, isvc); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile external service")
	}

	// Traffic management reconciler. Skipped when the InferenceService
	// declares no traffic intent (spec.traffic or ome.io/* annotation).
	// The reconciler invokes the active translator (chosen at startup
	// by the factory), applies the emitted backend policy resource,
	// and returns the TrafficStatus to write back. A nil reconciler
	// means the controller was set up without traffic management
	// (legitimate for tests / minimal configurations).
	if r.TrafficReconciler != nil {
		targetRoutes := traffic.ComputeTargetHTTPRoutes(isvc, mergedDecoder != nil, mergedRouter != nil)
		trafficStatus, err := r.TrafficReconciler.Reconcile(ctx, isvc, targetRoutes)
		if err != nil {
			r.Log.Error(err, "Failed to reconcile traffic policy",
				"namespace", isvc.Namespace, "inferenceService", isvc.Name,
				"translator", r.TrafficReconciler.TranslatorName())
			r.Recorder.Event(isvc, v1.EventTypeWarning, "TrafficReconcileError", err.Error())
			// Surface the status even on error so operators see the
			// TranslationFailed reason without waiting for the next
			// successful reconcile.
			isvc.Status.Traffic = trafficStatus
			return reconcile.Result{}, errors.Wrapf(err, "fails to reconcile traffic policy")
		}
		isvc.Status.Traffic = trafficStatus
	}

	// Set Status.Address for external service and add ingress disable annotation when ingress is disabled
	if resolvedIngressConfig.DisableIngressCreation {
		// Add annotation to InferenceService so cleanup logic knows to keep the external service
		if err := r.ensureIngressDisableAnnotation(isvc); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "fails to add ingress disable annotation")
		}
		if err := r.setExternalServiceURL(ctx, isvc, resolvedIngressConfig); err != nil {
			r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
			return reconcile.Result{}, errors.Wrapf(err, "fails to set external service URL")
		}
	}

	// Clean up resources for components that no longer exist
	// Move it under else condition to avoid an old deployment existing while its hpa is cleaned up
	// After migration, it will refactor.
	if err := r.cleanupRemovedComponents(ctx, isvc, mergedEngine, mergedDecoder, mergedRouter); err != nil {
		r.Log.Error(err, "Failed to cleanup removed components", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
		// Don't fail reconciliation on cleanup errors
	}

	// Clean up status for components that no longer exist
	if isvc.Status.Components != nil {
		r.Log.Info("Cleaning up component status",
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name,
			"mergedEngine", mergedEngine != nil,
			"mergedDecoder", mergedDecoder != nil,
			"mergedRouter", mergedRouter != nil,
			"statusComponents", len(isvc.Status.Components))

		if mergedEngine == nil {
			delete(isvc.Status.Components, v1beta1.EngineComponent)
			r.Log.Info("Deleted engine from status", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
		}
		if mergedDecoder == nil {
			delete(isvc.Status.Components, v1beta1.DecoderComponent)
			r.Log.Info("Deleted decoder from status", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
		}
		if mergedRouter == nil {
			delete(isvc.Status.Components, v1beta1.RouterComponent)
			r.Log.Info("Deleted router from status", "namespace", isvc.Namespace, "inferenceService", isvc.Name)
		}
	}

	if err = r.updateStatus(isvc, deploymentMode); err != nil {
		// A terminal status conflict is benign — requeue and re-reconcile
		// off fresh state instead of surfacing an ERROR-level failure.
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *InferenceServiceReconciler) handleVirtualDeployment(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	// We directly set URL and inference service status to Ready in VirtualDeployment mode

	// Honor the configured urlScheme rather than hardcoding http.
	ingressConfig, err := controllerconfig.NewIngressConfig(r.Clientset)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set URL across all Status components
	host := network.GetServiceHostname(isvc.Name, isvc.Namespace)
	openAIURL := &knapis.URL{
		Host:   host,
		Scheme: ingressConfig.UrlScheme,
	}
	addressURL := &duckv1.Addressable{
		URL: &knapis.URL{
			Host:   host,
			Scheme: ingressConfig.UrlScheme,
		},
	}
	isvc.Status.URL = openAIURL
	isvc.Status.Address = addressURL
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.EngineComponent: {
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
		// A terminal status conflict is benign — requeue and re-reconcile
		// off fresh state instead of surfacing an ERROR-level failure.
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		r.Recorder.Event(isvc, v1.EventTypeWarning, "InternalError", err.Error())
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

// markRuntimeUnresolved handles the "runtime cannot be resolved" case — a
// named runtime that doesn't exist, or no compatible runtime for the model.
// It is deliberately NON-DESTRUCTIVE: the reconcile bails before the child
// create/update/delete, so a currently-serving ISVC keeps running its
// existing pods. The problem is surfaced as the advisory RuntimeReady=False
// condition (NOT a dependent of the aggregate Ready, so a healthy ISVC stays
// Ready=True) plus a Warning event, then status is persisted and the
// reconcile returns WITHOUT an error — so the controller does not
// hot-requeue or spam ERROR logs. The ServingRuntime / ClusterServingRuntime
// watch re-triggers reconcile when a matching runtime is created, so this
// self-heals.
func (r *InferenceServiceReconciler) markRuntimeUnresolved(isvc *v1beta1.InferenceService, deploymentMode constants.DeploymentModeType, runtimeRef string, cause error) (reconcile.Result, error) {
	r.Log.Info("Runtime not found; awaiting runtime creation (ISVC left unchanged)",
		"runtime", runtimeRef, "InferenceService", isvc.Name)
	r.Recorder.Event(isvc, v1.EventTypeWarning, "RuntimeNotFound", cause.Error())
	isvc.Status.SetCondition(v1beta1.RuntimeReady, &knapis.Condition{
		Type:    v1beta1.RuntimeReady,
		Status:  v1.ConditionFalse,
		Reason:  "RuntimeNotFound",
		Message: cause.Error(),
	})
	if err := r.updateStatus(isvc, deploymentMode); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *InferenceServiceReconciler) clearRuntimeUnresolved(isvc *v1beta1.InferenceService) {
	if isvc.Status.GetCondition(v1beta1.RuntimeReady) == nil {
		return
	}
	isvc.Status.SetCondition(v1beta1.RuntimeReady, &knapis.Condition{
		Type:   v1beta1.RuntimeReady,
		Status: v1.ConditionTrue,
		Reason: "RuntimeResolved",
	})
}

func (r *InferenceServiceReconciler) updateStatus(desiredService *v1beta1.InferenceService, deploymentMode constants.DeploymentModeType) error {
	existingService := &v1beta1.InferenceService{}
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

func inferenceServiceReadiness(status v1beta1.InferenceServiceStatus) bool {
	return status.Conditions != nil &&
		status.GetCondition(knapis.ConditionReady) != nil &&
		status.GetCondition(knapis.ConditionReady).Status == v1.ConditionTrue
}

func inferenceServiceStatusEqual(s1, s2 v1beta1.InferenceServiceStatus) bool {
	return equality.Semantic.DeepEqual(s1, s2)
}

// ensureIngressDisableAnnotation adds the ome.io/ingress-disable-creation annotation to the InferenceService
// This annotation is used by the cleanup logic to determine if external service should be kept
func (r *InferenceServiceReconciler) ensureIngressDisableAnnotation(isvc *v1beta1.InferenceService) error {
	const ingressDisableAnnotation = "ome.io/ingress-disable-creation"

	// Check if annotation already exists
	if val, ok := isvc.Annotations[ingressDisableAnnotation]; ok && val == "true" {
		return nil
	}

	// Add annotation
	if isvc.Annotations == nil {
		isvc.Annotations = make(map[string]string)
	}
	isvc.Annotations[ingressDisableAnnotation] = "true"

	return nil
}

func (r *InferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager, deployConfig *controllerconfig.DeployConfig, ingressConfig *controllerconfig.IngressConfig) error {
	r.ClientConfig = mgr.GetConfig()

	// Register the spec.runtime.name cache index so runtime events
	// fan out to referencing ISVCs through the cache index instead of
	// scanning every cached InferenceService — see
	// isvcsReferencingRuntime.
	if err := registerISVCRuntimeNameIndex(context.Background(), mgr.GetFieldIndexer()); err != nil {
		return err
	}

	// NEW: Initialize StatusReconciler
	r.StatusManager = status.NewStatusReconciler()

	// Initialize RuntimeSelector
	r.RuntimeSelector = runtimeselector.New(mgr.GetClient())

	// Initialize AcceleratorClassSelector
	r.AcceleratorClassSelector = acceleratorclassselector.New(mgr.GetClient())

	vsFound, err := utils.IsCrdAvailable(r.ClientConfig, istioclientv1beta1.SchemeGroupVersion.String(), constants.IstioVirtualServiceKind)
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
		For(&v1beta1.InferenceService{}, builder.WithPredicates(isvcReconcileTriggerPredicate())).
		Owns(&appsv1.Deployment{}).
		Owns(&v1.Service{}).
		Owns(&v1.ConfigMap{}).
		Owns(&v1.PersistentVolume{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(ownedStatusIgnoringPredicate())).
		Owns(&policyv1.PodDisruptionBudget{})

	if kedaFound {
		ctrlBuilder = ctrlBuilder.Owns(&kedav1.ScaledObject{}, builder.WithPredicates(ownedStatusIgnoringPredicate()))
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

	// Gateway-API HTTPRoutes are owned by the ISVC, and IngressReady gates
	// on the route's parent (Gateway) programming status. Watch them so a
	// gateway flipping a route to programmed re-reconciles the ISVC —
	// otherwise IngressReady=False would persist until an unrelated event.
	// Scheme registration is conditional on EnableGatewayAPI (see
	// cmd/manager), so gate on both the scheme and the CRD being present.
	const httpRouteKind = "HTTPRoute"
	if mgr.GetScheme().Recognizes(gatewayapiv1.SchemeGroupVersion.WithKind(httpRouteKind)) {
		httpRouteFound, err := utils.IsCrdAvailable(r.ClientConfig, gatewayapiv1.SchemeGroupVersion.String(), httpRouteKind)
		if err != nil {
			return err
		}
		if httpRouteFound {
			ctrlBuilder = ctrlBuilder.Owns(&gatewayapiv1.HTTPRoute{})
		} else {
			r.Log.Info("The InferenceService controller won't watch gateway.networking.k8s.io/v1/HTTPRoute resources because the CRD is not available.")
		}
	}

	// Add watches for ServingRuntime and ClusterServingRuntime. Runtime
	// events fan out to EVERY ISVC referencing the runtime: autoSync=true
	// (float) ISVCs re-render from the live runtime and roll forward;
	// autoSync=false (pinned) ISVCs run drift detection and warn. Without
	// this fan-out a runtime change never triggers a float ISVC's reconcile,
	// so the edit silently fails to propagate — see isvcsReferencingRuntime.
	ctrlBuilder = ctrlBuilder.
		Watches(&v1beta1.ServingRuntime{},
			handler.EnqueueRequestsFromMapFunc(r.isvcsReferencingRuntime)).
		Watches(&v1beta1.ClusterServingRuntime{},
			handler.EnqueueRequestsFromMapFunc(r.isvcsReferencingRuntime))

	return ctrlBuilder.Complete(r)
}

func (r *InferenceServiceReconciler) setExternalServiceURL(ctx context.Context, isvc *v1beta1.InferenceService, ingressConfig *controllerconfig.IngressConfig) error {
	// Get the external service
	externalService := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, externalService); err != nil {
		return err
	}

	// Get the port from the external service
	var port int32 = constants.CommonISVCPort // default port
	if len(externalService.Spec.Ports) > 0 {
		port = externalService.Spec.Ports[0].Port
	}

	// Set the URL and Address of the external service with port
	host := network.GetServiceHostname(externalService.Name, externalService.Namespace)
	hostWithPort := fmt.Sprintf("%s:%d", host, port)

	// Honor the configured urlScheme rather than hardcoding http.
	isvc.Status.URL = &knapis.URL{
		Host:   hostWithPort,
		Scheme: ingressConfig.UrlScheme,
	}
	isvc.Status.Address = &duckv1.Addressable{
		URL: &knapis.URL{
			Host:   hostWithPort,
			Scheme: ingressConfig.UrlScheme,
		},
	}

	return nil
}

// referencing the runtime; with it MatchingFields narrows the list to
// the referencing ISVCs in O(matched). Registered on the manager cache
// in SetupWithManager (see registerISVCRuntimeNameIndex).
//
// The field name is an internal index identifier (a code constant), not
// a behavioral/user-facing value — no config surface is required.
const isvcRuntimeNameIndexField = "spec.runtime.name"

// isvcRuntimeNameIndexExtractor is the cache IndexerFunc registered for
// isvcRuntimeNameIndexField. It returns the ISVC's spec.runtime.name, or
// nil when the runtime ref (or its name) is absent — the unindexed
// match below skips those identically.
func isvcRuntimeNameIndexExtractor(obj client.Object) []string {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	if isvc.Spec.Runtime == nil || isvc.Spec.Runtime.Name == "" {
		return nil
	}
	return []string{isvc.Spec.Runtime.Name}
}

// isvcRuntimeUnresolvedIndexField indexes auto-select ISVCs (no
// spec.runtime.name) currently stuck with RuntimeReady=False. The name
// index above cannot cover them — they reference no runtime by name — so
// without this index a runtime created AFTER the ISVC never re-triggers
// its reconcile and markRuntimeUnresolved becomes an absorbing state.
// Internal index identifier, like isvcRuntimeNameIndexField.
const isvcRuntimeUnresolvedIndexField = "status.runtimeUnresolved"

// isvcRuntimeUnresolvedIndexValue is the single bucket value for
// isvcRuntimeUnresolvedIndexField (membership index, not a lookup key).
const isvcRuntimeUnresolvedIndexValue = "true"

func isvcRuntimeUnresolvedIndexExtractor(obj client.Object) []string {
	isvc, ok := obj.(*v1beta1.InferenceService)
	if !ok {
		return nil
	}
	if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
		// Named references fan out via the name index.
		return nil
	}
	cond := isvc.Status.GetCondition(v1beta1.RuntimeReady)
	if cond == nil || cond.Status != v1.ConditionFalse {
		return nil
	}
	return []string{isvcRuntimeUnresolvedIndexValue}
}

// registerISVCRuntimeNameIndex installs the runtime fan-out indexes on the
// supplied indexer (mgr.GetFieldIndexer()). Call once during manager
// setup, before Start, so isvcsReferencingRuntime resolves
// referencing ISVCs through the indexes instead of scanning every cached
// InferenceService.
func registerISVCRuntimeNameIndex(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &v1beta1.InferenceService{}, isvcRuntimeNameIndexField, isvcRuntimeNameIndexExtractor); err != nil {
		return err
	}
	return indexer.IndexField(ctx, &v1beta1.InferenceService{}, isvcRuntimeUnresolvedIndexField, isvcRuntimeUnresolvedIndexExtractor)
}

// isvcsReferencingRuntime returns reconcile requests for EVERY ISVC whose
// spec.runtime.name matches obj (respecting namespaced-SR scope), regardless
// of autoSync. Both float and pinned consumers must be re-reconciled on a
// runtime change:
//
//   - autoSync=true (default, "float"): the reconcile re-renders the pod
//     spec from the LIVE runtime, so a runtime edit rolls the ISVC forward.
//     Fanning these out is the whole point — without it a runtime change
//     never triggers the float ISVC's reconcile, so it silently fails to
//     propagate until some unrelated event happens to wake the ISVC. That
//     used to be masked by frequent incidental reconciles; once those were
//     trimmed (event-filter predicates, scoped informers), runtime edits
//     stopped reaching float ISVCs entirely.
//
//   - autoSync=false ("pinned"): the reconcile runs drift detection and
//     warns the operator that the runtime edit was rejected; it does not roll.
//
// NOTE: fanning out float ISVCs means a runtime edit rolls them. For
// RawDeployment that is a safe native rolling update; for OMENative it
// rolls per the Component's updateStrategy — an OMENative ISVC on
// RecreatePod with no rollout budget is recreated wholesale.
func (r *InferenceServiceReconciler) isvcsReferencingRuntime(ctx context.Context, obj client.Object) []reconcile.Request {
	runtimeName := obj.GetName()
	runtimeNamespace := obj.GetNamespace() // empty for cluster-scoped

	// Narrow to ISVCs whose spec.runtime.name matches via the field index;
	// the in-memory kind/namespace-scope filter below still applies.
	var isvcs v1beta1.InferenceServiceList
	if err := r.List(ctx, &isvcs, client.MatchingFields{isvcRuntimeNameIndexField: runtimeName}); err != nil {
		r.Log.Error(err, "fan-out runtime event: list InferenceServices failed")
		return nil
	}

	var reqs []reconcile.Request
	for i := range isvcs.Items {
		isvc := &isvcs.Items[i]
		if isvc.Spec.Runtime == nil || isvc.Spec.Runtime.Name != runtimeName {
			continue
		}
		// Namespaced SR only matches same-namespace ISVCs; a cluster-scoped
		// runtime (empty namespace) matches ISVCs in any namespace.
		if runtimeNamespace != "" && isvc.Namespace != runtimeNamespace {
			continue
		}
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: isvc.Namespace, Name: isvc.Name,
		}})
	}

	// Also wake auto-select ISVCs parked on RuntimeReady=False: the new or
	// edited runtime may be the compatible one they are waiting for, and no
	// name reference exists to fan them out through the index above.
	var unresolved v1beta1.InferenceServiceList
	if err := r.List(ctx, &unresolved, client.MatchingFields{isvcRuntimeUnresolvedIndexField: isvcRuntimeUnresolvedIndexValue}); err != nil {
		r.Log.Error(err, "fan-out runtime event: list unresolved InferenceServices failed")
		return reqs
	}
	for i := range unresolved.Items {
		isvc := &unresolved.Items[i]
		if runtimeNamespace != "" && isvc.Namespace != runtimeNamespace {
			continue
		}
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: isvc.Namespace, Name: isvc.Name,
		}})
	}
	return reqs
}
