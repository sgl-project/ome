package capacityreservation

import (
	"context"
	"fmt"
	"time"

	volbatchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
	vbatchv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	clusterQueueReconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/capacityreservation/reconcilers/kueueclusterqueue"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/capacityreservation/utils"
	generalutils "github.com/sgl-project/sgl-ome/pkg/utils"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
)

// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=capacityreservations/finalizers,verbs=update
// +kubebuilder:rbac:groups=ome.io,resources=clustercapacityreservations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=clustercapacityreservations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=clustercapacityreservations/finalizers,verbs=update
// +kubebuilder:rbac:groups=node.k8s.io,resources=runtimeclasses,verbs=list;watch
// +kubebuilder:rbac:groups=core,resources=limitranges,verbs=list;watch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/finalizers,verbs=update
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=resourceflavors,verbs=get;list;watch;update;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=resourceflavors/finalizers,verbs=update

// CapacityReservationReconciler reconciles a CapacityReservation object
type CapacityReservationReconciler struct {
	client.Client
	CapacityReservationReconcilePolicy *controllerconfig.CapacityReservationReconcilePolicyConfig
	ClientConfig                       *rest.Config
	Clientset                          kubernetes.Interface
	Log                                logr.Logger
	Scheme                             *runtime.Scheme
	Recorder                           record.EventRecorder
}

