package components

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/deployment"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/hpa"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/raw"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/rbac"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/status"
	isvcutils "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/utils"
)

var _ Component = &Router{}

// Router reconciles resources for the Router component of an InferenceService.
// It creates and manages the necessary Kubernetes resources (Deployment, Service, etc.)
// to route traffic to the Predictor pods of the InferenceService.
type Router struct {
	client                 client.Client
	clientset              kubernetes.Interface
	scheme                 *runtime.Scheme
	inferenceServiceConfig *controllerconfig.InferenceServicesConfig
	deploymentMode         constants.DeploymentModeType
	statusManager          *status.StatusReconciler
	Log                    logr.Logger
}

// NewRouter creates a new Router component instance with the provided dependencies.
// It returns an implementation of the Component interface that can reconcile Router resources.
func NewRouter(
	client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	inferenceServiceConfig *controllerconfig.InferenceServicesConfig,
	deploymentMode constants.DeploymentModeType,
) Component {
	return &Router{
		client:                 client,
		clientset:              clientset,
		scheme:                 scheme,
		inferenceServiceConfig: inferenceServiceConfig,
		deploymentMode:         deploymentMode,
		statusManager:          status.NewStatusReconciler(),
		Log:                    ctrl.Log.WithName("RouterReconciler"),
	}
}

// Reconcile observes the Router component of an InferenceService and attempts to drive
// the status towards the desired state by creating or updating the necessary resources.
// It returns a controller result and an error, if any.
func (r *Router) Reconcile(isvc *v1beta1.InferenceService) (ctrl.Result, error) {
	// Check if router is defined in the InferenceService spec
	if isvc.Spec.Router == nil {
		// Router is not defined, no need to reconcile
		return ctrl.Result{}, nil
	}

	r.Log.Info("Reconciling Router", "InferenceService", isvc.Name, "Namespace", isvc.Namespace)

	// Check if predictor is ready by looking at its URL in the status
	predictorStatus, predictorOk := isvc.Status.Components[v1beta1.PredictorComponent]
	if !predictorOk || predictorStatus.URL == nil {
		r.Log.Info("Router waiting for Predictor component to be ready (URL not populated in status)", "isvc", isvc.Name)
		// Don't treat this as an error, but requeue shortly to check again.
		// The InferenceService controller will eventually update the status when the predictor is ready.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	r.Log.Info("Predictor component is ready, proceeding with Router reconciliation", "isvc", isvc.Name, "PredictorURL", predictorStatus.URL.String())

	// Get predictor reference for router to connect to
	predictorRef, err := r.getPredictorReference(isvc)
	if err != nil {
		r.Log.Error(err, "Failed to get predictor reference")
		return ctrl.Result{}, errors.Wrap(err, "failed to get predictor reference")
	}

	// Reconcile and validate runtime
	sRuntime, runtimeName, result, err := r.getRuntime(isvc)
	if err != nil {
		return result, err
	}

	// Reconcile object metadata and pod spec
	objectMeta, result, err := r.reconcileObjectMeta(isvc, sRuntime, runtimeName, predictorRef)
	if err != nil {
		r.Log.Error(err, "Failed to reconcile object metadata")
		return result, errors.Wrap(err, "failed to reconcile object metadata")
	}

	podSpec, result, err := r.reconcilePodSpec(isvc, sRuntime.RouterConfig)
	if err != nil {
		r.Log.Error(err, "Failed to reconcile pod spec")
		return result, errors.Wrap(err, "failed to reconcile pod spec")
	}

	r.Log.Info("Resolved podSpec for router",
		"inferenceServiceName", isvc.Name,
		"namespace", isvc.Namespace)

	// Reconcile deployment for the router
	if result, err := r.reconcileDeployment(isvc, objectMeta, &podSpec); err != nil {
		r.Log.Error(err, "Failed to reconcile deployment")
		return result, errors.Wrap(err, "failed to reconcile deployment")
	}

	// Update the router status
	if err := r.updateRouterStatus(isvc, objectMeta); err != nil {
		r.Log.Error(err, "Failed to update router status")
		return ctrl.Result{}, errors.Wrap(err, "failed to update router status")
	}

	r.Log.Info("Successfully reconciled Router", "InferenceService", isvc.Name)
	return ctrl.Result{}, nil
}

// reconcileDeployment manages the deployment logic for the router.
// Currently, only raw deployments (standard Kubernetes resources) are supported.
func (r *Router) reconcileDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *corev1.PodSpec) (ctrl.Result, error) {
	// For now, we only support raw deployment for the router
	return r.reconcileRawDeployment(isvc, objectMeta, podSpec)
}

