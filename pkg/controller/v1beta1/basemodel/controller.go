package basemodel

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/backends/pernode"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/backends/pvc"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/basemodel/shared"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// +kubebuilder:rbac:groups=ome.io,resources=basemodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=basemodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=basemodels/finalizers,verbs=update
// +kubebuilder:rbac:groups=ome.io,resources=clusterbasemodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=clusterbasemodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=clusterbasemodels/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// modelKind is what differs between the two model kinds the reconciler
// serves. The pass, the backends, the finalizer handling and the watches
// are the same for both.
type modelKind struct {
	// name is the kind as it reads in log lines.
	name string
	// finalizer is the finalizer the reconciler owns on objects of the kind.
	finalizer string
	// clusterScoped selects the cluster-wide ConfigMap keys, Job labels and
	// PVC URI form (pvc://namespace:name/path) over the namespaced ones.
	clusterScoped bool
	// newObject returns an empty object of the kind for the client to fill.
	newObject func() client.Object
	// modelsUsingPVC lists the models of the kind whose storage URI names the PVC.
	modelsUsingPVC func(ctx context.Context, c client.Client, log logr.Logger, pvc client.Object) []reconcile.Request
}

var (
	baseModelKind = modelKind{
		name:           "BaseModel",
		finalizer:      constants.BaseModelFinalizer,
		newObject:      func() client.Object { return &v1beta1.BaseModel{} },
		modelsUsingPVC: pvc.MapToBaseModels,
	}
	clusterBaseModelKind = modelKind{
		name:           "ClusterBaseModel",
		finalizer:      constants.ClusterBaseModelFinalizer,
		clusterScoped:  true,
		newObject:      func() client.Object { return &v1beta1.ClusterBaseModel{} },
		modelsUsingPVC: pvc.MapToClusterBaseModels,
	}
)

// Reconciler is the one reconciler for both model kinds. It holds what a
// pass needs for either kind; BaseModelReconciler and
// ClusterBaseModelReconciler bind it to one kind each, so the manager
// runs one controller per kind over the same pass.
type Reconciler struct {
	client.Client
	// APIReader confirms Node absence without the informer cache before cleanup.
	APIReader      client.Reader
	Log            logr.Logger
	Scheme         *runtime.Scheme
	OmeAgentConfig *controllerconfig.OmeAgentConfig
}

// BaseModelReconciler is the Reconciler bound to the namespaced kind.
type BaseModelReconciler Reconciler

func (r *BaseModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return (*Reconciler)(r).reconcile(ctx, baseModelKind, req)
}

func (r *BaseModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return (*Reconciler)(r).setupWithManager(mgr, baseModelKind, r)
}

// ClusterBaseModelReconciler is the Reconciler bound to the cluster-scoped kind.
type ClusterBaseModelReconciler Reconciler

func (r *ClusterBaseModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return (*Reconciler)(r).reconcile(ctx, clusterBaseModelKind, req)
}

func (r *ClusterBaseModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return (*Reconciler)(r).setupWithManager(mgr, clusterBaseModelKind, r)
}

// backends is the dispatch list in match order: pvc, then the per-node
// backend, which matches every model and so goes last.
func (r *Reconciler) backends() []shared.Backend {
	return []shared.Backend{
		pvc.New(r.OmeAgentConfig),
		perNodeBackend{nodeReader: r.APIReader},
	}
}

// reconcile reads the model named by the request and runs the pass on
// it. A model that is already gone needs nothing.
func (r *Reconciler) reconcile(ctx context.Context, kind modelKind, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(strings.ToLower(kind.name), req.NamespacedName)

	model := kind.newObject()
	if err := r.Get(ctx, req.NamespacedName, model); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get "+kind.name)
		return ctrl.Result{}, err
	}
	return reconcileModel(ctx, r.Client, r.Scheme, log, r.backends(), model, kind)
}

// reconcileModel is the pass shared by both kinds: pick the backend, hand
// a deleting model to it, otherwise put the finalizer on the object
// before the backend writes anything, then let the backend reconcile.
func reconcileModel(ctx context.Context, c client.Client, scheme *runtime.Scheme, log logr.Logger, backends []shared.Backend, obj client.Object, kind modelKind) (ctrl.Result, error) {
	spec, status, err := shared.ModelSpecAndStatus(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Reconciling " + kind.name)

	backend := pickBackend(backends, spec)
	args := shared.BackendArgs{
		Client:          c,
		Scheme:          scheme,
		Log:             log,
		Obj:             obj,
		Spec:            spec,
		Status:          status,
		Finalizer:       kind.finalizer,
		IsClusterScoped: kind.clusterScoped,
		Kind:            kind.name,
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		log.Info("Handling " + kind.name + " deletion via " + backend.Name() + " backend")
		return backend.HandleDeletion(ctx, args)
	}

	if !controllerutil.ContainsFinalizer(obj, kind.finalizer) {
		log.Info("Adding finalizer to " + kind.name)
		controllerutil.AddFinalizer(obj, kind.finalizer)
		if err := c.Update(ctx, obj); err != nil {
			// A requeue can outrun the informer watch and read a stale
			// resourceVersion. Let the next pass retry from fresh state instead
			// of reporting the expected optimistic-lock rejection as a failure.
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	log.V(1).Info("Dispatching "+kind.name+" reconcile to backend", "backend", backend.Name())
	return backend.Reconcile(ctx, args)
}

// setupWithManager registers reconciler as the controller for the kind.
// It watches the kind's objects, the metadata Jobs they own, the status
// ConfigMaps the agents and the metadata Jobs write, Node deletions and
// the PVCs the models name.
func (r *Reconciler) setupWithManager(mgr ctrl.Manager, kind modelKind, reconciler reconcile.Reconciler) error {
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(kind.newObject()).
		Owns(&batchv1.Job{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return pernode.MapConfigMapToModelRequests(obj, r.Log, !kind.clusterScoped)
			}),
			// One watch serves the per-node model-status ConfigMaps and the
			// per-PVC status ConfigMaps: both key their entries by
			// GetModelConfigMapKey, so one mapper reads both.
			builder.WithPredicates(predicate.Or(pernode.CreateModelStatusConfigMapPredicate(), createPVCStatusConfigMapPredicate())),
		).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return pernode.HandleNodeDeletion(ctx, r.Client, r.Log, obj)
			}),
			builder.WithPredicates(pernode.CreateNodeDeletionPredicate()),
		).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return kind.modelsUsingPVC(ctx, r.Client, r.Log, obj)
			}),
			builder.WithPredicates(pvc.CreatePhasePredicate()),
		).
		Complete(reconciler)
}

// createPVCStatusConfigMapPredicate fires on ConfigMaps in the OME
// namespace carrying the PVC-status label. Lives here (not in pvc/)
// because it's OR'd with pernode's predicate in one watch.
func createPVCStatusConfigMapPredicate() predicate.Predicate {
	isPVC := func(obj client.Object) bool {
		if obj.GetNamespace() != constants.OMENamespace {
			return false
		}
		return obj.GetLabels()[constants.PVCStorageConfigMapLabel] == "true"
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return isPVC(e.Object) },
		UpdateFunc: func(e event.UpdateEvent) bool { return isPVC(e.ObjectNew) },
		DeleteFunc: func(e event.DeleteEvent) bool { return isPVC(e.Object) },
	}
}
