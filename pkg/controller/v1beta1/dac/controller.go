package dac

import (
	omev1beta1 "bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	nsreconciler "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/dac/reconcilers/namespace"
	queueReconciler "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/dac/reconcilers/volcanoqueue"
	"context"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
// +kubebuilder:rbac:groups=ome.io,resources=dedicatedaiclusterprofiles,verbs=get;list;watch

// DedicatedAIClusterReconciler reconciles a DedicatedAICluster object
// DedicatedAIClusterReconciler reconciles a DedicatedAICluster object
type DedicatedAIClusterReconciler struct {
	client.Client
	ClientConfig *rest.Config
	Clientset    kubernetes.Interface
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
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

	volcanoQueueReconcile, err := queueReconciler.NewQueueReconciler(r.Client, r.Scheme, req.NamespacedName.Name, mergedSpec.Resources, mergedSpec.Affinity)
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

	dacFinalizer := "dedicatedaiclusters.ome.io/finalizer"
	if dac.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(dac, dacFinalizer) {
			r.Log.Info("add dac finalizer", "dac", dac.Name)
			controllerutil.AddFinalizer(dac, dacFinalizer)
			if err := r.Update(context.Background(), dac); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(namespace, dacFinalizer) {
			r.Log.Info("remove namespace finalizer")
			controllerutil.RemoveFinalizer(namespace, dacFinalizer)
			if err := r.Update(context.Background(), namespace); err != nil {
				r.Log.Error(err, "failed to remove namespace finalizer")
			}
		}
		if controllerutil.ContainsFinalizer(dac, dacFinalizer) {
			r.Log.Info("remove dac finalizer", "dac", dac.Name)
			controllerutil.RemoveFinalizer(dac, dacFinalizer)
			if err := r.Update(context.Background(), dac); err != nil {
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(queue, dacFinalizer) {
			r.Log.Info("remove queue finalizer")
			controllerutil.RemoveFinalizer(queue, dacFinalizer)
			if err := r.Update(context.Background(), queue); err != nil {
				r.Log.Error(err, "failed to remove queue finalizer")
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

// SetupWithManager sets up the controller with the Manager.
func (r *DedicatedAIClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ClientConfig = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&omev1beta1.DedicatedAICluster{}).
		Owns(&corev1.Namespace{}).
		Owns(&schedulingv1beta1.Queue{}).
		Complete(r)
}
