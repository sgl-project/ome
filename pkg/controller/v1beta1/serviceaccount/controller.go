package serviceaccount

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
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

// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=serviceaccounts/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client
	Clientset    kubernetes.Interface
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	OpenAIClient *openaisdk.Client
}

const finalizerName = "serviceaccount.finalizers.openaisdk"

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

	r.Log.Info("Fetched ServiceAccountSpec", "Name", sa.Spec.Name, "ProjectRef", sa.Spec.ProjectRef.Name)

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(sa, finalizerName) {
		controllerutil.AddFinalizer(sa, finalizerName)
		if err := r.Update(ctx, sa); err != nil {
			r.Log.Error(err, "Failed to add finalizer to service account")
			return ctrl.Result{}, err
		}
	}

	// Handle deletion logic
	if !sa.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, sa)
	}

	projectID, err := r.getProjectID(ctx, sa)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Log.Info("Fetched ProjectID", "ProjectID", projectID)

	// Create a new openaiClient and send service account creation request
	createdSA, err := r.OpenAIClient.ServiceAccounts.Create(ctx, projectID, openaisdk.ProjectServiceAccountCreateRequest{Name: sa.Spec.Name})
	if err != nil {
		r.Log.Error(err, "Failed to create service account")
		return ctrl.Result{}, err
	}

	// Check if API key is present and create a secret
	if createdSA.APIKey == nil {
		return ctrl.Result{}, fmt.Errorf("created service account does not have an API key")
	}
	if createdSA.APIKey.Value != "" {
		if err := r.createSecret(ctx, sa, createdSA.APIKey.Value); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		r.Log.Info("API Key creation is not implemented yet", "level", "warning")
	}

	// Update the status with the created service account ID
	sa.Status.ServiceAccountID = createdSA.ProjectServiceAccount.ID
	if err := r.Client.Status().Update(ctx, sa); err != nil {
		r.Recorder.Eventf(sa, v1.EventTypeWarning, "StatusUpdateFailed", err.Error())
		return ctrl.Result{}, err
	}

	r.Log.Info("Service account created", "ServiceAccountID", createdSA.ProjectServiceAccount.ID)
	return ctrl.Result{}, nil
}

// getProjectID fetches the ProjectID resource using ProjectRef
func (r *ServiceAccountReconciler) getProjectID(ctx context.Context, sa *v1beta1.ServiceAccount) (string, error) {
	project := &v1beta1.Project{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: sa.Spec.ProjectRef.Name, Namespace: sa.Spec.ProjectRef.Namespace}, project); err != nil {
		r.Log.Error(err, "unable to fetch Project")
		return "", err
	}
	return project.Status.ProjectID, nil
}

// handleDeletion handles the deletion logic for the ServiceAccount
func (r *ServiceAccountReconciler) handleDeletion(ctx context.Context, sa *v1beta1.ServiceAccount) error {
	r.Log.Info("Deleting service account", "ServiceAccountID", sa.Status.ServiceAccountID)

	// Fetch the ProjectID from ProjectRef
	projectID, err := r.getProjectID(ctx, sa)
	if err != nil {
		return err
	}
	r.Log.Info("Fetched ProjectID", "ProjectID", projectID)

	// Delete the service account
	if _, err := r.OpenAIClient.ServiceAccounts.Delete(ctx, projectID, sa.Status.ServiceAccountID); err != nil {
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

// createSecret creates a Kubernetes Secret to store the API key
func (r *ServiceAccountReconciler) createSecret(ctx context.Context, sa *v1beta1.ServiceAccount, apiKey string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name + "-apikey",
			Namespace: sa.Namespace,
		},
		Data: map[string][]byte{"api-key": []byte(apiKey)},
	}

	if err := controllerutil.SetControllerReference(sa, secret, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to set owner reference on secret")
		return err
	}

	// Create the secret in Kubernetes
	if err := r.Client.Create(ctx, secret); err != nil {
		r.Log.Error(err, "Failed to create secret for API key")
		return err
	}

	sa.Status.APIKeySecretRef = &v1beta1.SecretReference{Name: secret.Name, Namespace: secret.Namespace}
	if err := r.Client.Status().Update(ctx, sa); err != nil {
		r.Log.Error(err, "Failed to update service account status with secret reference")
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ServiceAccount{}).
		Complete(r)
}
