package dac

import (
	"context"
	"fmt"
	"sort"
	"time"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	nsreconciler "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/dac/reconcilers/namespace"
	volcanoJobReconciler "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/dac/reconcilers/volcanojob"
	queueReconciler "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/dac/reconcilers/volcanoqueue"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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
	volbatchv1alpha1 "volcano.sh/apis/pkg/apis/batch/v1alpha1"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
)

// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=ome.io,resources=inferenceservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch.volcano.sh,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.volcano.sh,resources=jobs/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.volcano.sh,resources=jobs/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusterprofiles,verbs=get;list;watch

// DedicatedAIClusterReconciler reconciles a DedicatedAICluster object
// DedicatedAIClusterReconciler reconciles a DedicatedAICluster object
type DedicatedAIClusterReconciler struct {
	client.Client
	DacReconcilePolicy *omev1beta1.DacReconcilePolicyConfig
	ClientConfig       *rest.Config
	Clientset          kubernetes.Interface
	Log                logr.Logger
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
}

func (r *DedicatedAIClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// check if dedicatedAiCluster is ready, if not, create a namespace and mark as ready
	dac := &omev1beta1.DedicatedAICluster{}
	if err := r.Get(ctx, req.NamespacedName, dac); err != nil {
		if apierr.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "unable to get dedicatedAiCluster", "namespace", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if !dac.ObjectMeta.DeletionTimestamp.IsZero() { // dac is under deletion
		if controllerutil.ContainsFinalizer(dac, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("remove dac finalizer", "dac", dac.Name)
			controllerutil.RemoveFinalizer(dac, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), dac); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Initialize mergedSpec with the DAC spec
	mergedSpec := dac.Spec.DeepCopy()

	// If a profile is specified, fetch the corresponding DedicatedAIClusterProfile
	if dac.Spec.Profile != "" {
		profile := &omev1beta1.DedicatedAIClusterProfile{}
		profileNamespacedName := types.NamespacedName{Name: dac.Spec.Profile}

		// Fetch the cluster-scoped DedicatedAIClusterProfile
		if err := r.Get(ctx, profileNamespacedName, profile); err != nil {
			if apierr.IsNotFound(err) {
				r.Log.Error(err, "Profile not found", "profile", dac.Spec.Profile)
				return ctrl.Result{}, err
			}
			r.Log.Error(err, "unable to get DedicatedAIClusterProfile", "profile", dac.Spec.Profile)
			return ctrl.Result{}, err
		}

		// Merge the specs with DAC taking precedence
		mergedSpec = mergeSpecs(&profile.Spec, mergedSpec)
	}

	// Reconcile Namespace
	namespaceReconcile, err := nsreconciler.NewNamespaceReconciler(r.Client, r.Scheme, req.NamespacedName.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if namespaceReconcile.Namespace != nil && !metav1.IsControlledBy(namespaceReconcile.Namespace, dac) {
		r.Log.Info("add namespace controller")
		if err := controllerutil.SetControllerReference(dac, namespaceReconcile.Namespace, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set namespace owner reference for dac")
		}
	}
	namespace, err := namespaceReconcile.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile namespace")
	}
	// Set namespace controller at the first time
	r.Log.Info("namespace", "namespace", namespace)

	queueCount := mergedSpec.Count
	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState {
		if dac.Status.DacLifecycleState == omev1beta1.FAILED {
			queueCount = 0
		}
	}

	volcanoQueueReconcile, err := queueReconciler.NewQueueReconciler(r.Client, r.Scheme, req.NamespacedName.Name, mergedSpec.Resources, mergedSpec.Affinity, queueCount)
	if err != nil {
		return ctrl.Result{}, err
	}

	if volcanoQueueReconcile.Queue != nil && !metav1.IsControlledBy(volcanoQueueReconcile.Queue, dac) {
		r.Log.Info("add queue controller")
		if err := controllerutil.SetControllerReference(dac, volcanoQueueReconcile.Queue, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set queue owner reference for dac")
		}
	}
	queue, err := volcanoQueueReconcile.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile queue")
	}

	replicaCount, err := r.GetDesiredReservationReplicaCount(dac, mergedSpec.Count)
	if err != nil {
		return ctrl.Result{}, err
	}

	reservationJobReconciler, err := volcanoJobReconciler.NewReservationJobReconciler(r.Client, r.Scheme, req.NamespacedName.Name, mergedSpec.Resources, mergedSpec.Affinity, replicaCount)
	if err != nil {
		return ctrl.Result{}, err
	}

	if reservationJobReconciler.ReservationJob != nil && !metav1.IsControlledBy(reservationJobReconciler.ReservationJob, dac) {
		r.Log.Info("add reservation job controller")
		if err := controllerutil.SetControllerReference(dac, reservationJobReconciler.ReservationJob, r.Scheme); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to set reservation job owner reference for dac")
		}
	}

	reservationJob, err := reservationJobReconciler.Reconcile()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile reservation job")
	}

	requeue, err := r.updateDedicatedAIClusterStatus(dac, queue, reservationJob, reservationJobReconciler.CreationFailedTimeThreshold)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrapf(err, "failed to update the status of DadicatedAICluster %s", dac.Name)
	}
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}

	if dac.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(dac, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("add dac finalizer", "dac", dac.Name)
			controllerutil.AddFinalizer(dac, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), dac); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(namespace, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("remove namespace finalizer")
			controllerutil.RemoveFinalizer(namespace, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), namespace); err != nil {
				r.Log.Error(err, "failed to remove namespace finalizer")
			}
		}
		if controllerutil.ContainsFinalizer(dac, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("remove dac finalizer", "dac", dac.Name)
			controllerutil.RemoveFinalizer(dac, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), dac); err != nil {
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(queue, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("remove queue finalizer")
			controllerutil.RemoveFinalizer(queue, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), queue); err != nil {
				r.Log.Error(err, "failed to remove queue finalizer")
			}
		}
		if controllerutil.ContainsFinalizer(reservationJob, constants.DedicatedAiClusterFinalizer) {
			r.Log.Info("remove reservationJob finalizer")
			controllerutil.RemoveFinalizer(reservationJob, constants.DedicatedAiClusterFinalizer)
			if err := r.Update(context.Background(), reservationJob); err != nil {
				r.Log.Error(err, "failed to remove reservationJob finalizer")
			}
		}
	}

	return ctrl.Result{}, nil
}

