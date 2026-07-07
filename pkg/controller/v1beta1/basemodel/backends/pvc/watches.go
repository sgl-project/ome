package pvc

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

// CreatePhasePredicate fires on PVC create, delete, and phase
// transitions (e.g. Pending → Bound). Drops same-phase updates to
// avoid pointless re-reconciles on PVC annotation churn.
func CreatePhasePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPVC, ok := e.ObjectOld.(*corev1.PersistentVolumeClaim)
			if !ok {
				return false
			}
			newPVC, ok := e.ObjectNew.(*corev1.PersistentVolumeClaim)
			if !ok {
				return false
			}
			return oldPVC.Status.Phase != newPVC.Status.Phase
		},
		DeleteFunc: func(_ event.DeleteEvent) bool { return true },
	}
}

// MapToBaseModels enqueues BaseModels in the PVC's namespace that
// reference this PVC by URI.
func MapToBaseModels(ctx context.Context, kubeClient client.Client, log logr.Logger, obj client.Object) []reconcile.Request {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}
	bms := &v1beta1.BaseModelList{}
	if err := kubeClient.List(ctx, bms, client.InNamespace(pvc.Namespace)); err != nil {
		log.Error(err, "Failed to list BaseModels for PVC mapping", "pvc", pvc.Name, "namespace", pvc.Namespace)
		return nil
	}
	var reqs []reconcile.Request
	for i := range bms.Items {
		bm := &bms.Items[i]
		if !pvcSpecMatches(&bm.Spec, pvc.Name, pvc.Namespace, false) {
			continue
		}
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: bm.Name, Namespace: bm.Namespace},
		})
	}
	return reqs
}

// MapToClusterBaseModels enqueues ClusterBaseModels whose pvc://
// URI references this PVC's (namespace, name).
func MapToClusterBaseModels(ctx context.Context, kubeClient client.Client, log logr.Logger, obj client.Object) []reconcile.Request {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return nil
	}
	cbms := &v1beta1.ClusterBaseModelList{}
	if err := kubeClient.List(ctx, cbms); err != nil {
		log.Error(err, "Failed to list ClusterBaseModels for PVC mapping", "pvc", pvc.Name, "namespace", pvc.Namespace)
		return nil
	}
	var reqs []reconcile.Request
	for i := range cbms.Items {
		cbm := &cbms.Items[i]
		if !pvcSpecMatches(&cbm.Spec, pvc.Name, pvc.Namespace, true) {
			continue
		}
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: cbm.Name},
		})
	}
	return reqs
}

// pvcSpecMatches reports whether spec references the PVC identified
// by (pvcName, pvcNamespace). pvc:// URI format is pvc://namespace/name. Namespaced
// callers scope their List by namespace; cluster-scoped callers don't.
func pvcSpecMatches(spec *v1beta1.BaseModelSpec, pvcName, pvcNamespace string, isClusterScoped bool) bool {
	if !IsPVCStorage(spec) {
		return false
	}
	components, err := storage.ParsePVCStorageURI(*spec.Storage.StorageUri)
	if err != nil {
		return false
	}
	if components.PVCName != pvcName {
		return false
	}
	if isClusterScoped {
		return components.Namespace == pvcNamespace
	}
	return components.Namespace == ""
}
