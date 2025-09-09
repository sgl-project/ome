package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"cloud.google.com/go/iam/apiv1/iampb"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// GeminiServiceAccount implements ProjectScoped and ResourceOperation
type GeminiServiceAccount struct {
	ResourceBase
	Resource         *v1beta1.ServiceAccount
	iamClient        GcpIamClient
	gcpProjectClient GcpProjectClient
}

// NewGeminiServiceAccount creates a new ServiceAccount resource handler
func NewGeminiServiceAccount(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, sa *v1beta1.ServiceAccount) *GeminiServiceAccount {
	return &GeminiServiceAccount{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: sa,
	}
}

// GetGcpProjectClient initializes the GCP project client with proper error handling
func (sa *GeminiServiceAccount) GetGcpProjectClient(ctx context.Context) (GcpProjectClient, error) {
	if sa.gcpProjectClient != nil {
		// For unit testing
		return sa.gcpProjectClient, nil
	}
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get project for the service account %s: %w", sa.Resource.Name, err)
	}

	org, err := sa.GetOrganizationFromProject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization when creating gcp client: %w", err)
	}

	gcpProjectClient, err := sa.InitializeGcpProjectClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP project client: %w", err)
	}

	return gcpProjectClient, nil
}

func (sa *GeminiServiceAccount) GetIamClient(ctx context.Context, project *v1beta1.Project) (GcpIamClient, error) {
	if sa.iamClient != nil {
		// for unit testing
		return sa.iamClient, nil
	}

	org, err := sa.GetOrganizationFromProject(ctx, project)
	if err != nil {
		return nil, err
	}

	iamClient, err := sa.InitializeGcpIamClient(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to create iam client:%w", err)
	}

	return iamClient, nil
}

// Create implements ResourceOperation
func (sa *GeminiServiceAccount) Create(ctx context.Context) error {
	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	// Set the project as the owner of the service account
	if err := controllerutil.SetControllerReference(project, sa.Resource, sa.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Update the service account resource with the owner reference
	if err := sa.Client.Update(ctx, sa.Resource); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError,
			fmt.Errorf("failed to update service account with owner reference: %w", err))
	}

	// Check if ProjectId is available
	if project.Status.ProjectId == "" {
		sa.Log.Info("ProjectId is not available yet for service account", "name", sa.Resource.Name, "namespace", sa.Resource.Namespace)
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError,
			fmt.Errorf("project ID not available for project %s", project.Name))
	}

	googleConfig, err := controllerconfig.NewGoogleConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusConfigError, err)
	}
	projectId := project.Status.ProjectId
	serviceAccountId := strings.ToLower(GenerateId("user-", sa.Resource.UID))
	if googleConfig.EnableWif {
		// Assign vertex Ai access to the sa
		if err := sa.AssignVertexAiRole(ctx, projectId, ""); err != nil {
			return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError, err)
		}

		// Update status
		creationTime := metav1.NewTime(time.Now())
		serviceAccountId = "user-wif"
		sa.Resource.Status = v1beta1.ServiceAccountStatus{
			ServiceAccountId: &serviceAccountId,
			CreationTime:     &creationTime,
		}
		return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
	}

	createReq := &adminpb.CreateServiceAccountRequest{
		Name:      "projects/" + projectId,
		AccountId: serviceAccountId,
		ServiceAccount: &adminpb.ServiceAccount{
			DisplayName: serviceAccountId,
		},
	}

	iamClient, err := sa.GetIamClient(ctx, project)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}
	defer iamClient.Close()

	svc, err := iamClient.CreateServiceAccount(ctx, createReq)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError,
			fmt.Errorf("failed to create service account:%s", err))
	}

	sa.Log.Info("GCP service account created", "displayName", svc.DisplayName, "serviceAccountId", serviceAccountId)
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)

	retry := 3
	var keyResp *adminpb.ServiceAccountKey
	for retry > 0 {
		keyResp, err = iamClient.CreateServiceAccountKey(ctx, &adminpb.CreateServiceAccountKeyRequest{
			Name: fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, saEmail),
		})

		if err != nil {
			retry--
			fmt.Println(fmt.Errorf("failed to create service account key:%s, retry remaining: %d", err, retry))
			if retry > 0 {
				time.Sleep(5 * time.Second)
			}
		} else {
			break
		}
	}

	// Add service account key to common secret
	aiPlatformConfig, err := controllerconfig.NewAIPlatformConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusConfigError, err)
	}

	commonSecret, err := sa.getCommonSecret(ctx, aiPlatformConfig)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Update common secret
	if commonSecret.StringData == nil {
		commonSecret.StringData = map[string]string{}
	}

	commonSecret.StringData[serviceAccountId] = string(keyResp.PrivateKeyData)

	if err := sa.Client.Update(ctx, commonSecret); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusSecretError, err)
	}

	// Assign vertex Ai access to the sa
	if err := sa.AssignVertexAiRole(ctx, projectId, serviceAccountId); err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError, err)
	}

	// Update status
	creationTime := metav1.NewTime(time.Now())
	sa.Resource.Status = v1beta1.ServiceAccountStatus{
		ServiceAccountId: &serviceAccountId,
		CreationTime:     &creationTime,
	}
	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusCreated)
}

