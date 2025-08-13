package ratelimit

import (
	"context"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/aip/common"
	"github.com/go-logr/logr"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=ome.io,resources=ratelimits,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=ratelimits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=ratelimits/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=users,verbs=get;list;watch

// RateLimitReconciler reconciles a RateLimit object
type RateLimitReconciler struct {
	client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// Reconcile reads the state of the cluster for a RateLimit object and makes changes based on the state read
// and what is in the RateLimit.Spec. It handles the creation, update, and deletion of rate limits.
func (r *RateLimitReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("ratelimit", request.NamespacedName)

	// Fetch the RateLimit instance
	rateLimit := &v1beta1.RateLimit{}
	if err := r.Client.Get(ctx, request.NamespacedName, rateLimit); err != nil {
		return r.handleGetError(err, log)
	}

	log.Info("Reconciling rate limit", "name", rateLimit.Name, "project", rateLimit.Spec.ProjectRef.Name)

	// Handle deletion
	if !rateLimit.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(rateLimit, constants.RateLimitFinalizerName) {
			// Delete rate limit in external system
			err := r.deleteRateLimit(ctx, rateLimit)
			if err != nil {
				log.Error(err, "failed to delete rate limit")
				return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
			}

			// Remove finalizer
			controllerutil.RemoveFinalizer(rateLimit, constants.RateLimitFinalizerName)
			if err := r.Update(ctx, rateLimit); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(rateLimit, constants.RateLimitFinalizerName) {
		controllerutil.AddFinalizer(rateLimit, constants.RateLimitFinalizerName)
		if err := r.Update(ctx, rateLimit); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate references
	if err := r.validateReferences(ctx, rateLimit); err != nil {
		log.Error(err, "failed to validate references")
		return ctrl.Result{}, err
	}

	// Create or update rate limit
	err := r.reconcileRateLimit(ctx, rateLimit)
	if err != nil {
		log.Error(err, "failed to reconcile rate limit")
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RateLimitReconciler) handleGetError(err error, log logr.Logger) (reconcile.Result, error) {
	if apierr.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	log.Error(err, "unable to fetch RateLimit")
	return ctrl.Result{}, client.IgnoreNotFound(err)
}

func (r *RateLimitReconciler) validateReferences(ctx context.Context, rateLimit *v1beta1.RateLimit) error {
	// Validate project reference
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: rateLimit.Spec.ProjectRef.Namespace,
		Name:      rateLimit.Spec.ProjectRef.Name,
	}, project); err != nil {
		return err
	}

	return nil
}

func (r *RateLimitReconciler) reconcileRateLimit(ctx context.Context, rateLimit *v1beta1.RateLimit) error {
	log := r.Log.WithValues("ratelimit", rateLimit.Name)

	// Initialize rate limit resource
	rateLimitOp, err := common.NewRateLimit(
		ctx,
		r.Client,
		r.Clientset,
		log.WithName("ratelimit"),
		r.Scheme,
		rateLimit,
	)
	if err != nil {
		log.Error(err, "failed to create rate limit operation")
		if common.ShouldRetry(err) {
			return err
		} else {
			return nil
		}
	}

	// Create or update rate limit
	if len(rateLimit.Status.Conditions) == 0 {
		// Create rate limit
		err := rateLimitOp.Create(ctx)
		if err != nil {
			log.Error(err, "failed to create rate limit")
			return err
		}
	} else {
		// Update rate limit
		err := rateLimitOp.Update(ctx)
		if err != nil {
			log.Error(err, "failed to update rate limit")
			return err
		}
	}

	return nil
}

func (r *RateLimitReconciler) deleteRateLimit(ctx context.Context, rateLimit *v1beta1.RateLimit) error {
	log := r.Log.WithValues("ratelimit", rateLimit.Name)

	// Initialize rate limit resource
	rateLimitOp, err := common.NewRateLimit(
		ctx,
		r.Client,
		r.Clientset,
		log.WithName("ratelimit"),
		r.Scheme,
		rateLimit,
	)
	if err != nil {
		log.Error(err, "failed to create rate limit operation for deletion")
		return err
	}

	// Delete rate limit
	err = rateLimitOp.Delete(ctx)
	if err != nil {
		log.Error(err, "failed to delete rate limit")
		return err
	}

	return nil
}

func (r *RateLimitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.RateLimit{}).
		Complete(r)
}