// reconcileRawDeployment creates and manages standard Kubernetes resources (Deployment, Service, HPA)
// for the router component using dedicated reconcilers. It propagates the status to the InferenceService.
func (r *Router) reconcileRawDeployment(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta, podSpec *corev1.PodSpec) (ctrl.Result, error) {
	componentExt := isvc.Spec.Router // Router uses its own spec extension
	if componentExt == nil {
		// Should not happen if Reconcile check is done properly, but good practice
		return ctrl.Result{}, errors.New("router spec extension is nil in reconcileRawDeployment")
	}

	// 0. Reconcile RBAC resources (ServiceAccount, Role, RoleBinding)
	rbacReconciler := rbac.NewRBACReconciler(r.client, r.scheme, objectMeta, v1beta1.RouterComponent)
	if err := rbacReconciler.Reconcile(); err != nil {
		r.Log.Error(err, "Failed to reconcile RBAC resources for router")
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile router RBAC resources")
	}

	// Set the service account in the pod spec
	serviceAccountName := rbacReconciler.GetServiceAccountName()
	podSpec.ServiceAccountName = serviceAccountName
	r.Log.Info("Using service account for router", "serviceAccount", serviceAccountName)

	// 1. Reconcile Deployment
	// Corrected: Pass the embedded ComponentExtensionSpec
	deploymentReconciler := deployment.NewDeploymentReconciler(r.client, r.scheme, objectMeta, &componentExt.ComponentExtensionSpec, podSpec)
	deploymentResult, err := deploymentReconciler.Reconcile()
	if err != nil {
		r.Log.Error(err, "Failed to reconcile Deployment for router")
		// Removed call to undefined updateRouterTransitionStatus
		// isvc status will be updated via PropagateRawStatus or later checks
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile router Deployment")
	}
	// Assuming DeploymentReconciler sets owner references internally if creating

	// Check for entrypoint-component annotation using utility function
	routerAsEntrypoint := isvcutils.IsEntrypointRouter(isvc.Annotations)
	if routerAsEntrypoint {
		r.Log.Info("Router is the entrypoint", "isvc", isvc.Name)
		// Create service metadata
		// If router is the entrypoint, use inference service name for router service
		serviceObjectMeta := metav1.ObjectMeta{
			Name:        objectMeta.Name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(isvc, v1beta1.SchemeGroupVersion.WithKind("InferenceService")),
			},
		}

		// If router is the entrypoint, it should use the inference service name for its service
		if routerAsEntrypoint {
			serviceObjectMeta.Name = isvc.Name
		}

		r.Log.Info("Reconciling Router Service",
			"isvc", isvc.Name,
			"routerAsEntrypoint", routerAsEntrypoint,
			"serviceName", serviceObjectMeta.Name)

		// 2. Reconcile Service
		// Use the selector from the reconciled deployment for the service
		// Always pass false for router
		serviceSelector := deploymentResult.Spec.Selector.MatchLabels
		serviceReconciler := service.NewServiceReconciler(r.client, r.scheme, serviceObjectMeta, &componentExt.ComponentExtensionSpec, podSpec, serviceSelector, false)
		_, err = serviceReconciler.Reconcile() // We don't need the resulting service obj directly here
		if err != nil {
			r.Log.Error(err, "Failed to reconcile Service for router")
			// Removed call to undefined updateRouterTransitionStatus
			return ctrl.Result{}, errors.Wrap(err, "failed to reconcile router Service")
		}
	}

	// 3. Reconcile HPA (if scaling is configured)
	// Corrected: Pass the embedded ComponentExtensionSpec
	if componentExt.MinReplicas != nil && componentExt.MaxReplicas > 0 && *componentExt.MinReplicas != componentExt.MaxReplicas {
		hpaReconciler := hpa.NewHPAReconciler(r.client, r.scheme, objectMeta, &componentExt.ComponentExtensionSpec)
		_, err = hpaReconciler.Reconcile() // We don't need the resulting HPA obj directly here
		if err != nil {
			r.Log.Error(err, "Failed to reconcile HPA for router")
			// Removed call to undefined updateRouterTransitionStatus
			return ctrl.Result{}, errors.Wrap(err, "failed to reconcile router HPA")
		}
		// HPA reconciler handles owner references
	} else {
		// Ensure HPA is deleted if scaling is disabled or invalid
		r.Log.Info("Scaling is not configured or minReplicas==maxReplicas, ensuring HPA is deleted for router", "name", objectMeta.Name)
		// Corrected: Manually delete HPA as reconciler doesn't have Delete method
		hpaToDelete := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectMeta.Name,
				Namespace: objectMeta.Namespace,
			},
		}
		if err := r.client.Delete(context.Background(), hpaToDelete); err != nil && !apierrors.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete HPA for router")
			// Don't block reconciliation for HPA deletion failure, log it
		}
	}

	// 4. Propagate Status
	// Get the URL (use existing logic which requires clientset)
	url, err := raw.CreateRawURL(r.clientset, objectMeta)
	if err != nil {
		// Log error but don't fail the reconciliation just because URL generation failed
		r.Log.Error(err, "Failed to create raw URL for router status, status URL might be missing")
		// Continue without URL
	}

	// Propagate the status to the InferenceService using the reconciled deployment and generated URL
	r.statusManager.PropagateRawStatus(&isvc.Status, v1beta1.RouterComponent, deploymentResult, url)

	// Check if the component became ready
	// Corrected: Use constant correctly
	if !isvc.Status.IsConditionReady(v1beta1.RouterReady) {
		r.Log.Info("Router component is not ready", "deployment", deploymentResult.Name)
		// If not ready, we might want to requeue, PropagateRawStatus might set conditions that imply this.
		// For now, return no error and let the controller requeue based on Deployment/Service/HPA watches.
	} else {
		r.Log.Info("Router component is ready", "deployment", deploymentResult.Name)
	}

	// Update the overall InferenceService status condition for the router
	// Use the correct constant v1beta1.RouterReady
	isvc.Status.SetCondition(v1beta1.RouterReady, &knapis.Condition{
		Type:   v1beta1.RouterReady,
		Status: corev1.ConditionTrue, // Assuming success for now, adjust based on actual checks
	})

	return ctrl.Result{}, nil
}