func (sa *GeminiServiceAccount) AssignVertexAiRole(ctx context.Context, projectId string, serviceAccountId string) error {
	googleConfig, err := controllerconfig.NewGoogleConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusConfigError, err)
	}

	gcpProjectClient, err := sa.GetGcpProjectClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCP project Client: %w", err)
	}

	defer gcpProjectClient.Close()

	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)
	resource := fmt.Sprintf("projects/%s", projectId)

	getReq := &iampb.GetIamPolicyRequest{Resource: resource}
	policy, err := gcpProjectClient.GetIamPolicy(ctx, getReq)
	if err != nil {
		return fmt.Errorf("failed to get iam policy:%w", err)
	}

	var bindings []*iampb.Binding
	var binding *iampb.Binding
	if googleConfig.EnableWif {
		binding = &iampb.Binding{
			Role:    "roles/aiplatform.user",
			Members: []string{fmt.Sprintf("principal:%s", googleConfig.OkeServiceAccount)},
		}
	} else {
		binding = &iampb.Binding{
			Role:    "roles/aiplatform.user",
			Members: []string{fmt.Sprintf("serviceAccount:%s", saEmail)},
		}
	}

	if policy != nil {
		bindings = append(policy.Bindings, binding)
	} else {
		bindings = []*iampb.Binding{binding}
	}

	req := &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy: &iampb.Policy{
			Bindings: bindings,
		},
	}

	_, err = gcpProjectClient.SetIamPolicy(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to set iam policy:%w", err)
	}

	return nil
}

// Delete implements ResourceOperation
func (sa *GeminiServiceAccount) Delete(ctx context.Context) error {
	googleConfig, err := controllerconfig.NewGoogleConfig(sa.Clientset)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusConfigError, err)
	}

	if googleConfig.EnableWif {
		return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
	}

	project, err := sa.GetProject(ctx, sa.Resource)
	if err != nil {
		// Check if the error is because the project doesn't exist
		if apierrors.IsNotFound(err) {
			sa.Log.Info("Project not found during service account deletion, likely already deleted",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectRef", sa.Resource.Spec.ProjectRef.Name)

			// Even if the project is gone, we should still clean up the service account key from the common secret
			_ = sa.deleteServiceAccountKey(ctx, sa.Resource, true)

			// Update status and continue with deletion
			return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
		}

		// For other errors, propagate them normally
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusProjectError, err)
	}

	projectId := project.Status.ProjectId
	projectClient, err := sa.GetGcpProjectClient(ctx)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}
	defer projectClient.Close()

	// Query first to see if the project exist in GCP
	name := "projects/" + projectId
	_, err = projectClient.GetProject(ctx, &resourcemanagerpb.GetProjectRequest{Name: name})
	if err != nil {
		sa.Log.Info("failed get project from GCP", "projectId", projectId, "err", err)
		return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
	}

	iamClient, err := sa.GetIamClient(ctx, project)
	if err != nil {
		return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusInitError, err)
	}
	defer iamClient.Close()

	if project.Status.ProjectId != "" && sa.Resource.Status.ServiceAccountId != nil {
		projectId := project.Status.ProjectId
		accountId := sa.Resource.Status.ServiceAccountId
		saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", *accountId, projectId)
		resource := fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, saEmail)
		deleteReq := &adminpb.DeleteServiceAccountRequest{
			Name: resource,
		}

		if err = iamClient.DeleteServiceAccount(ctx, deleteReq); err != nil {
			// Log the error without stack trace for API errors as they might be transient
			sa.Log.Info("Failed to delete service account in API, will retry",
				"name", sa.Resource.Name,
				"namespace", sa.Resource.Namespace,
				"projectID", project.Status.ProjectId,
				"serviceAccountID", accountId,
				"error", err.Error())
			return sa.updateServiceAccountConditionWithError(ctx, sa.Resource, v1beta1.ServiceAccountStatusAPIError, err)
		}
	}

	// Delete service account key from common secret
	if err := sa.deleteServiceAccountKey(ctx, sa.Resource, false); err != nil {
		return err
	}

	return sa.updateServiceAccountCondition(ctx, sa.Resource, v1beta1.ServiceAccountStatusDeleted)
}

// SetGcpProjectClient sets a custom gcp project client for testing purposes
func (sa *GeminiServiceAccount) SetGcpProjectClient(client GcpProjectClient) {
	sa.gcpProjectClient = client
}

// SetGcpIamClient sets a custom gcp iam client for testing purposes
func (sa *GeminiServiceAccount) SetGcpIamClient(client GcpIamClient) {
	sa.iamClient = client
}
