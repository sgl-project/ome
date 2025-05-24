package dac

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	v1beta2 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"
	appsv1 "k8s.io/api/apps/v1"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	kueueQueueReconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/reconcilers/kueuequeue"
	kueueWorkloadReconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/reconcilers/kueueworkload"
	nsreconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/reconcilers/namespace"
	volcanoJobReconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/reconcilers/volcanojob"
	queueReconciler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/reconcilers/volcanoqueue"
	generalutils "github.com/sgl-project/sgl-ome/pkg/utils"
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
// +kubebuilder:rbac:groups=ome.io,resources=trainingjobs,verbs=get;list;watch
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
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=localqueues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads,verbs=get;list;watch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=localqueues/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=clusterqueues/finalizers,verbs=update
// +kubebuilder:rbac:groups=kueue.x-k8s.io,resources=localqueues/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusterprofiles,verbs=get;list;watch

// DedicatedAIClusterReconciler reconciles a DedicatedAICluster object
type DedicatedAIClusterReconciler struct {
	client.Client
	DacReconcilePolicy *controllerconfig.DacReconcilePolicyConfig
	ClientConfig       *rest.Config
	Clientset          kubernetes.Interface
	Log                logr.Logger
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
}

func (r *DedicatedAIClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// check if dedicatedAiCluster is ready, if not, create a namespace and mark as ready
	dac := &v1beta2.DedicatedAICluster{}
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
		profile := &v1beta2.DedicatedAIClusterProfile{}
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
		mergedSpec = MergeSpecs(&profile.Spec, mergedSpec)
	}

	// Determine if reconciling with Kueue by checking both if the Volcano queue is present and if the enableKueue flag is set as true
	isVolcanoQueuePresent, err := utils.IsVolcanoQueuePresent(r.Client, r.ClientConfig, req.NamespacedName.Name)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get volcano queue")
	}
	enableKueue := false
	if r.DacReconcilePolicy.ReconcileWithKueue && !isVolcanoQueuePresent {
		enableKueue = true
	}

	// Reconcile Namespace
	namespaceReconcile, err := nsreconciler.NewNamespaceReconciler(r.Client, r.Scheme, req.NamespacedName.Name, enableKueue)
	if err != nil {
		return ctrl.Result{}, err
	}
	if namespaceReconcile.Namespace != nil && !metav1.IsControlledBy(namespaceReconcile.Namespace, dac) {
		r.Log.Info("add namespace controller", "namespace", namespaceReconcile.Namespace.Name)
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

	// Check if DAC is supposed to be Active while it is Failed due to tight capacity
	isCapacityReserved, _ := utils.IsCapacityReserved(dac)

	queueCount := mergedSpec.Count
	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState || !isCapacityReserved {
		if dac.Status.DacLifecycleState == v1beta2.FAILED {
			queueCount = 0
		}
	}

	replicaCount, err := r.GetDesiredReservationReplicaCount(dac, mergedSpec.Count, isCapacityReserved)
	if err != nil {
		return ctrl.Result{}, err
	}

	var volcanoQueue *schedulingv1beta1.Queue
	var volcanoReservationJob *volbatchv1alpha1.Job
	var kueueClusterQueue *kueuev1beta1.ClusterQueue
	var kueueLocalQueue *kueuev1beta1.LocalQueue
	var deployment *appsv1.Deployment
	if !enableKueue {
		// Reconcile Volcano queue
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
		volcanoQueue, err = volcanoQueueReconcile.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile queue")
		}

		// Reconcile Volcano Job for reservation
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
		volcanoReservationJob, err = reservationJobReconciler.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile reservation job")
		}
		// Update DAC status
		requeue, err := r.updateDedicatedAIClusterStatus(dac, volcanoQueue, volcanoReservationJob, reservationJobReconciler.CreationFailedTimeThreshold, isCapacityReserved)
		if err != nil {
			return ctrl.Result{Requeue: true}, errors.Wrapf(err, "failed to update the status of DadicatedAICluster %s", dac.Name)
		}
		if requeue {
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// Reconcile Kueue ClusterQueue
		kueueClusterQueueReconcile := kueueQueueReconciler.NewClusterQueueReconciler(r.Client, r.Scheme, req.NamespacedName.Name, mergedSpec.Resources, queueCount)
		if kueueClusterQueueReconcile.ClusterQueue != nil && !metav1.IsControlledBy(kueueClusterQueueReconcile.ClusterQueue, dac) {
			r.Log.Info("add kueue cluster queue controller")
			if err := controllerutil.SetControllerReference(dac, kueueClusterQueueReconcile.ClusterQueue, r.Scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "failed to set kueue cluster queue owner reference for dac")
			}
		}
		kueueClusterQueue, err = kueueClusterQueueReconcile.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile Kueue cluster queue")
		}

		// Reconcile Kueue LocalQueue
		kueueLocalQueueReconcile := kueueQueueReconciler.NewLocalQueueReconciler(r.Client, r.Scheme, req.NamespacedName.Name)
		if kueueLocalQueueReconcile.LocalQueue != nil && !metav1.IsControlledBy(kueueLocalQueueReconcile.LocalQueue, dac) {
			r.Log.Info("add kueue local queue controller")
			if err := controllerutil.SetControllerReference(dac, kueueLocalQueueReconcile.LocalQueue, r.Scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "failed to set kueue local queue owner reference for dac")
			}
		}
		kueueLocalQueue, err = kueueLocalQueueReconcile.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile Kueue local queue")
		}

		// Reconcile Kueue deployment workload for reservation
		kueueDeploymentReconciler, err := kueueWorkloadReconciler.NewDeploymentReconciler(r.Client, r.Clientset, r.Scheme, req.NamespacedName.Name, mergedSpec.Resources, mergedSpec.Affinity, replicaCount)
		if err != nil {
			return ctrl.Result{}, err
		}
		if kueueDeploymentReconciler.Deployment != nil && !metav1.IsControlledBy(kueueDeploymentReconciler.Deployment, dac) {
			r.Log.Info("add kueue deployment controller")
			if err := controllerutil.SetControllerReference(dac, kueueDeploymentReconciler.Deployment, r.Scheme); err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "failed to set kueue deployment owner reference for dac")
			}
		}
		deployment, err = kueueDeploymentReconciler.Reconcile()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile Kueue reservation deployment")
		}

		// Update DAC status
		requeue, err := r.updateDedicatedAIClusterStatusUnderKueueCase(dac, deployment, replicaCount, kueueDeploymentReconciler.ReservationWorkloadConfig.CreationFailedTimeThresholdSecond, isCapacityReserved)
		if err != nil {
			return ctrl.Result{Requeue: true}, errors.Wrapf(err, "failed to update the status of DadicatedAICluster %s", dac.Name)
		}
		if requeue {
			return ctrl.Result{Requeue: true}, nil
		}
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

		if !enableKueue {
			if controllerutil.ContainsFinalizer(volcanoQueue, constants.DedicatedAiClusterFinalizer) {
				r.Log.Info("remove queue finalizer")
				controllerutil.RemoveFinalizer(volcanoQueue, constants.DedicatedAiClusterFinalizer)
				if err := r.Update(context.Background(), volcanoQueue); err != nil {
					r.Log.Error(err, "failed to remove queue finalizer")
				}
			}
			if controllerutil.ContainsFinalizer(volcanoReservationJob, constants.DedicatedAiClusterFinalizer) {
				r.Log.Info("remove reservationJob finalizer")
				controllerutil.RemoveFinalizer(volcanoReservationJob, constants.DedicatedAiClusterFinalizer)
				if err := r.Update(context.Background(), volcanoReservationJob); err != nil {
					r.Log.Error(err, "failed to remove reservationJob finalizer")
				}
			}
		} else {
			if controllerutil.ContainsFinalizer(kueueClusterQueue, constants.DedicatedAiClusterFinalizer) {
				r.Log.Info("remove kueue cluster queue finalizer")
				controllerutil.RemoveFinalizer(kueueClusterQueue, constants.DedicatedAiClusterFinalizer)
				if err := r.Update(context.Background(), kueueClusterQueue); err != nil {
					r.Log.Error(err, "failed to remove kueue cluster queue finalizer")
				}
			}
			if controllerutil.ContainsFinalizer(kueueLocalQueue, constants.DedicatedAiClusterFinalizer) {
				r.Log.Info("remove kueue local queue finalizer")
				controllerutil.RemoveFinalizer(kueueLocalQueue, constants.DedicatedAiClusterFinalizer)
				if err := r.Update(context.Background(), kueueLocalQueue); err != nil {
					r.Log.Error(err, "failed to remove kueue local queue finalizer")
				}
			}
			if controllerutil.ContainsFinalizer(deployment, constants.DedicatedAiClusterFinalizer) {
				r.Log.Info("remove deployment finalizer")
				controllerutil.RemoveFinalizer(deployment, constants.DedicatedAiClusterFinalizer)
				if err := r.Update(context.Background(), deployment); err != nil {
					r.Log.Error(err, "failed to remove deployment finalizer")
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// MergeSpecs merges the profile spec with the DAC spec, giving priority to DAC fields.
func MergeSpecs(profileSpec *v1beta2.DedicatedAIClusterProfileSpec, dacSpec *v1beta2.DedicatedAIClusterSpec) *v1beta2.DedicatedAIClusterSpec {

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

func (r *DedicatedAIClusterReconciler) updateDedicatedAIClusterStatusUnderKueueCase(
	dac *v1beta2.DedicatedAICluster,
	deployment *appsv1.Deployment,
	replicasCount int,
	failureThresholdInSeconds int,
	isCapacityReserved bool,
) (bool, error) {
	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState || !isCapacityReserved {
		if dac.Status.DacLifecycleState == v1beta2.FAILED {
			return false, nil
		}
	}

	checkStatus := func() (bool, error) {
		if int(deployment.Status.AvailableReplicas) == replicasCount {
			dac.Status.DacLifecycleState = v1beta2.ACTIVE
			dac.Status.LifecycleDetail = string(v1beta2.ACTIVE)
			return false, nil
		} else {
			lastUpdateTimeStr, lastUpdateTimeExist := deployment.ObjectMeta.Annotations[constants.DACLastUpdateTimeAnnotationKey]
			failureThresholdSecondsDuration := time.Duration(failureThresholdInSeconds) * time.Second
			if !lastUpdateTimeExist {
				lastUpdateTime := deployment.CreationTimestamp.Time
				if time.Since(lastUpdateTime) <= failureThresholdSecondsDuration {
					dac.Status.DacLifecycleState = v1beta2.CREATING
					dac.Status.LifecycleDetail = string(v1beta2.CREATING)
					return true, nil
				}
			} else {
				lastUpdateTime, err := time.Parse(time.RFC3339, lastUpdateTimeStr)
				if err != nil {
					return false, err
				}
				if time.Since(lastUpdateTime) <= failureThresholdSecondsDuration {
					dac.Status.DacLifecycleState = v1beta2.UPDATING
					dac.Status.LifecycleDetail = string(v1beta2.UPDATING)
					return true, nil
				}
			}
			// Handle failed case
			dac.Status.DacLifecycleState = v1beta2.FAILED
			failureReason, err := r.getPodsFailureReason(deployment, dac.Namespace)
			if err != nil {
				return false, err
			}
			dac.Status.LifecycleDetail = failureReason
			return false, nil
		}
	}

	requeue, err := checkStatus()
	if err != nil {
		dac.Status.DacLifecycleState = v1beta2.FAILED
		dac.Status.LifecycleDetail = err.Error()
	}

	attempt := 0
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		attempt++ // Increment attempt counter
		err := r.Client.Status().Update(context.TODO(), dac)
		if err != nil {
			r.Log.Error(err, "Failed to update DedicatedAICluster Status",
				"DedicatedAICluster", dac.Name,
				"Attempt", attempt)
		}
		return err
	})
	if err != nil {
		r.Log.Error(err, "Failed to update DedicatedAICluster Status", "DedicatedAICluster", dac.Name)
		return false, err
	}

	if dac.Status.DacLifecycleState == v1beta2.ACTIVE {
		if err = r.addExtraLabels(dac); err != nil {
			return false, err
		}
	}
	return requeue, nil
}

func (r *DedicatedAIClusterReconciler) addExtraLabels(dac *v1beta2.DedicatedAICluster) error {
	if dac.ObjectMeta.Labels == nil {
		dac.ObjectMeta.Labels = map[string]string{
			constants.DACCapacityReservedLabelKey: "true",
			constants.KueueEnabledLabelKey:        "true",
		}
	} else {
		dac.ObjectMeta.Labels[constants.DACCapacityReservedLabelKey] = "true"
		dac.ObjectMeta.Labels[constants.KueueEnabledLabelKey] = "true"
	}
	if err := r.Update(context.TODO(), dac); err != nil {
		r.Log.Error(err, "failed to add labels to dedicatedAiCluster", "DedicatedAICluster", dac.Name)
		return err
	}
	return nil
}

func (r *DedicatedAIClusterReconciler) updateDedicatedAIClusterStatus(
	dac *v1beta2.DedicatedAICluster,
	queue *schedulingv1beta1.Queue,
	reservationJob *volbatchv1alpha1.Job,
	creationFailedTimeThreshold time.Duration,
	isCapacityReserved bool) (bool, error) {

	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState || !isCapacityReserved {
		if dac.Status.DacLifecycleState == v1beta2.FAILED {
			return false, nil
		}
	}

	checkStatus := func() (bool, error) {
		if reservationJob.Status.State.Phase == volbatchv1alpha1.Running {
			dac.Status.DacLifecycleState = v1beta2.ACTIVE
			dac.Status.LifecycleDetail = string(v1beta2.ACTIVE)
		} else {
			if queue.Status.Running == 0 { // nothing could be allocated
				condition, hasScheduled, err := r.getFailedReservationPodGroupCondition(reservationJob)
				if err != nil {
					return false, err
				}

				if condition != nil {
					if condition.Type == schedulingv1beta1.PodGroupUnschedulableType {
						if hasScheduled {
							dac.Status.DacLifecycleState = v1beta2.UPDATING
							dac.Status.LifecycleDetail = condition.Reason
						} else {
							if reservationJob.CreationTimestamp.Add(creationFailedTimeThreshold).Before(time.Now()) {
								if shouldMarkFailed(dac) {
									dac.Status.DacLifecycleState = v1beta2.FAILED
									dac.Status.LifecycleDetail = condition.Reason
								}
							} else {
								dac.Status.DacLifecycleState = v1beta2.CREATING
								dac.Status.LifecycleDetail = string(v1beta2.CREATING)
							}
						}
					} else {
						return false, fmt.Errorf("need further investigation on the volcanoJob %s condition", reservationJob.Name)
					}
				} else {
					if reservationJob.CreationTimestamp.Add(creationFailedTimeThreshold).Before(time.Now()) {
						if shouldMarkFailed(dac) {
							dac.Status.DacLifecycleState = v1beta2.FAILED
							dac.Status.LifecycleDetail = "NotEnoughResources"
						}
					} else {
						dac.Status.DacLifecycleState = v1beta2.CREATING
						dac.Status.LifecycleDetail = string(v1beta2.CREATING)
					}
				}
			} else {
				dac.Status.DacLifecycleState = v1beta2.ACTIVE
				dac.Status.LifecycleDetail = string(v1beta2.ACTIVE)
			}
		}

		if dac.Status.DacLifecycleState == v1beta2.FAILED || dac.Status.DacLifecycleState == v1beta2.ACTIVE {
			return false, nil
		} else {
			return true, nil
		}
	}

	requeue, err := checkStatus()
	if err != nil {
		dac.Status.DacLifecycleState = v1beta2.FAILED
		dac.Status.LifecycleDetail = err.Error()
	}

	attempt := 0
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		attempt++ // Increment attempt counter
		err := r.Client.Status().Update(context.TODO(), dac)
		if err != nil {
			r.Log.Error(err, "Failed to update DedicatedAICluster Status",
				"DedicatedAICluster", dac.Name,
				"Attempt", attempt)
		}
		return err
	})
	if err != nil {
		r.Log.Error(err, "Failed to update DedicatedAICluster Status", "DedicatedAICluster", dac.Name)
		return false, err
	}
	return requeue, nil
}