// updateRouterStatus updates the router component status in the InferenceService status.
// This includes URL, address, and overall component status.
func (r *Router) updateRouterStatus(isvc *v1beta1.InferenceService, objectMeta metav1.ObjectMeta) error {
	// If Status.Components is not initialized, initialize it
	if isvc.Status.Components == nil {
		isvc.Status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
	}

	// Create or update the router URL if needed
	if _, exists := isvc.Status.Components[v1beta1.RouterComponent]; !exists {
		// Initialize component status if it doesn't exist
		isvc.Status.Components[v1beta1.RouterComponent] = v1beta1.ComponentStatusSpec{}
	}

	// Create a URL for the router if needed
	if isvc.Status.Components[v1beta1.RouterComponent].URL == nil {
		url, err := raw.CreateRawURL(r.clientset, objectMeta)
		if err != nil {
			return errors.Wrap(err, "failed to create router URL")
		}

		// Update the status - we have to create a new map entry with the updated URL
		componentStatus := isvc.Status.Components[v1beta1.RouterComponent]
		componentStatus.URL = url
		isvc.Status.Components[v1beta1.RouterComponent] = componentStatus
	}

	return nil
}

// getPredictorReference gets the reference to the predictor component for the router to connect to.
// It returns the appropriate service name or selector for directing traffic to predictor pods.
func (r *Router) getPredictorReference(isvc *v1beta1.InferenceService) (string, error) {
	// Use a standard naming convention for the predictor
	// TODO: Dynamically generate the predictor name based on the spec
	predictorName := fmt.Sprintf("%s-%s", isvc.Name, v1beta1.PredictorComponent)

	// Otherwise return the service name if the predictor should have its own service
	return predictorName, nil
}

