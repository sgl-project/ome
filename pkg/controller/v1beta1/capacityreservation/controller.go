package capacityreservation

import (
	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	clusterQueueReconciler "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/capacityreservation/reconcilers/kueueclusterqueue"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/capacityreservation/utils"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
	"time"
)

// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations/finalizers,verbs=update
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/finalizers,verbs=update
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=resourceflavors,verbs=get;list;watch;update;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=resourceflavors/finalizers,verbs=update

// CapacityReservationReconciler reconciles a CapacityReservation object
type CapacityReservationReconciler struct {
	client.Client
	CapacityReservationReconcilePolicy *omev1beta1.CapacityReservationReconcilePolicyConfig
	ClientConfig                       *rest.Config
	Clientset                          kubernetes.Interface
	Log                                logr.Logger
	Scheme                             *runtime.Scheme
	Recorder                           record.EventRecorder
}

func (r *CapacityReservationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// check if capacityReservation is ready, if not, create a clusterQueue and mark as ready
	clusterCapacityReservation := &omev1beta1.ClusterCapacityReservation{}
	r.Log.Info("Reconcile ClusterCapacityReservation", "name", req.NamespacedName.Name)
	if err := r.Get(ctx, req.NamespacedName, clusterCapacityReservation); err != nil {
		if apierr.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "unable to fetch ClusterCapacityReservation", "name", req.NamespacedName.Name)
		return ctrl.Result{}, err
	}

	if !clusterCapacityReservation.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, clusterCapacityReservation)
	}

	spec := clusterCapacityReservation.Spec.DeepCopy()

	// resource is sufficient to process the reconcile request
	clusterQueueReconcile := clusterQueueReconciler.NewClusterQueueReconciler(
		r.Client,
		r.Scheme,
		req.NamespacedName.Name,
		spec.ResourceGroups,
		spec.Cohort,
		spec.PreemptionRule,
	)
	if clusterQueueReconcile.ClusterQueue != nil && !metav1.IsControlledBy(clusterQueueReconcile.ClusterQueue, clusterCapacityReservation) {
		r.Log.Info("Add clusterQueue owner reference", "name", clusterCapacityReservation.Name)
		if err := controllerutil.SetControllerReference(clusterCapacityReservation, clusterQueueReconcile.ClusterQueue, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set clusterqueue owner reference")
		}
	}

	clusterQueue, err := clusterQueueReconcile.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile clusterqueue")
	}
	requeue, err := r.updateClusterCapacityReservationStatus(clusterCapacityReservation, clusterQueue)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrapf(err, "failed to update the status of capacityreservation %s", clusterCapacityReservation.Name)
	}
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}
	if !clusterCapacityReservation.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, clusterCapacityReservation)
	} else {
		if err = r.ensureFinalizer(clusterCapacityReservation); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *CapacityReservationReconciler) updateClusterCapacityReservationStatus(
	clusterCapacityReservation *omev1beta1.ClusterCapacityReservation,
	clusterQueue *kueuev1beta1.ClusterQueue) (bool, error) {

	r.Log.Info("Update ClusterCapacityReservation Status", "name", clusterCapacityReservation.Name)
	if !r.CapacityReservationReconcilePolicy.ReconcileFailedLifecycleState {
		if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationFailed {
			r.Log.Info("ClusterCapacityReservation reconcile failed", "name", clusterCapacityReservation.Name)
			return false, nil
		}
	}

	checkStatus := func() (bool, error) {
		if utils.CheckClusterQueueActive(clusterQueue) {
			if clusterCapacityReservation.Status.CapacityReservationLifecycleState != omev1beta1.CapacityReservationActive {
				r.Log.Info("ClusterCapacityReservation is active", "name", clusterCapacityReservation.Name)
				clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationActive
				clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.CapacityReservationActive)

				capacity := utils.ConvertResourceGroupsToFlavorUsage(clusterCapacityReservation.Spec.ResourceGroups)
				clusterCapacityReservation.Status.Capacity = capacity
				clusterCapacityReservation.Status.Allocatable = utils.DeepCopyFlavorsUsage(capacity)

				ready := omev1beta1.CapacityReservationCondition{
					Type:               omev1beta1.CapacityReservationReady,
					Status:             v1.ConditionStatus(metav1.ConditionTrue),
					Reason:             "Initialized",
					Message:            "CapacityReservation initialized",
					LastTransitionTime: metav1.NewTime(time.Now()),
				}
				setCondition(&clusterCapacityReservation.Status.Conditions, ready)
			}
		} else {
			if clusterQueue.CreationTimestamp.IsZero() {
				r.Log.Info("ClusterCapacityReservation is creating", "name", clusterCapacityReservation.Name)
				clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationCreating
				clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.CREATING)
			} else {
				r.Log.Info("ClusterCapacityReservation is updating", "name", clusterCapacityReservation.Name)
				clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationUpdating
				clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.UPDATING)
			}
		}

		if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationFailed || clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationActive {
			return false, nil
		}
		return true, nil
	}

	requeue, err := checkStatus()
	if err != nil {
		r.Log.Error(err, "Failed to check CapacityReservation Status", "CapacityReservation", clusterCapacityReservation.Name)
		clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationFailed
		clusterCapacityReservation.Status.LifecycleDetail = err.Error()

		// Set the failure condition
		ready := omev1beta1.CapacityReservationCondition{
			Type:               omev1beta1.CapacityReservationReady,
			Status:             v1.ConditionStatus(metav1.ConditionFalse),
			Reason:             "Failed",
			Message:            fmt.Sprintf("CapacityReservation failed: %v", err),
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
		setCondition(&clusterCapacityReservation.Status.Conditions, ready)
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Client.Status().Update(context.TODO(), clusterCapacityReservation)
	})
	if err != nil {
		r.Log.Error(err, "Failed to update CapacityReservation Status", "CapacityReservation", clusterCapacityReservation.Name)
		return true, err
	}
	return requeue, nil
}

