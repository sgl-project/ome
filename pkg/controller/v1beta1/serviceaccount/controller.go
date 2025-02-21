package serviceaccount

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client
	Clientset           kubernetes.Interface
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	Recorder            record.EventRecorder
	OpenAIClientFactory func(apiKey string, baseURL string) *openaisdk.Client
}

const finalizerName = "serviceaccount.ome.io.finalizers"

// Reconcile reads the state of the cluster for a ServiceAccount object and makes changes based on the state read
// and what is in the ServiceAccount.Spec. It handles the creation and deletion of service accounts and manages
// the associated API key by creating a Kubernetes Secret.
func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ServiceAccount instance
	sa := &v1beta1.ServiceAccount{}
	if err := r.Client.Get(ctx, request.NamespacedName, sa); err != nil {
		r.Log.Error(err, "unable to fetch ServiceAccount")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling ServiceAccount", "Name", sa.Spec.Name, "ProjectRef", sa.Spec.ProjectRef.Name)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(sa, finalizerName) {
		controllerutil.AddFinalizer(sa, finalizerName)
		if err := r.Update(ctx, sa); err != nil {
			r.Log.Error(err, "Failed to add finalizer to service account")
			return ctrl.Result{}, err
		}
	}

	// First check if project exists
	projectID, err := r.getProjectID(ctx, sa)
	if err != nil {
		r.Log.Error(err, "Failed to get project ID")
		return ctrl.Result{}, err
	}

	r.Log.Info("Fetched ProjectID", "ProjectID", projectID)

	// Initialize OpenAI client
	openAIClient, err := r.initializeOpenaiClient(ctx, sa)
	if err != nil {
		r.Log.Error(err, "Failed to initialize OpenAI client")
		return ctrl.Result{}, err
	}

	// Handle deletion logic
	if !sa.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, sa, openAIClient)
	}

	// Create service account if it doesn't exist
	checkResult, _, err := r.checkServiceAcctExist(ctx, sa, openAIClient, projectID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if service account exists: %w", err)
	}
	switch checkResult {
	case constants.CheckResultCreate:
		if err := r.createServiceAccount(ctx, sa, openAIClient, projectID); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create service account: %w", err)
		}
	case constants.CheckResultUpdate:
		if err := r.updateServiceAccount(ctx, sa, openAIClient, projectID); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update service account: %w", err)
		}
	default:
		r.Log.Info("Service account already exists")
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) checkServiceAcctExist(ctx context.Context, sa *v1beta1.ServiceAccount, client *openaisdk.Client, projectID string) (constants.CheckResultType, *v1beta1.ServiceAccount, error) {
	if sa.Status.ServiceAccountID == nil {
		return constants.CheckResultCreate, sa, nil
	}

	existingSA, err := client.ServiceAccounts.Get(ctx, projectID, *sa.Status.ServiceAccountID)
	if err != nil {
		return constants.CheckResultCreate, sa, err
	}
	if existingSA.Name != *sa.Spec.Name {
		return constants.CheckResultUpdate, sa, nil
	}

	return constants.CheckResultExisted, nil, nil
}

// getProjectID fetches the ProjectID resource using ProjectRef
func (r *ServiceAccountReconciler) getProjectID(ctx context.Context, sa *v1beta1.ServiceAccount) (string, error) {
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name, Namespace: sa.Spec.ProjectRef.Namespace}, project); err != nil {
		r.Log.Error(err, "unable to fetch Project")
		return "", err
	}
	if project.Status.ProjectID == "" {
		return "", fmt.Errorf("project ID is empty")
	}
	return project.Status.ProjectID, nil
}

// handleDeletion handles the deletion logic for the ServiceAccount
func (r *ServiceAccountReconciler) handleDeletion(ctx context.Context, sa *v1beta1.ServiceAccount, openAIClient *openaisdk.Client) error {
	r.Log.Info("Deleting service account", "ServiceAccountID", sa.Status.ServiceAccountID)

	// Fetch the ProjectID from ProjectRef
	projectID, err := r.getProjectID(ctx, sa)
	if err != nil {
		return err
	}
	r.Log.Info("Fetched ProjectID", "ProjectID", projectID)

	// Delete the service account
	if _, err := openAIClient.ServiceAccounts.Delete(ctx, projectID, *sa.Status.ServiceAccountID); err != nil {
		r.Log.Error(err, "Failed to delete service account")
		return err
	}
	// Remove finalizer if needed
	controllerutil.RemoveFinalizer(sa, finalizerName)
	if err := r.Update(ctx, sa); err != nil {
		r.Log.Error(err, "Failed to remove finalizer from service account")
		return err
	}
	return nil
}

// initializeOpenaiClient initializes the OpenAI client by getting the API key from the organization through project reference
func (r *ServiceAccountReconciler) initializeOpenaiClient(ctx context.Context, sa *v1beta1.ServiceAccount) (*openaisdk.Client, error) {
	// Get project first
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name, Namespace: sa.Spec.ProjectRef.Namespace}, project); err != nil {
		r.Log.Error(err, "Failed to get project")
		return nil, err
	}

	// Get organization
	organization := &v1beta1.Organization{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: project.Spec.OrganizationRef.Name}, organization); err != nil {
		r.Log.Error(err, "Failed to get organization")
		return nil, err
	}

	// Get API key from secret
	keySecret := &v1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      organization.Spec.SecretRef.Name,
		Namespace: organization.Spec.SecretRef.Namespace,
	}, keySecret); err != nil {
		r.Log.Error(err, "Failed to get secret")
		return nil, err
	}

	apiKeyValue, exists := keySecret.Data[organization.Spec.SecretRef.Key]
	if !exists {
		return nil, fmt.Errorf("API key not found in secret")
	}

	if r.OpenAIClientFactory != nil {
		return r.OpenAIClientFactory(string(apiKeyValue), ""), nil
	}

	return openaisdk.NewClient(option.WithAPIKey(string(apiKeyValue))), nil
}

