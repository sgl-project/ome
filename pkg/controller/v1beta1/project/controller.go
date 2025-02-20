package project

import (
	"context"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
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
	// Fetch the Project instance
	p := &v1beta1.Project{}
	if err := r.Client.Get(ctx, request.NamespacedName, p); err != nil {
		if apierr.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		r.Log.Error(err, "unable to fetch Project")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Fetched ProjectSpec", "Name", p.Spec.Name, "OrganizationRef", p.Spec.OrganizationRef.Name)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(p, constants.ProjectFinalizerName) {
		controllerutil.AddFinalizer(p, constants.ProjectFinalizerName)
		if err := r.Update(ctx, p); err != nil {
			r.Log.Error(err, "Failed to add finalizer to project")
			return ctrl.Result{}, err
		}
	}

	// Initialize OpenAI client. It will initialize the client with the API key from the organization
	openAIClient, err := r.initializeOpenaiClient(ctx, p)
	r.Log.Info("Initialized OpenAI client")
	if err != nil {
		r.Log.Error(err, "Failed to initialize OpenAI client")
		return ctrl.Result{}, err
	}

	// Handle deletion logic
	if !p.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.deleteProject(ctx, p, openAIClient); err != nil {
			r.Log.Error(err, "Failed to delete project")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Create the project
	if p.Status.ProjectID == "" {
		r.Log.Info("Creating project", "Name", p.Spec.Name)
		if err := r.createProject(ctx, p, openAIClient); err != nil {
			r.Log.Error(err, "Failed to create project")
			return ctrl.Result{}, err
		}
		r.Log.Info("Project Created", "Name", p.Spec.Name)
		return ctrl.Result{}, nil
	}

	// Check if update is needed by comparing specs
	needsUpdate := false
	proj, err := openAIClient.Projects.Get(ctx, p.Status.ProjectID)
	if err != nil {
		r.Log.Error(err, "Failed to get project")
		return ctrl.Result{}, err
	}

	if proj.Name != p.Spec.Name {
		r.Log.Info("Project name changed, need to update it", "OldName", proj.Name, "NewName", p.Spec.Name)
		needsUpdate = true
	}

	// Only update if changes are needed
	if needsUpdate {
		r.Log.Info("Updating project", "Name", p.Spec.Name)
		if err := r.updateProject(ctx, p, openAIClient); err != nil {
			r.Log.Error(err, "Failed to update project")
			return ctrl.Result{}, err
		}
		r.Log.Info("Project Updated", "Name", p.Spec.Name)
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) getOrganizationApiKeyNameAndNamespace(ctx context.Context, p *v1beta1.Project) (string, string, string, error) {
	organiztion := &v1beta1.Organization{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: p.Spec.OrganizationRef.Name}, organiztion); err != nil {
		r.Log.Error(err, "Failed to get organization")
		return "", "", "", err
	}
	return organiztion.Spec.SecretRef.Key, organiztion.Spec.SecretRef.Name, organiztion.Spec.SecretRef.Namespace, nil
}

func (r *ProjectReconciler) initializeOpenaiClient(ctx context.Context, p *v1beta1.Project) (*openaisdk.Client, error) {
	apiKey, apiKeyName, apiKeyNamespace, err := r.getOrganizationApiKeyNameAndNamespace(ctx, p)
	if err != nil {
		r.Log.Error(err, "Failed to get organization API key name from organization reference")
		return nil, err
	}
	keySecret := &v1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: apiKeyName, Namespace: apiKeyNamespace}, keySecret); err != nil {
		r.Log.Error(err, "Failed to get secret")
		return nil, err
	}
	apiKeyValue, exists := keySecret.Data[apiKey]
	if !exists {
		r.Log.Error(err, "API key not found in secret")
		return nil, err
	}
	return openaisdk.NewClient(option.WithAPIKey(string(apiKeyValue))), nil
}

func (r *ProjectReconciler) createProject(ctx context.Context, p *v1beta1.Project, openAIClient *openaisdk.Client) error {
	name := p.Spec.Name
	createPro, err := openAIClient.Projects.Create(ctx, openaisdk.ProjectCreateRequest{Name: name})
	if err != nil {
		r.Log.Error(err, "Failed to create project")
		return err
	}
	if err := r.updateProjectStatus(ctx, p, *createPro); err != nil {
		r.Log.Error(err, "Failed to update project status")
		return err
	}
	return nil
}

func (r *ProjectReconciler) updateProject(ctx context.Context, p *v1beta1.Project, openAIClient *openaisdk.Client) error {
	name := p.Spec.Name
	_, err := openAIClient.Projects.Update(ctx, p.Status.ProjectID, openaisdk.ProjectUpdateRequest{Name: name})
	if err != nil {
		r.Log.Error(err, "Failed to update project")
		return err
	}

	return nil
}

func (r *ProjectReconciler) updateProjectStatus(ctx context.Context, p *v1beta1.Project, project openaisdk.Project) error {
	if project.ID != "" {
		p.Status.ProjectID = project.ID
	}
	if project.CreatedAt != 0 && p.Status.CreationTime == nil {
		p.Status.CreationTime = &metav1.Time{Time: time.Unix(project.CreatedAt, 0)}
	}
	if err := r.Client.Status().Update(ctx, p); err != nil {
		r.Recorder.Eventf(p, v1.EventTypeWarning, "StatusUpdateFailed", err.Error())
		return err
	}
	return nil
}

func (r *ProjectReconciler) deleteProject(ctx context.Context, p *v1beta1.Project, openAIClient *openaisdk.Client) error {
	projectID := p.Status.ProjectID
	r.Log.Info("Deleting project", "ProjectID", projectID)
	if _, err := openAIClient.Projects.Archive(ctx, projectID); err != nil {
		r.Log.Error(err, "Failed to archive project")
		return err
	}
	controllerutil.RemoveFinalizer(p, constants.ProjectFinalizerName)
	if err := r.Update(ctx, p); err != nil {
		r.Log.Error(err, "Failed to remove finalizer from project")
		return err
	}
	r.Log.Info("Project deleted", "ProjectID", projectID)
	return nil
}

func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Project{}).
		Complete(r)
}
