package common

import (
	"context"
	"fmt"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/golang/mock/gomock"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	testingpkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

// setupTestGeminiServiceAccount creates a test environment for gemini service account testing
func setupTestGeminiServiceAccount(t *testing.T) (*GeminiServiceAccount, kubernetes.Interface) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			SecretRef: &v1beta1.SecretReference{
				Name:      "test-secret",
				Namespace: "default",
				Key:       "api-key",
			},
			Vendor: vendorPtr(v1beta1.VendorGoogle),
		},
	}

	// Create test secret
	testSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-key": []byte("test-api-key"),
		},
	}

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-project",
			Generation: 1,
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
		Status: v1beta1.ProjectStatus{
			ProjectId: "proj_123",
		},
	}

	// Create test service account
	testServiceAccount := &v1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sa",
			UID:        "e89674fe-af27-4fdd-91ed-34087115d191",
			Generation: 1,
		},
		Spec: v1beta1.ServiceAccountSpec{
			Name: testingpkg.StringPtr("test-sa-name"),
			ProjectRef: v1beta1.CrossReference{
				Name: "test-project",
			},
		},
	}

	// Create common secret for AIPlatformConfig
	commonSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "common-secret",
			Namespace: "ome",
		},
		Data: map[string][]byte{},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret, testProject, testServiceAccount, commonSecret).
		WithStatusSubresource(testServiceAccount).
		Build()

	// Create fake clientset for AIPlatformConfig
	fakeClientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aiplatform-config",
			Namespace: "ome",
		},
		Data: map[string]string{
			"aiplatform-config": `{"secretConfig": {"secretName": "common-secret", "namespace": "ome"}}`,
		},
	})

	// Create service account handler
	serviceAccount := NewGeminiServiceAccount(
		fakeClient,
		fakeClientset,
		logr.Discard(),
		scheme,
		testServiceAccount,
	)

	return serviceAccount, fakeClientset
}

func addMockedGcpProjectClientForGetIamPolicy(
	mockGcpClient *testingpkg.MockGcpProjectClient, projectId string, mockClose bool) {
	mockedPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{
			&iampb.Binding{
				Role:    "roles/owner",
				Members: []string{"test@gmail.com"},
			},
		},
	}

	resource := fmt.Sprintf("projects/%s", projectId)
	mockGcpClient.EXPECT().GetIamPolicy(
		gomock.Any(),
		&iampb.GetIamPolicyRequest{
			Resource: resource,
		}).Return(mockedPolicy, nil).Times(1)
	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpProjectClientForSetIamPolicy(
	mockGcpClient *testingpkg.MockGcpProjectClient,
	serviceAccountId string, projectId string, mockClose bool) {
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)
	resource := fmt.Sprintf("projects/%s", projectId)
	mockedPolicy := &iampb.Policy{
		Bindings: []*iampb.Binding{
			{
				Role:    "roles/owner",
				Members: []string{"test@gmail.com"},
			},
			{
				Role:    "roles/aiplatform.user",
				Members: []string{fmt.Sprintf("serviceAccount:%s", saEmail)},
			},
		},
	}

	mockGcpClient.EXPECT().SetIamPolicy(
		gomock.Any(),
		&iampb.SetIamPolicyRequest{
			Resource: resource,
			Policy: &iampb.Policy{
				Bindings: []*iampb.Binding{
					{
						Role:    "roles/owner",
						Members: []string{"test@gmail.com"},
					},
					{
						Role:    "roles/aiplatform.user",
						Members: []string{fmt.Sprintf("serviceAccount:%s", saEmail)},
					},
				},
			}}).Return(mockedPolicy, nil).Times(1)

	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpIamClientForCreateServiceAccount(mockIamClient *testingpkg.MockGcpIamClient,
	projectId string, serviceAccountId string, mockClose bool) {
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)
	mockedServiceAccount := &adminpb.ServiceAccount{
		Name:        fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, saEmail),
		DisplayName: serviceAccountId,
		Email:       saEmail,
		ProjectId:   projectId,
	}

	mockIamClient.EXPECT().CreateServiceAccount(gomock.Any(),
		&adminpb.CreateServiceAccountRequest{
			Name:      fmt.Sprintf("projects/%s", projectId),
			AccountId: serviceAccountId,
			ServiceAccount: &adminpb.ServiceAccount{
				DisplayName: serviceAccountId,
			},
		}).Return(mockedServiceAccount, nil).Times(1)

	if mockClose {
		mockIamClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpIamClientForCreateServiceAccountKey(mockIamClient *testingpkg.MockGcpIamClient,
	projectId string, serviceAccountId string, mockClose bool) {
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)
	mockedServiceAccountKey := &adminpb.ServiceAccountKey{
		Name:           fmt.Sprintf("projects/%s/serviceAccounts/%s/keys/test-key", projectId, serviceAccountId),
		PrivateKeyData: []byte("test-private-key-data"),
	}

	mockIamClient.EXPECT().CreateServiceAccountKey(gomock.Any(),
		&adminpb.CreateServiceAccountKeyRequest{
			Name: fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, saEmail),
		}).Return(mockedServiceAccountKey, nil).Times(1)

	if mockClose {
		mockIamClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpIamClientForDeleteServiceAccount(mockIamClient *testingpkg.MockGcpIamClient,
	projectId string, serviceAccountId string, mockClose bool) {
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccountId, projectId)
	mockIamClient.EXPECT().DeleteServiceAccount(gomock.Any(),
		&adminpb.DeleteServiceAccountRequest{
			Name: fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, saEmail),
		}).Return(nil).Times(1)

	if mockClose {
		mockIamClient.EXPECT().Close().Return(nil).Times(1)
	}
}