func (r *CapacityReservationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		err := r.handleDeletion(ctx, clusterCapacityReservation)
		return ctrl.Result{}, err
	}

	spec := clusterCapacityReservation.Spec.DeepCopy()

	isSufficient, err := r.isResourceSufficient(ctx, clusterCapacityReservation, spec.ResourceGroups)
	if err != nil {
		r.Log.Error(err, "Error checking resource sufficiency for capacity reservation", "name", clusterCapacityReservation.Name)
		return ctrl.Result{}, err
	}
	if !isSufficient {
		// When resources are insufficient, the controller logs an error and updates the status to "Failed".
		// The controller does not fail outright but continues to reconcile with exponential back-off,
		// allowing for potential cluster auto-scaling or resource changes in the future.
		// However, in this specific case, we prioritize fast failure due to GPU resources limited auto-scaling capabilities.
		// If updating the status to "Failed" succeeds, the controller returns without error; otherwise, it returns an error.
		err = fmt.Errorf("insufficient resources for capacity reservation %s", clusterCapacityReservation.Name)
		r.Log.Error(err, "Insufficient resources for capacity reservation", "name", clusterCapacityReservation.Name)
		err = r.updateStatusToFailedWhenResourcesInsufficient(clusterCapacityReservation, err)
		return ctrl.Result{Requeue: false, RequeueAfter: 0}, err
	}
	r.Log.Info("sufficient resources to admit clusterCapacityReservation", "name", clusterCapacityReservation.Name)
	// if capacity reservation failed previously due to insufficient resources, remove failed state and message to revert the change
	if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationFailed {
		resourcesSufficientCondition := findCondition(clusterCapacityReservation.Status.Conditions, omev1beta1.ResourcesSufficient)
		if resourcesSufficientCondition != nil && resourcesSufficientCondition.Status != v1.ConditionTrue {
			clusterCapacityReservation.Status.CapacityReservationLifecycleState = ""
			clusterCapacityReservation.Status.LifecycleDetail = ""
		}
	}

	// resource is sufficient to process the reconcile request
	// reconcile children components
	// decoupling: create a new reconciler for child cr instead of reusing
	clusterQueueReconcile, err := clusterQueueReconciler.NewClusterQueueReconciler(
		r.Client,
		r.Scheme,
		req.NamespacedName.Name,
		spec.ResourceGroups,
		spec.Cohort,
		spec.PreemptionRule,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if clusterQueueReconcile.ClusterQueue != nil && !metav1.IsControlledBy(clusterQueueReconcile.ClusterQueue, clusterCapacityReservation) {
		r.Log.Info("Add clusterQueue owner reference", "name", clusterCapacityReservation.Name)
		if err = controllerutil.SetControllerReference(clusterCapacityReservation, clusterQueueReconcile.ClusterQueue, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set clusterqueue owner reference")
		}
	}

	clusterQueue, err := clusterQueueReconcile.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile clusterqueue")
	}

	// update status
	requeue, err := r.updateClusterCapacityReservationStatus(clusterCapacityReservation, clusterQueue, clusterQueueReconcile.CreationFailedTimeThreshold)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrapf(err, "failed to update the status of capacityreservation %s", clusterCapacityReservation.Name)
	}
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}

	if !clusterCapacityReservation.ObjectMeta.DeletionTimestamp.IsZero() {
		err = r.handleDeletion(ctx, clusterCapacityReservation)
		return ctrl.Result{}, err
	}

	// ensure finalizer
	if !controllerutil.ContainsFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer) {
		r.Log.Info("add clusterCapacityReservation finalizer", "name", clusterCapacityReservation.Name)
		controllerutil.AddFinalizer(clusterCapacityReservation, constants.ClusterCapacityReservationFinalizer)
	}

	// process update
	if err = r.Update(ctx, clusterCapacityReservation); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CapacityReservationReconciler) updateClusterCapacityReservationStatus(
	clusterCapacityReservation *omev1beta1.ClusterCapacityReservation,
	clusterQueue *kueuev1beta1.ClusterQueue,
	creationFailedTimeThreshold time.Duration) (bool, error) {

	r.Log.Info("Update ClusterCapacityReservation Status", "name", clusterCapacityReservation.Name)
	if !r.CapacityReservationReconcilePolicy.ReconcileFailedLifecycleState {
		if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationFailed {
			r.Log.Info("ClusterCapacityReservation reconcile failed", "name", clusterCapacityReservation.Name)
			return false, nil
		}
	}

	checkStatus := func() (bool, error) {
		if utils.CheckClusterQueueActive(clusterQueue) {
			// cluster queue creation/update succeeds, mark ClusterCapacityReservation ready
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
			resourcesSufficient := omev1beta1.CapacityReservationCondition{
				Type:               omev1beta1.ResourcesSufficient,
				Status:             v1.ConditionStatus(metav1.ConditionTrue),
				Reason:             "Resource Sufficient",
				Message:            "Resource Sufficient for CapacityReservation",
				LastTransitionTime: metav1.NewTime(time.Now()),
			}
			setCondition(&clusterCapacityReservation.Status.Conditions, ready)
			setCondition(&clusterCapacityReservation.Status.Conditions, resourcesSufficient)
		} else if utils.CheckClusterQueueInactive(clusterQueue) {
			// cluster queue creation/update fails, let CapacityReservationReconciler fail fast
			r.Log.Info("ClusterCapacityReservation reconciliation failed because associated clusterQueue failed", "name", clusterCapacityReservation.Name)
			clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationFailed
			clusterCapacityReservation.Status.LifecycleDetail = "Associated clusterQueue failed"
			// log detailed reason
			if clusterQueue.Status.Conditions != nil {
				conditions := clusterQueue.Status.Conditions
				for i := range conditions {
					if conditions[i].Type == kueuev1beta1.ClusterQueueActive {
						r.Log.Info("clusterQueue failed", "reason", conditions[i].Reason, "message", conditions[i].Message)
						clusterCapacityReservation.Status.LifecycleDetail = strings.Join([]string{clusterCapacityReservation.Status.LifecycleDetail, conditions[i].Message}, ". ")
					}
				}
			}
		} else {
			// if no capacity is set, still creating
			if clusterCapacityReservation.Status.Capacity == nil {
				// exceeds threshold on creation
				if clusterQueue.CreationTimestamp.Add(creationFailedTimeThreshold).Before(time.Now()) {
					r.Log.Info("ClusterCapacityReservation failed on creation because associated clusterQueue remains inactive beyond creationFailedTimeThreshold", "name", clusterCapacityReservation.Name, "creationFailedTimeThreshold", creationFailedTimeThreshold)
					if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationCreating {
						clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationFailed
						clusterCapacityReservation.Status.LifecycleDetail = fmt.Sprintf("Associated clusterQueue remains inactive beyond creationFailedTimeThreshold %v", creationFailedTimeThreshold)
					} else {
						// need further investigate if it is under other lifecycle states
						r.Log.Info("clusterCapacityReservation status", "lifecycleState", clusterCapacityReservation.Status.CapacityReservationLifecycleState, "lifecycleDetail", clusterCapacityReservation.Status.LifecycleDetail)
						return false, fmt.Errorf("please investigate on clusterQueue %s condition", clusterCapacityReservation.Name)
					}
				} else {
					r.Log.Info("ClusterCapacityReservation is creating", "name", clusterCapacityReservation.Name)
					clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationCreating
					clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.CapacityReservationCreating)
				}
			} else {
				// TODO: prevent controller stuck on updating. ClusterQueue does not log lastTransitionTime.
				r.Log.Info("ClusterCapacityReservation is updating", "name", clusterCapacityReservation.Name)
				clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationUpdating
				clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.CapacityReservationUpdating)
			}
		}

		if clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationFailed || clusterCapacityReservation.Status.CapacityReservationLifecycleState == omev1beta1.CapacityReservationActive {
			return false, nil
		}
		return true, nil
	}

	requeue, err := checkStatus()
	if err != nil {
		r.Log.Error(err, "Failed to check CapacityReservation Status", "name", clusterCapacityReservation.Name)
		clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationFailed
		clusterCapacityReservation.Status.LifecycleDetail = err.Error()

		// flip ready condition if exists
		if findCondition(clusterCapacityReservation.Status.Conditions, omev1beta1.CapacityReservationReady) != nil {
			ready := omev1beta1.CapacityReservationCondition{
				Type:               omev1beta1.CapacityReservationReady,
				Status:             v1.ConditionStatus(metav1.ConditionFalse),
				Reason:             "Failed",
				Message:            fmt.Sprintf("CapacityReservation failed: %v", err),
				LastTransitionTime: metav1.NewTime(time.Now()),
			}
			setCondition(&clusterCapacityReservation.Status.Conditions, ready)
		}
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Client.Status().Update(context.TODO(), clusterCapacityReservation)
	})
	if err != nil {
		r.Log.Error(err, "Failed to update CapacityReservation Status", "name", clusterCapacityReservation.Name)
		return true, err
	}
	return requeue, nil
}

