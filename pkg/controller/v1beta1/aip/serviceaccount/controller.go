package serviceaccount

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/aip/common"
)

// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

const finalizerName = "serviceaccount.ome.io.finalizers"

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// Reconcile reads the state of the cluster for a ServiceAccount object and makes changes based on the state read
func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("serviceaccount", request.NamespacedName)

	// Fetch the ServiceAccount instance
	sa := &v1beta1.ServiceAccount{}
	if err := r.Get(ctx, request.NamespacedName, sa); err != nil {
		return r.handleGetError(err, log)
	}

	log.Info("Reconciling service account", "name", sa.Spec.Name, "project", sa.Spec.ProjectRef.Name)

	// Initialize resource handler
	resource := common.NewServiceAccount(r.Client, r.Clientset, log, r.Scheme, sa)

	// Ensure finalizer
	if err := r.ensureFinalizer(ctx, sa); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer: %w", err)
	}

	// Handle deletion
	if !sa.DeletionTimestamp.IsZero() {
		if err := resource.Delete(ctx); err != nil {
			// Log without stack trace for better readability
			log.Info("Failed to delete service account, will retry",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"error", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}

		controllerutil.RemoveFinalizer(sa, finalizerName)
		if err := r.Update(ctx, sa); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Handle creation/update
	if sa.Status.ServiceAccountID == nil {
		if err := resource.Create(ctx); err != nil {
			// Log without stack trace for better readability
			log.Info("Failed to create service account, will retry",
				"name", sa.Name,
				"namespace", sa.Namespace,
				"error", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	}

	// Update if needed
	if err := resource.Update(ctx, sa); err != nil {
		// Log without stack trace for better readability
		log.Info("Failed to update service account, will retry",
			"name", sa.Name,
			"namespace", sa.Namespace,
			"error", err.Error())
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) handleGetError(err error, log logr.Logger) (reconcile.Result, error) {
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	log.Error(err, "Failed to get ServiceAccount")
	return ctrl.Result{}, err
}

func (r *ServiceAccountReconciler) ensureFinalizer(ctx context.Context, sa *v1beta1.ServiceAccount) error {
	if !controllerutil.ContainsFinalizer(sa, finalizerName) {
		controllerutil.AddFinalizer(sa, finalizerName)
		return r.Update(ctx, sa)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ServiceAccount{}).
		Complete(r)
}
