package project

import (
	"context"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/aip/common"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"github.com/go-logr/logr"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=ome.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=projects/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list

// ProjectService handles OpenAI project operations
type ProjectService struct {
}

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Clientset kubernetes.Interface
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
}

// Reconcile reads the state of the cluster for a Project object and makes changes based on the state read
// and what is in the Project.Spec. It handles the creation and deletion of projects and manages
// the associated API key by creating a Kubernetes Secret.
func (r *ProjectReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("project", request.NamespacedName)

	// Fetch the Project instance
	proj := &v1beta1.Project{}
	if err := r.Client.Get(ctx, request.NamespacedName, proj); err != nil {
		return r.handleGetError(err, log)
	}

	log.Info("Reconciling project", "name", proj.Spec.Name, "organization", proj.Spec.OrganizationRef.Name)

	// Initialize project resource
	project := common.NewProject(
		r.Client,
		r.Clientset,
		log.WithName("project"),
		r.Scheme,
		proj,
	)

	// Handle deletion
	if !proj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(proj, constants.ProjectFinalizerName) {
			if proj.Status.ProjectID != "" {
				// Delete project in OpenAI
				err := project.Delete(ctx)
				if err != nil {
					log.Error(err, "failed to delete project")
					return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
				}
			}

			// Remove finalizer
			controllerutil.RemoveFinalizer(proj, constants.ProjectFinalizerName)
			if err := r.Update(ctx, proj); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(proj, constants.ProjectFinalizerName) {
		controllerutil.AddFinalizer(proj, constants.ProjectFinalizerName)
		if err := r.Update(ctx, proj); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Create or update project
	if proj.Status.ProjectID == "" {
		// Create project
		err := project.Create(ctx)
		if err != nil {
			log.Error(err, "failed to create project")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
	}

	// Only attempt to get the project if we have a project ID
	var existingProject *openaisdk.Project
	var err error
	if proj.Status.ProjectID != "" {
		existingProject, err = project.GetProject(ctx)
		if err != nil {
			log.Error(err, "failed to get project")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}

		if proj.Spec.Name != existingProject.Name {
			// Update project
			err := project.Update(ctx)
			if err != nil {
				log.Error(err, "failed to update project")
				return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) handleGetError(err error, log logr.Logger) (reconcile.Result, error) {
	if apierr.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	log.Error(err, "unable to fetch Project")
	return ctrl.Result{}, client.IgnoreNotFound(err)
}

func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add index for service account owner references
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1beta1.ServiceAccount{}, ".metadata.controller", func(rawObj client.Object) []string {
		sa := rawObj.(*v1beta1.ServiceAccount)
		owner := metav1.GetControllerOf(sa)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != "ome.io/v1beta1" || owner.Kind != "Project" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Project{}).
		Complete(r)
}
