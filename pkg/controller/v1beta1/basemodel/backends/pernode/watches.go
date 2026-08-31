package pernode

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/shared"
)

func CreateNodeDeletionPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool { return false },
		DeleteFunc: func(e event.DeleteEvent) bool { return true },
	}
}

// HandleNodeDeletion deletes the per-node model-status ConfigMap when
// the Node it was tracking goes away. No-op if no ConfigMap exists
// (nodes that never ran model-agent).
func HandleNodeDeletion(ctx context.Context, kubeClient client.Client, log logr.Logger, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}

	nodeName := node.GetName()
	log = log.WithValues("node", nodeName)

	configMap := &corev1.ConfigMap{}
	configMapKey := types.NamespacedName{
		Namespace: constants.OMENamespace,
		Name:      nodeName,
	}

	if err := kubeClient.Get(ctx, configMapKey, configMap); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "Failed to check ConfigMap for deleted node")
		return nil
	}

	if !IsModelStatusConfigMap(configMap) {
		return nil
	}

	log.Info("Node deleted, cleaning up associated model status ConfigMap")
	if err := kubeClient.Delete(ctx, configMap); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete ConfigMap for deleted node")
		}
		return nil
	}

	log.Info("Successfully deleted ConfigMap for deleted node")
	return nil
}

func CreateModelStatusConfigMapPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return IsModelStatusConfigMap(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return IsModelStatusConfigMap(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return IsModelStatusConfigMap(e.Object)
		},
	}
}

func IsModelStatusConfigMap(obj client.Object) bool {
	if obj.GetNamespace() != constants.OMENamespace {
		return false
	}
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[constants.ModelStatusConfigMapLabel] == "true"
}

// MapConfigMapToModelRequests fans a per-node ConfigMap event out to
// reconcile requests for the models it tracks. isNamespaced selects
// BaseModel (namespaced) vs ClusterBaseModel (cluster-scoped).
func MapConfigMapToModelRequests(obj client.Object, log logr.Logger, isNamespaced bool) []reconcile.Request {
	var requests []reconcile.Request

	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return requests
	}

	for key, data := range configMap.Data {
		namespace, modelName, isClusterBaseModel, success := constants.ParseModelInfoFromConfigMapKey(key)
		if !success {
			continue
		}
		if (isNamespaced && isClusterBaseModel) || (!isNamespaced && !isClusterBaseModel) {
			continue
		}

		var modelEntry shared.ModelEntry
		if err := json.Unmarshal([]byte(data), &modelEntry); err != nil {
			log.V(1).Info("Failed to parse model entry in ConfigMap", "configMap", configMap.Name, "key", key, "error", err)
			continue
		}

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: modelName}}
		if isNamespaced {
			req.Namespace = namespace
		}
		requests = append(requests, req)
	}

	return requests
}