// reconcileObjectMeta reconciles the object metadata for the router component.
// It processes annotations and labels, and builds the complete ObjectMeta.
func (r *Router) reconcileObjectMeta(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, runtimeName string, predictorRef string) (metav1.ObjectMeta, ctrl.Result, error) {
	// Process router annotations
	annotations, err := r.processAnnotations(isvc, runtimeName, predictorRef)
	if err != nil {
		return metav1.ObjectMeta{}, ctrl.Result{}, errors.Wrap(err, "failed to process annotations")
	}

	// Process router labels
	labels, err := r.processLabels(isvc, predictorRef)
	if err != nil {
		return metav1.ObjectMeta{}, ctrl.Result{}, errors.Wrap(err, "failed to process labels")
	}

	// Build the router name
	routerName := fmt.Sprintf("%s-%s", isvc.Name, v1beta1.RouterComponent)

	// Build and return the object metadata
	objectMeta := r.buildObjectMeta(isvc, sRuntime, routerName, annotations, labels)
	return objectMeta, ctrl.Result{}, nil
}

// processAnnotations processes and merges annotations for the router component.
// It adds router-specific annotations and includes any custom annotations from the spec.
func (r *Router) processAnnotations(isvc *v1beta1.InferenceService, runtimeName string, predictorRef string) (map[string]string, error) {
	annotations := make(map[string]string)

	// Add router-specific annotations
	annotations[constants.InferenceServicePodLabelKey] = isvc.Name
	annotations[constants.ServingRuntimeKeyName] = runtimeName
	annotations[constants.OMEAPIGroupName+"/predictor-ref"] = predictorRef

	// Add router-specific config if provided in the spec
	if isvc.Spec.Router.Config != nil {
		for k, v := range isvc.Spec.Router.Config {
			annotations[fmt.Sprintf("%s/config-%s", constants.OMEAPIGroupName, k)] = v
		}
	}

	// Merge with user-provided annotations
	return utils.Union(annotations, isvc.Spec.Router.Annotations), nil
}

// processLabels processes and merges labels for the router component.
// It adds router-specific labels and includes any custom labels from the spec.
func (r *Router) processLabels(isvc *v1beta1.InferenceService, predictorRef string) (map[string]string, error) {
	// Create base labels for the router component
	baseLabels := map[string]string{
		constants.InferenceServicePodLabelKey:        isvc.Name,
		constants.KServiceComponentLabel:             string(v1beta1.RouterComponent),
		constants.OMEAPIGroupName + "/predictor-ref": predictorRef,
	}

	// Create label map using utils.Union to merge labels from multiple sources
	labels := utils.Union(
		baseLabels,
		isvc.Labels,
		isvc.Spec.Router.Labels,
	)

	return labels, nil
}

// buildObjectMeta builds the object metadata for the router component.
// It creates a new ObjectMeta with the provided name, labels, and annotations,
// and sets the controller reference to the InferenceService.
func (r *Router) buildObjectMeta(isvc *v1beta1.InferenceService, sRuntime v1beta1.ServingRuntimeSpec, routerName string, annotations, labels map[string]string) metav1.ObjectMeta {
	objectMeta := metav1.ObjectMeta{
		Name:        routerName,
		Namespace:   isvc.Namespace,
		Labels:      labels,
		Annotations: annotations,
	}

	// Set owner reference to establish parent-child relationship for garbage collection
	if err := controllerutil.SetControllerReference(isvc, &objectMeta, r.scheme); err != nil {
		r.Log.Error(err, "Failed to set controller reference for router")
	}

	return objectMeta
}