func (r *CapacityReservationReconciler) updateStatus(ctx context.Context, reservation *omev1beta1.ClusterCapacityReservation) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Status().Update(ctx, reservation)
	})
}

func (r *CapacityReservationReconciler) handleDeletion(ctx context.Context, clusterCapacityReservation *omev1beta1.ClusterCapacityReservation) error {
	r.Log.Info("Deleting CapacityReservation", "name", clusterCapacityReservation.Name)
	clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationDeleting
	clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.DELETING)

	if controllerutil.ContainsFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer) {
		r.Log.Info("remove clusterCapacityReservation finalizer")
		controllerutil.RemoveFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer)
		if err := r.Update(ctx, clusterCapacityReservation); err != nil {
			r.Log.Error(err, "failed to remove clusterCapacityReservation finalizer")
			return err
		}
	}
	// Children components do not have parent Finalizer, no need to remove
	return nil
}

func (r *CapacityReservationReconciler) ensureFinalizer(clusterCapacityReservation *omev1beta1.ClusterCapacityReservation) error {
	if !controllerutil.ContainsFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer) {
		r.Log.Info("add clusterCapacityReservation finalizer", "name", clusterCapacityReservation.Name)
		controllerutil.AddFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer)
		if err := r.Update(context.Background(), clusterCapacityReservation); err != nil {
			return err
		}
	}
	return nil
}

func setCondition(conditions *[]omev1beta1.CapacityReservationCondition, condition omev1beta1.CapacityReservationCondition) {
	existing := findCondition(*conditions, condition.Type)
	if existing == nil {
		*conditions = append(*conditions, condition)
		return
	}
	// Update existing condition if changed.
	if existing.Status != condition.Status || existing.Reason != condition.Reason || existing.Message != condition.Message {
		*existing = condition
	}
}

func findCondition(conditions []omev1beta1.CapacityReservationCondition, t omev1beta1.CapacityReservationConditionType) *omev1beta1.CapacityReservationCondition {
	for i := range conditions {
		if conditions[i].Type == t {
			return &conditions[i]
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CapacityReservationReconciler) SetupWithManager(mgr ctrl.Manager, capacityReservationReconcilePolicyConfig *omev1beta1.CapacityReservationReconcilePolicyConfig) error {
	r.ClientConfig = mgr.GetConfig()
	r.CapacityReservationReconcilePolicy = capacityReservationReconcilePolicyConfig

	predicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
	eventHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{Name: obj.GetNamespace()},
			},
		}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&omev1beta1.ClusterCapacityReservation{}).
		Owns(&kueuev1beta1.ClusterQueue{}).
		Watches(
			&omev1beta1.InferenceService{},
			eventHandler,
			builder.WithPredicates(predicates)).
		Complete(r)
}
