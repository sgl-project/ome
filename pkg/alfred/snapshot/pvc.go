package snapshot

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/utils/storage"
)

// resolvePVCAvailability fills a BackendPVC ModelAvailability from the claim
// behind a pvc:// storage URI:
//
//   - Access modes decide migratability: RWX (and ROX — model weights are
//     read-only at inference time) volumes mount on many nodes, so any
//     topology-compatible node is a target and no model pull is ever needed.
//     RWO/RWOP volumes attach to one node at a time; a surge replacement
//     cannot attach while the source pod holds the volume, so the workload is
//     VolumePinned (advisory-only).
//   - The bound PV's node affinity yields the CSI-reachable node set; an
//     unbound claim or a PV without affinity means unconstrained (nil).
//
// Resolution failures land in ResolveError rather than failing the snapshot:
// one broken model must not blind Alfred to the rest of the cluster.
func resolvePVCAvailability(ctx context.Context, r client.Reader, avail *ModelAvailability, defaultNamespace, storageURI string, nodes map[string]*Node) {
	components, err := storage.ParsePVCStorageURI(storageURI)
	if err != nil {
		avail.ResolveError = fmt.Sprintf("parse pvc storage uri: %v", err)
		return
	}
	namespace := components.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}
	if namespace == "" {
		avail.ResolveError = "pvc namespace unresolved (cluster-scoped model with no namespace in uri)"
		return
	}

	var pvc corev1.PersistentVolumeClaim
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: components.PVCName}, &pvc); err != nil {
		avail.ResolveError = fmt.Sprintf("get pvc %s/%s: %v", namespace, components.PVCName, err)
		return
	}

	modes := pvc.Spec.AccessModes

	var pv *corev1.PersistentVolume
	if pvc.Spec.VolumeName != "" {
		var fetched corev1.PersistentVolume
		if err := r.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, &fetched); err != nil {
			avail.ResolveError = fmt.Sprintf("get pv %s: %v", pvc.Spec.VolumeName, err)
			return
		}
		pv = &fetched
		if len(modes) == 0 {
			modes = pv.Spec.AccessModes
		}
	}

	for _, m := range modes {
		avail.PVCAccessModes = append(avail.PVCAccessModes, string(m))
	}
	avail.VolumePinned = volumePinned(modes)

	if pv != nil && pv.Spec.NodeAffinity != nil && pv.Spec.NodeAffinity.Required != nil {
		var reachable []string
		for name, node := range nodes {
			if matchNodeSelector(node.Labels, name, pv.Spec.NodeAffinity.Required) {
				reachable = append(reachable, name)
			}
		}
		sort.Strings(reachable)
		// Non-nil even when empty: an affinity that matches no node is a
		// real "no feasible target", distinct from unconstrained (nil).
		if reachable == nil {
			reachable = []string{}
		}
		avail.PVCTopologyNodes = reachable
	}
}

// volumePinned reports whether the access modes pin the volume to a single
// node at a time. A volume offering RWX or ROX is multi-node mountable and
// therefore migratable regardless of what else it offers.
func volumePinned(modes []corev1.PersistentVolumeAccessMode) bool {
	for _, m := range modes {
		if m == corev1.ReadWriteMany || m == corev1.ReadOnlyMany {
			return false
		}
	}
	return true
}

// matchNodeSelector evaluates a corev1.NodeSelector (terms OR'd, expressions
// within a term AND'd) against a node's labels and name. Implemented locally
// because k8s.io/component-helpers is not a module dependency.
func matchNodeSelector(labels map[string]string, nodeName string, selector *corev1.NodeSelector) bool {
	for i := range selector.NodeSelectorTerms {
		if matchNodeSelectorTerm(labels, nodeName, &selector.NodeSelectorTerms[i]) {
			return true
		}
	}
	return false
}

func matchNodeSelectorTerm(labels map[string]string, nodeName string, term *corev1.NodeSelectorTerm) bool {
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return false
	}
	for i := range term.MatchExpressions {
		expr := &term.MatchExpressions[i]
		value, exists := labels[expr.Key]
		if !matchRequirement(value, exists, expr) {
			return false
		}
	}
	for i := range term.MatchFields {
		expr := &term.MatchFields[i]
		// metadata.name is the only supported node field selector.
		if expr.Key != "metadata.name" {
			return false
		}
		if !matchRequirement(nodeName, true, expr) {
			return false
		}
	}
	return true
}

func matchRequirement(value string, exists bool, expr *corev1.NodeSelectorRequirement) bool {
	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		if !exists {
			return false
		}
		for _, v := range expr.Values {
			if v == value {
				return true
			}
		}
		return false
	case corev1.NodeSelectorOpNotIn:
		if !exists {
			return true
		}
		for _, v := range expr.Values {
			if v == value {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpExists:
		return exists
	case corev1.NodeSelectorOpDoesNotExist:
		return !exists
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		if !exists || len(expr.Values) != 1 {
			return false
		}
		lhs, err1 := strconv.ParseInt(value, 10, 64)
		rhs, err2 := strconv.ParseInt(expr.Values[0], 10, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		if expr.Operator == corev1.NodeSelectorOpGt {
			return lhs > rhs
		}
		return lhs < rhs
	}
	return false
}