// It builds a PodSpec based on the RouterSpec and adds necessary environment variables
// and arguments like --worker-urls based on predictor discovery.
func (r *Router) reconcilePodSpec(isvc *v1beta1.InferenceService, routerConfig *v1beta1.RouterSpec) (corev1.PodSpec, ctrl.Result, error) {
	// Check if RouterSpec is defined
	if isvc.Spec.Router == nil {
		// Should not happen based on initial Reconcile check, but good practice
		return corev1.PodSpec{}, ctrl.Result{}, errors.New("router spec is nil in reconcilePodSpec")
	}

	routerContainerIdx := isvcutils.GetContainerIndex(routerConfig.Containers, constants.RouterContainerName)
	container, err := r.createMergedContainer(isvc, routerConfig, routerContainerIdx)
	if err != nil {
		return corev1.PodSpec{}, ctrl.Result{}, err
	}

	// Inject worker discovery arguments
	if err := r.updateWorkerDiscoveryArgs(isvc, container); err != nil {
		r.Log.Error(err, "Failed to inject worker discovery arguments")
		// We log the error but don't fail reconciliation to maintain backward compatibility
	}

	podSpec, err := r.createMergedPodSpec(isvc, routerConfig)
	if err != nil {
		return corev1.PodSpec{}, ctrl.Result{}, err
	}

	r.updatePodSpec(isvc, routerConfig, routerContainerIdx, container, &podSpec)
	// Ensure the pod spec has necessary labels/annotations if not already present
	if podSpec.RestartPolicy == "" {
		podSpec.RestartPolicy = corev1.RestartPolicyAlways // Example default
	}

	return podSpec, ctrl.Result{}, nil
}

// updateWorkerDiscoveryArgs configures the router container to use Kubernetes service discovery
// to find and connect to predictor pods automatically.
func (r *Router) updateWorkerDiscoveryArgs(isvc *v1beta1.InferenceService, container *corev1.Container) error {
	// 1. Define predictor selector
	selectorLabels := map[string]string{
		constants.InferenceServicePodLabelKey: isvc.Name,
		constants.KServiceComponentLabel:      string(v1beta1.PredictorComponent),
	}

	// 2. Convert selector to string format required by the router (key1=value1 key2=value2)
	selectorStr := []string{}
	for k, v := range selectorLabels {
		selectorStr = append(selectorStr, fmt.Sprintf("%s=%s", k, v))
	}

	// 3. Check if service discovery arguments already exist
	hasSelectorArg := false
	hasServiceDiscoveryPortArg := false
	hasServiceDiscoveryNamespaceArg := false

	for _, existingArg := range container.Args {
		if strings.HasPrefix(existingArg, "--selector") {
			hasSelectorArg = true
		} else if strings.HasPrefix(existingArg, "--service-discovery-port") {
			hasServiceDiscoveryPortArg = true
		} else if strings.HasPrefix(existingArg, "--service-discovery-namespace") {
			hasServiceDiscoveryNamespaceArg = true
		}
	}

	// 4. Add service discovery arguments if they don't exist
	// Add selector argument
	if !hasSelectorArg && len(selectorStr) > 0 {
		// Sort selector strings to ensure deterministic order
		sort.Strings(selectorStr)
		isvcutils.AppendContainerArgs(container, &[]string{"--selector"})
		isvcutils.AppendContainerArgs(container, &selectorStr)
	}

	// Add service discovery port argument if it doesn't exist
	if !hasServiceDiscoveryPortArg {
		portArg := fmt.Sprintf("--service-discovery-port=%d", constants.CommonISVCPort)
		isvcutils.AppendContainerArgs(container, &[]string{portArg})
	}

	// Add service discovery namespace argument if it doesn't exist
	if !hasServiceDiscoveryNamespaceArg {
		namespaceArg := fmt.Sprintf("--service-discovery-namespace=%s", isvc.Namespace)
		isvcutils.AppendContainerArgs(container, &[]string{namespaceArg})
	}

	return nil
}

// getRuntime retrieves the serving runtime for the predictor.
func (r *Router) getRuntime(isvc *v1beta1.InferenceService) (v1beta1.ServingRuntimeSpec, string, ctrl.Result, error) {
	if isvc.Spec.Predictor.Model.Runtime != nil {
		runtimeSpec, result, err := r.getSpecifiedRuntime(isvc)
		return runtimeSpec, *isvc.Spec.Predictor.Model.Runtime, result, err
	}
	return v1beta1.ServingRuntimeSpec{}, "", ctrl.Result{}, fmt.Errorf("no runtime specified with predictor within inference service %s/%s", isvc.Namespace, isvc.Name)
}

