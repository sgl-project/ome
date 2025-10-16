package acceleratorclass

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/fields"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// +kubebuilder:rbac:groups=ome.io,resources=acceleratorclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=acceleratorclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=acceleratorclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

type AcceleratorClassReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *AcceleratorClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("acceleratorclass", req.NamespacedName)

	ac := &v1beta1.AcceleratorClass{}
	if err := r.Get(ctx, req.NamespacedName, ac); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get AcceleratorClass")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !ac.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(ac, constants.AcceleratorClassFinalizer) {
			controllerutil.RemoveFinalizer(ac, constants.AcceleratorClassFinalizer)
			if err := r.Update(ctx, ac); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer present
	if !controllerutil.ContainsFinalizer(ac, constants.AcceleratorClassFinalizer) {
		controllerutil.AddFinalizer(ac, constants.AcceleratorClassFinalizer)
		if err := r.Update(ctx, ac); err != nil {
			return ctrl.Result{}, err
		}
	}

	// List nodes and apply filters
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		log.Error(err, "failed to list nodes")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	matchedNodes := make([]string, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if !nodePassesDiscovery(ac, &node) {
			continue
		}
		if !nodeMatchCapabilities(ac, &node) {
			continue
		}
		matchedNodes = append(matchedNodes, node.Name)
	}
	sort.Strings(matchedNodes)

	// In Reconcile, after computing desired fields (without setting LastUpdated yet):
	latest := &v1beta1.AcceleratorClass{}
	if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	desired := latest.DeepCopy()
	desired.Status.Nodes = matchedNodes
	desired.Status.AvailableNodes = int32(len(matchedNodes))

	// Only update status if something changed (except LastUpdated):
	if !acceleratorClassStatusEqualIgnoreTime(latest.Status, desired.Status) {
		desired.Status.LastUpdated = metav1.Now()
		if err := r.Status().Patch(ctx, desired, client.MergeFrom(latest)); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager wires the controller and watches nodes to trigger reconciles
func (r *AcceleratorClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.AcceleratorClass{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				// Any node change could affect any AcceleratorClass; requeue all
				acList := &v1beta1.AcceleratorClassList{}
				if err := r.List(ctx, acList); err != nil {
					return nil
				}
				requests := make([]reconcile.Request, 0, len(acList.Items))
				for i := range acList.Items {
					requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&acList.Items[i])})
				}
				return requests
			}),
			builder.WithPredicates(),
		).
		Complete(r)
}

func nodePassesDiscovery(ac *v1beta1.AcceleratorClass, node *corev1.Node) bool {
	// NodeSelector map: all key=value must match
	if len(ac.Spec.Discovery.NodeSelector) > 0 {
		for k, v := range ac.Spec.Discovery.NodeSelector {
			if node.Labels[k] != v {
				return false
			}
		}
	}

	return true
}

func matchNodeSelectorTerm(node *corev1.Node, term corev1.NodeSelectorTerm) bool {
	if len(term.MatchExpressions) > 0 {
		for _, req := range term.MatchExpressions {
			if !matchNodeSelectorExpressions(node, req) {
				return false
			}
		}
	}
	if len(term.MatchFields) > 0 {
		for _, req := range term.MatchFields {
			if !matchNodeSelectorFields(node, req) {
				return false
			}
		}
	}
	return true
}

func matchNodeSelectorExpressions(node *corev1.Node, req corev1.NodeSelectorRequirement) bool {
	val, has := node.Labels[req.Key]
	switch req.Operator {
	case corev1.NodeSelectorOpIn:
		if !has {
			return false
		}
		for _, v := range req.Values {
			if v == val {
				return true
			}
		}
		return false
	case corev1.NodeSelectorOpNotIn:
		if !has {
			return true
		}
		for _, v := range req.Values {
			if v == val {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpExists:
		return has
	case corev1.NodeSelectorOpDoesNotExist:
		return !has
	case corev1.NodeSelectorOpGt:
		if !has || len(req.Values) == 0 {
			return false
		}
		return strings.Compare(val, req.Values[0]) > 0
	case corev1.NodeSelectorOpLt:
		if !has || len(req.Values) == 0 {
			return false
		}
		return strings.Compare(val, req.Values[0]) < 0
	default:
		return true
	}
}

func matchNodeSelectorFields(node *corev1.Node, req corev1.NodeSelectorRequirement) bool {
	val, has := extractNodeFields(node)[req.Key]
	if !has {
		return false
	}
	for _, v := range req.Values {
		if v == val {
			return true
		}
	}
	return false
}

func extractNodeFields(n *corev1.Node) fields.Set {
	f := make(fields.Set)
	if len(n.Name) > 0 {
		f["metadata.name"] = n.Name
	}
	return f
}

func nodeMatchCapabilities(ac *v1beta1.AcceleratorClass, node *corev1.Node) bool {
	// memoryGB: compare with node memory capacity
	if ac.Spec.Capabilities.MemoryGB != nil {
		memQty := node.Status.Capacity[corev1.ResourceMemory]
		if memQty.Cmp(*ac.Spec.Capabilities.MemoryGB) < 0 {
			return false
		}
	}

	// computeCapability: require at least 1 GPU present
	if ac.Spec.Capabilities.ComputeCapability != "" {
		acCompute, err := strconv.ParseFloat(ac.Spec.Capabilities.ComputeCapability, 64)
		if err != nil {
			return false
		}
		// Get GPU count from node resources
		total, _ := getGPUCapacity(node)
		if total < int64(acCompute) {
			return false
		}
	}
	return true
}

func getGPUCapacity(node *corev1.Node) (total int64, byResource map[string]int64) {
	byResource = make(map[string]int64)

	// Prefer Capacity if you want physical capacity; use Allocatable if you care about schedulable.
	res := node.Status.Capacity
	if len(res) == 0 {
		res = node.Status.Allocatable
	}

	for name, q := range res {
		n := string(name)

		// NVIDIA classic
		if n == "nvidia.com/gpu" {
			v := q.Value()
			byResource[n] = v
			total += v
			continue
		}

		// NVIDIA MIG profiles (treat as accelerators; not equivalent to card count)
		if strings.HasPrefix(n, "nvidia.com/mig-") {
			v := q.Value()
			byResource[n] += v
			total += v
			continue
		}

		// AMD (common)
		if n == "amd.com/gpu" {
			v := q.Value()
			byResource[n] = v
			total += v
			continue
		}

		// Intel (common plugin exposes under gpu.intel.com/*; skip memory-only resources)
		if strings.HasPrefix(n, "gpu.intel.com/") && !strings.Contains(n, "memory") {
			v := q.Value()
			byResource[n] += v
			total += v
			continue
		}
	}
	return total, byResource
}

// returns true if equal when ignoring LastUpdated
func acceleratorClassStatusEqualIgnoreTime(a, b v1beta1.AcceleratorClassStatus) bool {
	aCopy := a
	bCopy := b
	aCopy.LastUpdated = metav1.Time{}
	bCopy.LastUpdated = metav1.Time{}
	return equality.Semantic.DeepEqual(aCopy, bCopy)
}