// mergeSpecs merges the profile spec with the DAC spec, giving priority to DAC fields.
func mergeSpecs(profileSpec *omev1beta1.DedicatedAIClusterProfileSpec, dacSpec *omev1beta1.DedicatedAIClusterSpec) *omev1beta1.DedicatedAIClusterSpec {

	// Merge Resources
	if dacSpec.Resources == nil {
		dacSpec.Resources = &profileSpec.Resources
	} else {
		if dacSpec.Resources.Requests == nil {
			dacSpec.Resources.Requests = profileSpec.Resources.Requests
		}
		if dacSpec.Resources.Limits == nil {
			dacSpec.Resources.Limits = profileSpec.Resources.Limits
		}
	}

	// Merge Affinity
	if dacSpec.Affinity == nil {
		dacSpec.Affinity = profileSpec.Affinity
	}

	// Merge Tolerations
	if dacSpec.Tolerations == nil {
		dacSpec.Tolerations = profileSpec.Tolerations
	}

	// Merge NodeSelector
	if dacSpec.NodeSelector == nil {
		dacSpec.NodeSelector = profileSpec.NodeSelector
	}

	// Merge PriorityClassName
	if dacSpec.PriorityClassName == "" {
		dacSpec.PriorityClassName = profileSpec.PriorityClassName
	}

	// Merge Count
	if dacSpec.Count == 0 {
		dacSpec.Count = profileSpec.Count
	}

	return dacSpec
}

func (r *DedicatedAIClusterReconciler) updateDedicatedAIClusterStatus(
	dac *omev1beta1.DedicatedAICluster,
	queue *schedulingv1beta1.Queue,
	reservationJob *volbatchv1alpha1.Job,
	creationFailedTimeThreshold time.Duration) (bool, error) {

	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState {
		if dac.Status.DacLifecycleState == omev1beta1.FAILED {
			return false, nil
		}
	}

	checkStatus := func() (bool, error) {
		if reservationJob.Status.State.Phase == volbatchv1alpha1.Running {
			dac.Status.DacLifecycleState = omev1beta1.ACTIVE
			dac.Status.LifecycleDetail = string(omev1beta1.ACTIVE)
		} else {
			if queue.Status.Running == 0 { // nothing could be allocated
				condition, hasScheduled, err := r.getFailedReservationPodGroupCondition(reservationJob)
				if err != nil {
					return false, err
				}

				if condition != nil {
					if condition.Type == schedulingv1beta1.PodGroupUnschedulableType {
						if hasScheduled {
							dac.Status.DacLifecycleState = omev1beta1.UPDATING
							dac.Status.LifecycleDetail = condition.Reason
						} else {
							if reservationJob.CreationTimestamp.Add(creationFailedTimeThreshold).Before(time.Now()) {
								if shouldMarkFailed(dac) {
									dac.Status.DacLifecycleState = omev1beta1.FAILED
									dac.Status.LifecycleDetail = condition.Reason
								}
							} else {
								dac.Status.DacLifecycleState = omev1beta1.CREATING
								dac.Status.LifecycleDetail = string(omev1beta1.CREATING)
							}
						}
					} else {
						return false, fmt.Errorf("need further investigation on the volcanoJob %s condition", reservationJob.Name)
					}
				} else {
					if reservationJob.CreationTimestamp.Add(creationFailedTimeThreshold).Before(time.Now()) {
						if shouldMarkFailed(dac) {
							dac.Status.DacLifecycleState = omev1beta1.FAILED
							dac.Status.LifecycleDetail = "NotEnoughResources"
						}
					} else {
						dac.Status.DacLifecycleState = omev1beta1.CREATING
						dac.Status.LifecycleDetail = string(omev1beta1.CREATING)
					}
				}
			} else {
				dac.Status.DacLifecycleState = omev1beta1.ACTIVE
				dac.Status.LifecycleDetail = string(omev1beta1.ACTIVE)
			}
		}

		if dac.Status.DacLifecycleState == omev1beta1.FAILED || dac.Status.DacLifecycleState == omev1beta1.ACTIVE {
			return false, nil
		} else {
			return true, nil
		}
	}

	requeue, err := checkStatus()
	if err != nil {
		dac.Status.DacLifecycleState = omev1beta1.FAILED
		dac.Status.LifecycleDetail = err.Error()
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		err := r.Client.Status().Update(context.TODO(), dac)
		if err != nil {
			return err
		}
		if err != nil {
			r.Log.Error(err, "Failed to update DedicatedAICluster Status", "DedicatedAICluster", dac.Name)
			return err
		}
		err = r.Client.Status().Update(context.TODO(), dac)
		if err != nil {
			return err
		}
		if err != nil {
			r.Log.Error(err, "Failed to update DedicatedAICluster Status", "DedicatedAICluster", dac.Name)
			return err
		}
		return nil
	})
	if err != nil {
		r.Log.Error(err, "Failed to update DedicatedAICluster Status", "DedicatedAICluster", dac.Name)
		return false, err
	}
	return requeue, nil
}