// getSpecifiedRuntime retrieves the specified runtime for the predictor.
func (r *Router) getSpecifiedRuntime(isvc *v1beta1.InferenceService) (v1beta1.ServingRuntimeSpec, ctrl.Result, error) {
	rt, err := isvcutils.GetServingRuntime(r.client, *isvc.Spec.Predictor.Model.Runtime, isvc.Namespace)
	if err != nil {
		r.Log.Error(err, "Failed to get serving runtime")
		return v1beta1.ServingRuntimeSpec{}, ctrl.Result{}, err
	}
	return *rt, ctrl.Result{}, nil
}

// updateModelTransitionStatus updates the model transition status for the predictor.
func (r *Router) updateModelTransitionStatus(isvc *v1beta1.InferenceService, reason v1beta1.FailureReason, message string) {
	r.statusManager.UpdateModelTransitionStatus(&isvc.Status, v1beta1.InvalidSpec, &v1beta1.FailureInfo{
		Reason:  reason,
		Message: message,
	})
}

// createMergedContainer merges the runtime and router containers.
func (r *Router) createMergedContainer(isvc *v1beta1.InferenceService, routerConfig *v1beta1.RouterSpec, routerContainerIdx int) (*corev1.Container, error) {
	if isvc.Spec.Router != nil && isvc.Spec.Router.Containers != nil && len(isvc.Spec.Router.Containers) > routerContainerIdx {
		container, err := isvcutils.MergeRuntimeContainers(&routerConfig.Containers[routerContainerIdx], &isvc.Spec.Router.Containers[routerContainerIdx])
		if err != nil {
			r.updateModelTransitionStatus(isvc, v1beta1.InvalidRouterSpec, "Failed to merge router container args")
			return nil, errors.Wrapf(err, "failed to merge router container args")
		}

		if err = isvcutils.ReplacePlaceholders(container, isvc.ObjectMeta); err != nil {
			r.updateModelTransitionStatus(isvc, v1beta1.InvalidRouterSpec, "Failed to replace placeholders in serving router Container")
			return nil, errors.Wrapf(err, "failed to replace placeholders in serving router Container")
		}

		isvcutils.UpdateImageTag(container, isvc.Spec.Predictor.Model.RuntimeVersion, isvc.Spec.Predictor.Model.Runtime)
		return container, nil
	} else {
		return &routerConfig.Containers[routerContainerIdx], nil
	}
}

// createMergedPodSpec merges the runtime router config and router pod specs.
func (r *Router) createMergedPodSpec(isvc *v1beta1.InferenceService, routerSpec *v1beta1.RouterSpec) (corev1.PodSpec, error) {
	mergedPodSpec, err := isvcutils.MergeRouterPodSpec(routerSpec, &isvc.Spec.Router.PodSpec)
	if err != nil {
		r.updateModelTransitionStatus(isvc, v1beta1.InvalidRouterSpec, "Failed to get router PodSpec")
		return corev1.PodSpec{}, errors.Wrapf(err, "failed to consolidate router PodSpecs")
	}
	return *mergedPodSpec, nil
}

// updatePodSpec updates the pod spec for the router.
func (r *Router) updatePodSpec(isvc *v1beta1.InferenceService,
	routerConfig *v1beta1.RouterSpec,
	routerContainerIdx int,
	container *corev1.Container,
	podSpec *corev1.PodSpec,
) {
	// Update containers by inserting the custom container and keeping the other router containers
	podSpec.Containers = append([]corev1.Container{*container}, routerConfig.Containers[:routerContainerIdx]...)
	podSpec.Containers = append(podSpec.Containers, routerConfig.Containers[routerContainerIdx+1:]...)

	r.Log.Info("PodSpec updated", "inference service", isvc.Name, "namespace", isvc.Namespace)
}
