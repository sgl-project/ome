package project

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	apierr "k8s.io/apimachinery/pkg/api/errors"

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
// +kubebuilder:rbac:groups=ome.io,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list

// ProjectService handles OpenAI project operations
type ProjectService struct {
	client *openaisdk.Client
	log    logr.Logger
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
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, request.NamespacedName, project); err != nil {
		return r.handleGetError(err, log)
	}

	log.Info("Reconciling project", "name", project.Spec.Name, "organization", project.Spec.OrganizationRef.Name)

	if err := r.ensureFinalizer(ctx, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer: %w", err)
	}

	// Initialize project service
	projectService, err := r.initializeProjectService(ctx, project)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to initialize project service: %w", err)
	}

	// Handle deletion
	if !project.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, project, projectService)
	}

	// Handle creation or update
	return r.handleReconciliation(ctx, project, projectService)
}

func (r *ProjectReconciler) handleGetError(err error, log logr.Logger) (reconcile.Result, error) {
	if apierr.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	log.Error(err, "unable to fetch Project")
	return ctrl.Result{}, client.IgnoreNotFound(err)
}

func (r *ProjectReconciler) ensureFinalizer(ctx context.Context, project *v1beta1.Project) error {
	if !controllerutil.ContainsFinalizer(project, constants.ProjectFinalizerName) {
		controllerutil.AddFinalizer(project, constants.ProjectFinalizerName)
		return r.Update(ctx, project)
	}
	return nil
}

func (r *ProjectReconciler) initializeProjectService(ctx context.Context, project *v1beta1.Project) (*ProjectService, error) {
	openAIClient, err := r.initializeOpenaiClient(ctx, project)
	if err != nil {
		return nil, err
	}
	return &ProjectService{
		client: openAIClient,
		log:    r.Log.WithName("project-service"),
	}, nil
}

func (r *ProjectReconciler) handleDeletion(ctx context.Context, project *v1beta1.Project, service *ProjectService) (reconcile.Result, error) {
	// First, ensure all service accounts are deleted
	if err := r.ensureServiceAccountsDeleted(ctx, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete service accounts: %w", err)
	}

	if err := r.deleteProject(ctx, project, service.client); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete project: %w", err)
	}
	return reconcile.Result{}, nil
}

func (r *ProjectReconciler) ensureServiceAccountsDeleted(ctx context.Context, project *v1beta1.Project) error {
	// List all service accounts owned by this project
	serviceAccounts := &v1beta1.ServiceAccountList{}
	if err := r.Client.List(ctx, serviceAccounts, client.MatchingFields{
		".metadata.controller": project.Name,
	}); err != nil {
		r.Log.Error(err, "Failed to list service accounts")
		return err
	}

	// Delete each service account
	for _, sa := range serviceAccounts.Items {
		if err := r.Client.Delete(ctx, &sa); err != nil {
			if !errors.IsNotFound(err) {
				r.Log.Error(err, "Failed to delete service account", "ServiceAccount", sa.Name)
				return err
			}
		}
	}

	// Wait for service accounts to be deleted
	if len(serviceAccounts.Items) > 0 {
		// Requeue to wait for service accounts to be deleted
		return fmt.Errorf("waiting for service accounts to be deleted")
	}

	return nil
}

func (r *ProjectReconciler) handleReconciliation(ctx context.Context, project *v1beta1.Project, service *ProjectService) (reconcile.Result, error) {
	// Create project if it doesn't exist
	if project.Status.ProjectID == "" {
		if err := r.createProject(ctx, project, service.client); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create project: %w", err)
		}
		return reconcile.Result{}, nil
	}

	// Check if update is needed
	needsUpdate, err := r.checkIfUpdateNeeded(ctx, project, service.client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if update needed: %w", err)
	}

	if needsUpdate {
		if err := r.updateProject(ctx, project, service.client); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update project: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *ProjectReconciler) checkIfUpdateNeeded(ctx context.Context, project *v1beta1.Project, client *openaisdk.Client) (bool, error) {
	proj, err := client.Projects.Get(ctx, project.Status.ProjectID)
	if err != nil {
		return false, err
	}
	return proj.Name != project.Spec.Name, nil
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

func (r *ProjectReconciler) updateProjectStatus(ctx context.Context, project *v1beta1.Project, openAIProject openaisdk.Project) error {
	original := project.DeepCopy()

	if openAIProject.ID != "" {
		project.Status.ProjectID = openAIProject.ID
	}
	if openAIProject.CreatedAt != 0 && project.Status.CreationTime == nil {
		project.Status.CreationTime = &metav1.Time{Time: time.Unix(openAIProject.CreatedAt, 0)}
	}

	if err := r.Client.Status().Patch(ctx, project, client.MergeFrom(original)); err != nil {
		r.Recorder.Eventf(project, v1.EventTypeWarning, "StatusUpdateFailed", "Failed to update status: %v", err)
		return fmt.Errorf("failed to update status: %w", err)
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

// SetupWithManager sets up the controller with the Manager.
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
