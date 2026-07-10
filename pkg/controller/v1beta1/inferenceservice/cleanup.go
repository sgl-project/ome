package inferenceservice

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils"
)

// cleanupRemovedComponents deletes resources for components no longer specified in the spec.
func (r *InferenceServiceReconciler) cleanupRemovedComponents(
	ctx context.Context,
	isvc *v1beta1.InferenceService,
	engine *v1beta1.EngineSpec,
	decoder *v1beta1.DecoderSpec,
	router *v1beta1.RouterSpec,
) error {
	active := map[v1beta1.ComponentType]bool{
		v1beta1.EngineComponent:  engine != nil,
		v1beta1.DecoderComponent: decoder != nil,
		v1beta1.RouterComponent:  router != nil,
	}
	return r.deleteOrphanedResourcesByOwnerRef(ctx, isvc, active)
}

// deleteOrphanedResourcesByOwnerRef deletes resources owned by isvc that are not in activeComponents.
func (r *InferenceServiceReconciler) deleteOrphanedResourcesByOwnerRef(
	ctx context.Context,
	isvc *v1beta1.InferenceService,
	activeComponents map[v1beta1.ComponentType]bool,
) error {
	log := log.FromContext(ctx)

	selector := labels.Set{
		constants.InferenceServicePodLabelKey: isvc.Name,
	}.AsSelector()

	gvks, err := r.getAvailableResourceTypes()
	if err != nil {
		log.Error(err, "Failed to retrieve all available resource types, using core set")
		gvks = getCoreResourceTypes()
	}

	for _, gvk := range gvks {
		if err := r.cleanupResourcesOfType(ctx, gvk, isvc, selector, activeComponents); err != nil {
			log.Error(err, "Failed to cleanup resources of type", "gvk", gvk)
		}
	}
	return nil
}

// cleanupResourcesOfType deletes orphaned resources of a specific GVK.
func (r *InferenceServiceReconciler) cleanupResourcesOfType(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	isvc *v1beta1.InferenceService,
	selector labels.Selector,
	activeComponents map[v1beta1.ComponentType]bool,
) error {
	log := log.FromContext(ctx)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := r.List(ctx, list,
		client.InNamespace(isvc.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("list %s: %w", gvk.Kind, err)
	}

	for _, obj := range list.Items {
		if !r.isOwnedBy(&obj, isvc) {
			continue
		}
		component := v1beta1.ComponentType(obj.GetLabels()[constants.OMEComponentLabel])
		if component == "" || activeComponents[component] {
			continue
		}

		// Special handling for external service
		if component == "external-service" && gvk.Kind == "Service" {
			// External service should exist if ingress is disabled and there are active components
			if r.shouldKeepExternalService(isvc, activeComponents) {
				continue
			}
		}

		log.Info("Deleting orphaned resource", "gvk", gvk, "name", obj.GetName(), "component", component)
		if err := r.Delete(ctx, &obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s/%s: %w", gvk.Kind, obj.GetName(), err)
		}
	}
	return nil
}

// isOwnedBy returns true if obj is owned by isvc.
func (r *InferenceServiceReconciler) isOwnedBy(obj *unstructured.Unstructured, isvc *v1beta1.InferenceService) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "InferenceService" &&
			ref.APIVersion == v1beta1.SchemeGroupVersion.String() &&
			ref.Name == isvc.Name &&
			ref.UID == isvc.UID {
			return true
		}
	}
	return false
}

// shouldKeepExternalService determines if the external service should be kept based on active components
func (r *InferenceServiceReconciler) shouldKeepExternalService(isvc *v1beta1.InferenceService, activeComponents map[v1beta1.ComponentType]bool) bool {
	// Check if ingress creation is disabled via annotation
	if val, ok := isvc.Annotations["ome.io/ingress-disable-creation"]; ok && val == "true" {
		// Keep external service if any component that can serve traffic is active
		return activeComponents[v1beta1.RouterComponent] ||
			activeComponents[v1beta1.EngineComponent]
	}
	return false
}

// getAvailableResourceTypes returns known and discovered GVKs.
func (r *InferenceServiceReconciler) getAvailableResourceTypes() ([]schema.GroupVersionKind, error) {
	core := getCoreResourceTypes()

	optionals := []struct {
		gvk schema.GroupVersionKind
	}{
		{gvk: schema.GroupVersionKind{Group: "ray.io", Version: "v1", Kind: "RayCluster"}},
		{gvk: schema.GroupVersionKind{Group: "serving.knative.dev", Version: "v1", Kind: "Service"}},
		{gvk: schema.GroupVersionKind{Group: "leaderworkerset.x-k8s.io", Version: "v1", Kind: "LeaderWorkerSet"}},
		{gvk: schema.GroupVersionKind{Group: "keda.sh", Version: "v1alpha1", Kind: "ScaledObject"}},
		{gvk: schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1beta1", Kind: "VirtualService"}},
	}

	for _, res := range optionals {
		if r.ClientConfig == nil {
			continue
		}
		ok, err := utils.IsCrdAvailable(r.ClientConfig, res.gvk.GroupVersion().String(), res.gvk.Kind)
		if err != nil {
			log.Log.V(1).Info("Failed to check CRD", "gvk", res.gvk, "error", err)
			continue
		}
		if ok {
			core = append(core, res.gvk)
		}
	}

	return core, nil
}

// getCoreResourceTypes returns always-available Kubernetes resource types.
func getCoreResourceTypes() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"},
		{Group: "policy", Version: "v1", Kind: "PodDisruptionBudget"},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"},
		{Group: "", Version: "v1", Kind: "ServiceAccount"},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
	}
}

// isDeploymentReady checks if a deployment is ready by verifying the Available condition
func (r *InferenceServiceReconciler) isDeploymentReady(ctx context.Context, name, namespace string) (bool, error) {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Deployment doesn't exist yet, not ready
			return false, nil
		}
		return false, fmt.Errorf("failed to get deployment %s: %w", name, err)
	}

	// Check if deployment is available
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}