func shouldMarkFailed(dac *v1beta2.DedicatedAICluster) bool {
	return dac.Status.DacLifecycleState == v1beta2.CREATING || dac.Status.DacLifecycleState == ""
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

func (r *DedicatedAIClusterReconciler) GetDesiredReservationReplicaCount(dac *v1beta2.DedicatedAICluster, reservationCount int, isCapacityReserved bool) (int, error) {
	if !r.DacReconcilePolicy.ReconcileFailedLifecycleState || !isCapacityReserved {
		if dac.Status.DacLifecycleState == v1beta2.FAILED {
			return 0, nil
		}
	}

	var baseCount int
	if len(dac.Spec.Profile) > 0 {
		dacProfile := &v1beta2.DedicatedAIClusterProfile{}
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

	isvcList := &v1beta2.InferenceServiceList{}
	if err := r.List(context.TODO(), isvcList, client.InNamespace(dac.Name)); err != nil {
		return reservationCount, err
	}

	if len(isvcList.Items) == 0 {
		// Check if there is any training job (under target namespace) running in progress
		trainingPodList := &v1beta2.TrainingJobList{}
		if err := r.List(context.TODO(), trainingPodList, client.InNamespace(dac.Name)); err != nil {
			return reservationCount, err
		}

		trainingJobCompleteCount := 0
		for _, trainingJob := range trainingPodList.Items {
			completeCondition := metav1.ConditionFalse
			for _, condition := range trainingJob.Status.Conditions {
				if condition.Type == "Complete" && condition.Status == metav1.ConditionTrue {
					completeCondition = metav1.ConditionTrue
				}
			}
			if completeCondition == metav1.ConditionTrue {
				trainingJobCompleteCount++
			}
		}

		if trainingJobCompleteCount == len(trainingPodList.Items) {
			return reservationCount, nil
		} else {
			return 0, nil
		}
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

func (r *DedicatedAIClusterReconciler) getPodsFailureReason(deployment *appsv1.Deployment, namespace string) (string, error) {
	podList := corev1.PodList{}
	selectedLabel := deployment.Spec.Selector.MatchLabels
	if err := r.List(context.TODO(), &podList, client.InNamespace(namespace), client.MatchingLabels(selectedLabel)); err != nil {
		r.Log.Error(err, "Failed to list pods under reservation deployment", "with label", selectedLabel, "DedicatedAICluster", namespace)
		return "", err
	}

	r.Log.Info("podList", "podList", podList)
	for _, pod := range podList.Items {
		for _, podCondition := range pod.Status.Conditions {
			if podCondition.Type == corev1.PodScheduled && (podCondition.Status == corev1.ConditionFalse || podCondition.Status == corev1.ConditionUnknown) {
				r.Log.Error(fmt.Errorf("reservation pod scheduling failed"), "DedicatedAICluster", namespace, "podName", pod.Name, "message", podCondition.Message)
				return "NotEnoughResources", nil
			}
		}
	}
	// Only report back the error caused by scheduling issue due to resource shortage, other errors just return "FAILED"
	return string(v1beta2.FAILED), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DedicatedAIClusterReconciler) SetupWithManager(mgr ctrl.Manager, dacReconcilePolicyConfig *controllerconfig.DacReconcilePolicyConfig) error {
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

	volcanoJobFound, err := generalutils.IsCrdAvailable(r.ClientConfig, volbatchv1alpha1.SchemeGroupVersion.String(), constants.VolcanoJobKind)
	if err != nil {
		return err
	}

	volcanoQueueFound, err := generalutils.IsCrdAvailable(r.ClientConfig, schedulingv1beta1.SchemeGroupVersion.String(), constants.VolcanoQueueKind)
	if err != nil {
		return err
	}

	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.DedicatedAICluster{}).
		Owns(&corev1.Namespace{}).
		Owns(&kueuev1beta1.ClusterQueue{}).
		Owns(&kueuev1beta1.LocalQueue{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			&v1beta2.InferenceService{},
			eventHandler,
			builder.WithPredicates(predicates)).
		Watches(
			&v1beta2.TrainingJob{},
			eventHandler,
			builder.WithPredicates(predicates))

	if volcanoJobFound {
		ctrlBuilder.Owns(&volbatchv1alpha1.Job{})
	} else {
		r.Log.Info("The DAC controller won't watch batch.volcano.sh/v1alpha1/Job resources because the CRD is not available.")
	}

	if volcanoQueueFound {
		ctrlBuilder.Owns(&schedulingv1beta1.Queue{})
	} else {
		r.Log.Info("The DAC controller won't watch scheduling.volcano.sh/v1beta1/Queue resources because the CRD is not available.")
	}

	return ctrlBuilder.Complete(r)
}