// TestGeminiServiceAccount_GetProject tests the GetProject method
func TestGeminiServiceAccount_GetProject(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
		},
	}

	testServiceAccount := &v1beta1.ServiceAccount{
		Spec: v1beta1.ServiceAccountSpec{
			ProjectRef: v1beta1.CrossReference{
				Name: "test-project",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject).
		Build()

	serviceAccount := NewGeminiServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	project, err := serviceAccount.GetProject(context.Background(), serviceAccount.Resource)
	require.NoError(t, err)
	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "test-project-name", project.Spec.Name)
}

// TestGeminiServiceAccount_GetProject_Error tests the error case for GetProject
func TestGeminiServiceAccount_GetProject_Error(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testServiceAccount := &v1beta1.ServiceAccount{
		Spec: v1beta1.ServiceAccountSpec{
			ProjectRef: v1beta1.CrossReference{
				Name: "non-existent-project",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	serviceAccount := NewGeminiServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetProject(context.Background(), serviceAccount.Resource)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

// TestGeminiServiceAccount_GetOrganization tests the GetOrganization method
func TestGeminiServiceAccount_GetOrganization(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorGoogle),
		},
	}

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	testServiceAccount := &v1beta1.ServiceAccount{
		Spec: v1beta1.ServiceAccountSpec{
			ProjectRef: v1beta1.CrossReference{
				Name: "test-project",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testProject).
		Build()

	serviceAccount := NewGeminiServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	org, err := serviceAccount.GetOrganization(context.Background(), serviceAccount.Resource)
	require.NoError(t, err)
	assert.Equal(t, "test-org", org.Name)
	assert.Equal(t, v1beta1.VendorGoogle, *org.Spec.Vendor)
}

// TestGeminiServiceAccount_GetOrganization_Error tests the error case for GetOrganization
func TestGeminiServiceAccount_GetOrganization_Error(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "non-existent-org",
			},
		},
	}

	testServiceAccount := &v1beta1.ServiceAccount{
		Spec: v1beta1.ServiceAccountSpec{
			ProjectRef: v1beta1.CrossReference{
				Name: "test-project",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject).
		Build()

	serviceAccount := NewGeminiServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetOrganization(context.Background(), serviceAccount.Resource)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

// TestGeminiServiceAccount_Create tests the Create method
func TestGeminiServiceAccount_Create(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	serviceAccount, _ := setupTestGeminiServiceAccount(t)

	projectId := "proj_123"
	serviceAccountId := "user-74syguzywy34fgsek7uas9"
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	mockIamClient := testingpkg.NewMockGcpIamClient(ctrl)
	addMockedGcpProjectClientForGetIamPolicy(mockGcpClient, projectId, false)
	addMockedGcpProjectClientForSetIamPolicy(mockGcpClient, serviceAccountId, projectId, true)
	serviceAccount.SetGcpProjectClient(mockGcpClient)

	addMockedGcpIamClientForCreateServiceAccount(mockIamClient, projectId, serviceAccountId, false)
	addMockedGcpIamClientForCreateServiceAccountKey(mockIamClient, projectId, serviceAccountId, true)
	serviceAccount.SetGcpIamClient(mockIamClient)

	// Test
	err := serviceAccount.Create(context.Background())
	require.NoError(t, err)

	// Verify the service account status was updated
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	aiPlatformConfig, err := controllerconfig.NewAIPlatformConfig(serviceAccount.Clientset)
	require.NoError(t, err)
	commonSecret, err := serviceAccount.getCommonSecret(context.Background(), aiPlatformConfig)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, "test-private-key-data", commonSecret.StringData[serviceAccountId])

	// Check status fields
	assert.NotNil(t, updatedServiceAccount.Status.ServiceAccountId)
	assert.Equal(t, "user-74syguzywy34fgsek7uas9", *updatedServiceAccount.Status.ServiceAccountId)
	assert.NotNil(t, updatedServiceAccount.Status.CreationTime)
	assert.Nil(t, updatedServiceAccount.Status.APIKey)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusCreated), condition.Reason)
	assert.Equal(t, "Service account successfully created", condition.Message)
}