// checkIfUpdateNeeded checks if the service account needs to be updated
func (r *ServiceAccountReconciler) checkIfUpdateNeeded(ctx context.Context, sa *v1beta1.ServiceAccount, client *openaisdk.Client, projectID string) (bool, error) {
	existingSA, err := client.ServiceAccounts.Get(ctx, projectID, *sa.Status.ServiceAccountID)
	if err != nil {
		return false, err
	}
	return existingSA.Name != *sa.Spec.Name, nil
}

// createServiceAccount creates a new service account and updates the status
func (r *ServiceAccountReconciler) createServiceAccount(ctx context.Context, sa *v1beta1.ServiceAccount, client *openaisdk.Client, projectID string) error {
	// Get the project to set owner reference
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: sa.Spec.ProjectRef.Name, Namespace: sa.Spec.ProjectRef.Namespace}, project); err != nil {
		r.Log.Error(err, "Failed to get project for owner reference")
		return err
	}

	// Set project as owner
	if err := controllerutil.SetControllerReference(project, sa, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set owner reference to project")
		return err
	}
	if err := r.Client.Update(ctx, sa); err != nil {
		r.Log.Error(err, "Failed to update service account with owner reference")
		return err
	}

	createdSA, err := client.ServiceAccounts.Create(ctx, projectID, openaisdk.ProjectServiceAccountCreateRequest{Name: *sa.Spec.Name})
	if err != nil {
		r.Log.Error(err, "Failed to create service account")
		return err
	}

	// Update status with service account ID
	sa.Status.ServiceAccountID = &createdSA.ProjectServiceAccount.ID

	// Create secret if API key is present
	if createdSA.APIKey != nil && createdSA.APIKey.Value != "" {
		if err := r.createOrUpdateSecret(ctx, sa, createdSA.APIKey); err != nil {
			return err
		}
	}

	if err := r.Client.Status().Update(ctx, sa); err != nil {
		r.Recorder.Eventf(sa, v1.EventTypeWarning, "StatusUpdateFailed", err.Error())
		return err
	}

	return nil
}

// updateServiceAccount updates an existing service account
func (r *ServiceAccountReconciler) updateServiceAccount(ctx context.Context, sa *v1beta1.ServiceAccount, client *openaisdk.Client, projectID string) error {
	// Note: Currently OpenAI API doesn't support updating service accounts
	// This is a placeholder for future implementation
	return nil
}

// reconcileSecret ensures the K8s secret is in the desired state
func (r *ServiceAccountReconciler) reconcileSecret(ctx context.Context, sa *v1beta1.ServiceAccount) error {
	if sa.Status.APIKey.APIKeySecretRef == nil {
		return nil // No secret reference, nothing to reconcile
	}

	// Check if secret exists
	existingSecret := &v1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      sa.Status.APIKey.APIKeySecretRef.Name,
		Namespace: sa.Status.APIKey.APIKeySecretRef.Namespace,
	}, existingSecret)

	if err != nil {
		if errors.IsNotFound(err) {
			// Secret doesn't exist but should - this is an error state
			// We can't recreate it because we don't have the API key value anymore
			r.Log.Error(err, "API key secret is missing")
			return fmt.Errorf("API key secret is missing: %w", err)
		}
		return err
	}

	// Secret exists - verify owner reference
	if err := r.ensureSecretOwnerRef(ctx, sa, existingSecret); err != nil {
		return err
	}

	return nil
}

// createOrUpdateSecret creates or updates the K8s secret for the API key
func (r *ServiceAccountReconciler) createOrUpdateSecret(ctx context.Context, sa *v1beta1.ServiceAccount, apiKey *openaisdk.ProjectServiceAccountAPIKey) error {
	secretName := sa.Name + "-api-key"
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: sa.Namespace,
		},
		Data: map[string][]byte{
			*sa.Status.ServiceAccountID: []byte(apiKey.Value),
		},
	}

	if err := controllerutil.SetControllerReference(sa, secret, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set owner reference on secret")
		return err
	}

	// Try to create the secret
	err := r.Client.Create(ctx, secret)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing secret
			existing := &v1.Secret{}
			if err := r.Client.Get(ctx, client.ObjectKey{Name: secretName, Namespace: sa.Namespace}, existing); err != nil {
				return err
			}
			existing.Data = secret.Data
			if err := r.Client.Update(ctx, existing); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Initialize APIKey if it's nil
	if sa.Status.APIKey == nil {
		sa.Status.APIKey = &v1beta1.APIKeySpec{
			Name: sa.Spec.Name, // Use the service account name for the API key name
		}
	}

	// Update status with secret reference
	sa.Status.APIKey.APIKeySecretRef = &v1beta1.SecretReference{
		Name:      secretName,
		Key:       "api-key",
		Namespace: sa.Namespace,
	}
	sa.Status.APIKey.APIKeyId = &apiKey.ID
	return r.Client.Status().Update(ctx, sa)
}

// ensureSecretOwnerRef ensures the secret has the correct owner reference
func (r *ServiceAccountReconciler) ensureSecretOwnerRef(ctx context.Context, sa *v1beta1.ServiceAccount, secret *v1.Secret) error {
	if !metav1.IsControlledBy(secret, sa) {
		if err := controllerutil.SetControllerReference(sa, secret, r.Scheme); err != nil {
			return err
		}
		return r.Client.Update(ctx, secret)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ServiceAccount{}).
		Complete(r)
}