func (r *CapacityReservationReconciler) updateStatusToFailedWhenResourcesInsufficient(clusterCapacityReservation *omev1beta1.ClusterCapacityReservation, err error) error {
	clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationFailed
	clusterCapacityReservation.Status.LifecycleDetail = err.Error()

	// flip ready condition if exists
	if findCondition(clusterCapacityReservation.Status.Conditions, omev1beta1.CapacityReservationReady) != nil {
		ready := omev1beta1.CapacityReservationCondition{
			Type:               omev1beta1.CapacityReservationReady,
			Status:             v1.ConditionStatus(metav1.ConditionFalse),
			Reason:             "Failed",
			Message:            fmt.Sprintf("CapacityReservation failed: %v", err),
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
		setCondition(&clusterCapacityReservation.Status.Conditions, ready)
	}

	// flip resourcesSufficient condition if exists
	if findCondition(clusterCapacityReservation.Status.Conditions, omev1beta1.ResourcesSufficient) != nil {
		resourcesSufficient := omev1beta1.CapacityReservationCondition{
			Type:               omev1beta1.ResourcesSufficient,
			Status:             v1.ConditionStatus(metav1.ConditionFalse),
			Reason:             "Failed due to resources insufficient",
			Message:            fmt.Sprintf("CapacityReservation failed: %v", err),
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
		setCondition(&clusterCapacityReservation.Status.Conditions, resourcesSufficient)
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Client.Status().Update(context.TODO(), clusterCapacityReservation)
	})
	if err != nil {
		r.Log.Error(err, "Failed to update CapacityReservation Status", "name", clusterCapacityReservation.Name)
		return err
	}
	return nil
}

func (r *CapacityReservationReconciler) handleDeletion(ctx context.Context, clusterCapacityReservation *omev1beta1.ClusterCapacityReservation) error {
	r.Log.Info("Deleting CapacityReservation", "name", clusterCapacityReservation.Name)
	clusterCapacityReservation.Status.CapacityReservationLifecycleState = omev1beta1.CapacityReservationDeleting
	clusterCapacityReservation.Status.LifecycleDetail = string(omev1beta1.CapacityReservationDeleting)

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

func (r *CapacityReservationReconciler) isResourceSufficient(ctx context.Context, clusterCapacityReservation *omev1beta1.ClusterCapacityReservation, desired []kueuev1beta1.ResourceGroup) (bool, error) {
	r.Log.Info("check resource sufficiency for clusterCapacityReservation", "name", clusterCapacityReservation.Name)
	// check whether it is a creation
	var changeMap map[kueuev1beta1.ResourceFlavorReference]map[v1.ResourceName]resource.Quantity

	if clusterCapacityReservation.Status.Capacity != nil {
		// for update request, detect resources change
		changeMap = utils.CompareResourcesChange(desired, clusterCapacityReservation.Status.Capacity)
		if !utils.IsIncreased(changeMap) {
			r.Log.Info("resources are not increased in reconcile request")
			return true, nil
		}
	} else {
		// for creation request
		changeMap = utils.ConvertResourceGroupsToMap(desired)
	}
	r.Log.Info("resourceGroups changes in reconcile request", "changeMap", changeMap)

	// detect resource is increased in request
	// get available resources in cluster
	availableMap, err := utils.GetClusterAvailableResource()
	if err != nil {
		return false, err
	}
	r.Log.Info("available capacities in cluster", "availableMap", availableMap)

	// get the sum of capacities of all active capacity reservations in cluster
	capacityReservationList, err := r.listCapacityReservations(ctx)
	if err != nil {
		return false, err
	}
	capacityMap := utils.GetTotalCapacitiesFromCapacityReservationList(capacityReservationList)
	r.Log.Info("capacities of all capacityReservations in cluster", "capacityMap", capacityMap)

	return utils.IsResourceSufficient(availableMap, capacityMap, changeMap), nil
}

func (r *CapacityReservationReconciler) listCapacityReservations(ctx context.Context) (omev1beta1.ClusterCapacityReservationList, error) {
	var capacityReservationList omev1beta1.ClusterCapacityReservationList
	if err := r.List(ctx, &capacityReservationList); err != nil {
		return omev1beta1.ClusterCapacityReservationList{}, fmt.Errorf("failed to list CapacityReservations: %w", err)
	}
	return capacityReservationList, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CapacityReservationReconciler) SetupWithManager(mgr ctrl.Manager, capacityReservationReconcilePolicyConfig *controllerconfig.CapacityReservationReconcilePolicyConfig) error {
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

	volcanoJobFound, err := generalutils.IsCrdAvailable(r.ClientConfig, volbatchv1alpha1.SchemeGroupVersion.String(), constants.VolcanoJobKind)
	if err != nil {
		return err
	}

	volcanoQueueFound, err := generalutils.IsCrdAvailable(r.ClientConfig, vbatchv1beta1.SchemeGroupVersion.String(), constants.VolcanoQueueKind)
	if err != nil {
		return err
	}

	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&omev1beta1.ClusterCapacityReservation{}).
		Owns(&kueuev1beta1.ClusterQueue{}).
		Watches(
			&omev1beta1.InferenceService{},
			eventHandler,
			builder.WithPredicates(predicates))

	if volcanoJobFound {
		ctrlBuilder.Owns(&volbatchv1alpha1.Job{})
	}
	if volcanoQueueFound {
		ctrlBuilder.Owns(&vbatchv1beta1.Queue{})
	}

	return ctrlBuilder.Complete(r)
}