// TestGeminiServiceAccount_Delete tests the Delete method
func TestGeminiServiceAccount_Delete(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	serviceAccount, _ := setupTestGeminiServiceAccount(t)

	// Set up service account with an ID for deletion
	serviceAccount.Resource.Status.ServiceAccountId = testingpkg.StringPtr("sa-123")

	projectId := "proj_123"
	serviceAccountId := "sa-123"
	mockProjectClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProject(mockProjectClient, projectId, "test-project",
		"test-project-name", true)
	serviceAccount.SetGcpProjectClient(mockProjectClient)

	mockIamClient := testingpkg.NewMockGcpIamClient(ctrl)
	addMockedGcpIamClientForDeleteServiceAccount(mockIamClient, projectId, serviceAccountId, true)
	serviceAccount.SetGcpIamClient(mockIamClient)

	// Test
	err := serviceAccount.Delete(context.Background())
	require.NoError(t, err)

	// Verify the service account status was updated
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusDeleted), condition.Reason)
	assert.Equal(t, "Service account successfully deleted", condition.Message)
}

// TestGeminiServiceAccount_Delete_NoGcpProject tests the Delete method
func TestGeminiServiceAccount_Delete_NoGcpProject(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	serviceAccount, _ := setupTestGeminiServiceAccount(t)

	// Set up service account with an ID for deletion
	serviceAccount.Resource.Status.ServiceAccountId = testingpkg.StringPtr("sa-123")

	projectId := "proj_123"
	mockProjectClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProjectNoGcp(mockProjectClient, projectId, true)
	serviceAccount.SetGcpProjectClient(mockProjectClient)

	// Test
	err := serviceAccount.Delete(context.Background())
	require.NoError(t, err)

	// Verify the service account status was updated
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusDeleted), condition.Reason)
	assert.Equal(t, "Service account successfully deleted", condition.Message)
}