func shouldMarkFailed(dac *omev1beta1.DedicatedAICluster) bool {
	return dac.Status.DacLifecycleState == omev1beta1.CREATING || dac.Status.DacLifecycleState == ""
}

func (r *DedicatedAIClusterReconciler) getFailedReservationPodGroupCondition(
	reservationJob *volbatchv1alpha1.Job) (*schedulingv1beta1.PodGroupCondition, bool, error) {

	existingPodGroup := &schedulingv1beta1.PodGroup{}
	podGroupName := fmt.Sprintf("%s-%s", reservationJob.Name, reservationJob.UID)
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: podGroupName, Namespace: reservationJob.Namespace}, existingPodGroup)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var hasScheduled bool = false
	if existingPodGroup.Status.Phase == schedulingv1beta1.PodGroupPending ||
		existingPodGroup.Status.Phase == schedulingv1beta1.PodGroupUnknown ||
		existingPodGroup.Status.Phase == schedulingv1beta1.PodGroupInqueue {
		conditions := existingPodGroup.Status.Conditions
		if len(conditions) == 0 {
			return nil, false, nil
		} else {
			sort.Slice(conditions, func(a, b int) bool {
				return conditions[a].LastTransitionTime.After(conditions[b].LastTransitionTime.Time)
			})

			for _, c := range conditions {
				if c.Type == schedulingv1beta1.PodGroupScheduled {
					hasScheduled = true
					break
				}
			}
			return &conditions[0], hasScheduled, nil
		}
	}

	return nil, false, nil
}

func (r *DedicatedAIClusterReconciler) GetDesiredReservationReplicaCount(dac *omev1beta1.DedicatedAICluster, reservationCount int) (int, error) {
	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState {
		if dac.Status.DacLifecycleState == omev1beta1.FAILED {
			return 0, nil
		}
	}

	var baseCount int
	if len(dac.Spec.Profile) > 0 {
		dacProfile := &omev1beta1.DedicatedAIClusterProfile{}
		err := r.Get(context.TODO(), types.NamespacedName{Name: dac.Spec.Profile}, dacProfile)
		if err != nil {
			if apierr.IsNotFound(err) {
				r.Log.Error(err, "Failed to find the DedicatedAICluster Profile ", dac.Spec.Profile, " DedicatedAICluster", dac.Name)
			}
			return 0, err
		}

		baseCount = dacProfile.Spec.Count
	} else {
		baseCount = 1
	}

	isvcList := &omev1beta1.InferenceServiceList{}
	if err := r.List(context.TODO(), isvcList, client.InNamespace(dac.Name)); err != nil {
		return 0, err
	}

	if len(isvcList.Items) == 0 {
		return reservationCount, nil
	}

	var totalIsvcOccupation int = 0
	for _, isvc := range isvcList.Items {
		totalIsvcOccupation += (isvc.Spec.Predictor.ComponentExtensionSpec.MaxReplicas * baseCount)
	}

	if reservationCount-totalIsvcOccupation < 0 {
		return 0, nil
	}
	return reservationCount - totalIsvcOccupation, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DedicatedAIClusterReconciler) SetupWithManager(mgr ctrl.Manager, dacReconcilePolicyConfig *omev1beta1.DacReconcilePolicyConfig) error {
	r.ClientConfig = mgr.GetConfig()
	r.DacReconcilePolicy = dacReconcilePolicyConfig

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
		For(&omev1beta1.DedicatedAICluster{}).
		Owns(&corev1.Namespace{}).
		Owns(&schedulingv1beta1.Queue{}).
		Owns(&volbatchv1alpha1.Job{}).
		Watches(
			&omev1beta1.InferenceService{},
			eventHandler,
			builder.WithPredicates(predicates)).
		Complete(r)
}
