package common

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	testingpkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"k8s.io/apimachinery/pkg/types"

	"cloud.google.com/go/billing/apiv1/billingpb"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// setupTest creates a test environment for gemini project
func setupTestGeminiProject(t *testing.T, projectName string, projectDisplayName string) *GeminiProject {
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
			Name:       projectName,
			UID:        types.UID("e89674fe-af27-4fdd-91ed-34087115d191"),
			Generation: 1,
		},
		Spec: v1beta1.ProjectSpec{
			Name: projectDisplayName,
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
		Status: v1beta1.ProjectStatus{
			ProjectId: "test-project",
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret, testProject).
		WithStatusSubresource(testProject).
		Build()

	// Create fake clientset for GoogleConfig
	fakeClientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "google-config",
			Namespace: "ome",
		},
		Data: map[string]string{
			"google-config": `{"enableBudget": true, "billingAccount": "0182A9-xxxx-cccc", "projectFolder": "folders/542786757384"}`,
		},
	})

	// Create project handler
	project := NewGeminiProject(
		fakeClient,
		fakeClientset,
		logr.Discard(),
		scheme,
		testProject,
	)

	return project
}

func addMockedGcpProjectClientForGetProjectNoGcp(mockGcpClient *testingpkg.MockGcpProjectClient, projectId string, mockClose bool) {
	mockGcpClient.EXPECT().GetProject(
		gomock.Any(),
		&resourcemanagerpb.GetProjectRequest{
			Name: "projects/" + projectId,
		}).Return(nil, fmt.Errorf("not found")).Times(1)

	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpBudgetClient(mockedGcpBudgetClient *testingpkg.MockGcpBudgetClient,
	projectId string, mockClose bool) {

	mockedBudget := &budgetspb.Budget{
		Name:        "Budget name",
		DisplayName: "Budget display name",
	}

	mockedGcpBudgetClient.EXPECT().CreateBudget(
		gomock.Any(), gomock.Any()).Return(mockedBudget, nil).Times(1)

	if mockClose {
		mockedGcpBudgetClient.EXPECT().Close().Return(nil).Times(1)
	}
}
func addMockedGcpBillingClient(mockedGcpBillingClient *testingpkg.MockGcpBillingClient,
	projectId string, mockClose bool) {
	mockedProjectBillingInfo := &billingpb.ProjectBillingInfo{
		ProjectId:          projectId,
		Name:               fmt.Sprintf("projects/%s/billingInfo", projectId),
		BillingAccountName: "billingAccounts/012345-567890-ABCDEF",
		BillingEnabled:     true,
	}

	mockedGcpBillingClient.EXPECT().UpdateProjectBillingInfo(
		gomock.Any(), gomock.Any()).Return(mockedProjectBillingInfo, nil).Times(1)

	if mockClose {
		mockedGcpBillingClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpServiceClient(mockedGcpServiceClient *testingpkg.MockGcpServiceUsageClient) {
	mockedGcpServiceClient.EXPECT().
		Enable(gomock.Any(), "aiplatform.googleapis.com").Return(nil).Times(1)
}

func addMockedGcpProjectClientForCreateProject(mockGcpClient *testingpkg.MockGcpProjectClient,
	projectId string, projectName string, projectDisplayName string, parent string, mockClose bool) {
	mockCreatedProject := &resourcemanagerpb.Project{
		Name:        projectName,
		ProjectId:   projectId,
		DisplayName: projectDisplayName,
	}

	mockGcpClient.EXPECT().CreateProject(
		gomock.Any(),
		&resourcemanagerpb.CreateProjectRequest{
			Project: &resourcemanagerpb.Project{
				ProjectId:   projectId,
				DisplayName: projectDisplayName,
				Parent:      parent,
			},
		}).Return(mockCreatedProject, nil).Times(1)
	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func addMockedGcpProjectClientForGetProject(mockGcpClient *testingpkg.MockGcpProjectClient,
	projectId string, projectName string, projectDisplayName string, mockClose bool) {
	mockedProject := &resourcemanagerpb.Project{
		Name:        projectName,
		ProjectId:   projectId,
		DisplayName: projectDisplayName,
	}

	mockGcpClient.EXPECT().GetProject(
		gomock.Any(),
		&resourcemanagerpb.GetProjectRequest{
			Name: "projects/" + projectId,
		}).Return(mockedProject, nil).Times(1)

	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func AddMockedGcpProjectClientForDeleteProject(mockGcpClient *testingpkg.MockGcpProjectClient,
	projectId string, projectName string, projectDisplayName string, mockClose bool) {
	mockDeletedProject := &resourcemanagerpb.Project{
		Name:        projectName,
		ProjectId:   projectId,
		DisplayName: projectDisplayName,
	}

	mockGcpClient.EXPECT().DeleteProject(
		gomock.Any(),
		&resourcemanagerpb.DeleteProjectRequest{
			Name: "projects/" + projectId,
		}).Return(mockDeletedProject, nil).Times(1)
	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

func AddMockedGcpProjectClientForUpdateProject(mockGcpClient *testingpkg.MockGcpProjectClient,
	projectId string, projectName string, projectDisplayName string, mockClose bool) {
	mockUpdatedProject := &resourcemanagerpb.Project{
		Name:        projectName,
		ProjectId:   projectId,
		DisplayName: projectDisplayName,
	}

	mockGcpClient.EXPECT().UpdateProject(
		gomock.Any(),
		&resourcemanagerpb.UpdateProjectRequest{
			Project: &resourcemanagerpb.Project{
				Name:        "projects/" + projectId,
				DisplayName: projectDisplayName,
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"display_name"},
			},
		}).Return(mockUpdatedProject, nil).Times(1)

	if mockClose {
		mockGcpClient.EXPECT().Close().Return(nil).Times(1)
	}
}

// TestProject_GetOrganizationRef tests the GetOrganizationRef method
func TestGeminiProject_GetOrganizationRef(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	project := NewGeminiProject(nil, nil, logr.Discard(), nil, testProject)

	// Test
	ref := project.GetOrganizationRef(project.Resource)
	assert.Equal(t, "test-org", ref.Name)
}

// TestGeminiProject_Create tests the Create method
func TestGeminiProject_Create(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	project := setupTestGeminiProject(t, "test-project", "test-project-name")

	projectId := "proj-74syguzywy34fgsek7uas9"
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForCreateProject(mockGcpClient, projectId,
		"test-project", "test-project-name", "folders/542786757384", true)
	project.SetGcpProjectClient(mockGcpClient)
	mockedGcpBillingClient := testingpkg.NewMockGcpBillingClient(ctrl)
	mockedGcpBudgetClient := testingpkg.NewMockGcpBudgetClient(ctrl)
	addMockedGcpBillingClient(mockedGcpBillingClient, projectId, true)
	addMockedGcpBudgetClient(mockedGcpBudgetClient, projectId, true)
	project.SetGcpBillingClient(mockedGcpBillingClient)
	project.SetGcpBudgetClient(mockedGcpBudgetClient)
	mockedGcpServiceClient := testingpkg.NewMockGcpServiceUsageClient(ctrl)
	addMockedGcpServiceClient(mockedGcpServiceClient)
	project.SetGcpServiceUsageClient(mockedGcpServiceClient)

	// Test
	err := project.Create(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check status fields
	assert.Equal(t, projectId, updatedProject.Status.ProjectId)
	assert.NotNil(t, updatedProject.Status.CreationTime)
	assert.NotNil(t, updatedProject.Status.LastUpdatedTime)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusCreated), condition.Reason)
	assert.Equal(t, "Project successfully created", condition.Message)
}

// TestGeminiProject_Delete tests the Delete method
func TestGeminiProject_Delete(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "test-project-name")

	// Test
	projectId := "proj-74syguzywy34fgsek7uas9"
	project.Resource.Status.ProjectId = projectId
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProject(mockGcpClient, projectId, "test-project",
		"test-project-name", false)
	AddMockedGcpProjectClientForDeleteProject(mockGcpClient, projectId, "test-project",
		"test-project-name", true)
	project.SetGcpProjectClient(mockGcpClient)

	err := project.Delete(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusArchived), condition.Reason)
	assert.Equal(t, "Project successfully archived", condition.Message)
}

// TestGeminiProject_Delete_NoGcpProject tests the Delete method
func TestGeminiProject_Delete_NoGcpProject(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "test-project-name")

	// Test
	projectId := "proj-74syguzywy34fgsek7uas9"
	project.Resource.Status.ProjectId = projectId
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProjectNoGcp(mockGcpClient, projectId, true)
	project.SetGcpProjectClient(mockGcpClient)

	err := project.Delete(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusArchived), condition.Reason)
	assert.Equal(t, "Project successfully archived", condition.Message)
}

// TestGeminiProject_Update_NoChange ensures no update is made when display name hasn't changed
func TestGeminiProject_Update_NoChange(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "test-project-name")

	projectId := "proj-74syguzywy34fgsek7uas9"
	project.Resource.Spec.Name = "test-project-name"
	project.Resource.Status.ProjectId = projectId
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProject(mockGcpClient, projectId, "test-project",
		"test-project-name", true)
	project.SetGcpProjectClient(mockGcpClient)

	// Test
	err := project.Update(context.Background())

	// Verify
	assert.NoError(t, err)
}

// TestGeminiProject_Update_ChangesDisplayName verifies status is updated when display name changes
func TestGeminiProject_Update_ChangesDisplayName(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "new-display-name")

	projectId := "proj-74syguzywy34fgsek7uas9"
	project.Resource.Status.ProjectId = projectId
	project.Resource.Status.LastUpdatedTime = nil

	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProject(mockGcpClient, projectId, "test-project",
		"test-project-name", true)
	AddMockedGcpProjectClientForUpdateProject(mockGcpClient, projectId, "test-project",
		"new-display-name", true)
	project.SetGcpProjectClient(mockGcpClient)

	// Test
	err := project.Update(context.Background())
	assert.NoError(t, err)

	// Verify status updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	assert.Equal(t, projectId, updatedProject.Status.ProjectId)
	assert.NotNil(t, updatedProject.Status.LastUpdatedTime)
	assert.Equal(t, "new-display-name", project.Resource.Spec.Name)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusUpdated), condition.Reason)
	assert.Equal(t, "Project successfully updated", condition.Message)
}

// TestGeminiProject_GetProject_MissingProjectID returns error if ProjectID is not set
func TestGeminiProject_GetProject_MissingProjectID(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "test-project-name")
	project.Resource.Status.ProjectId = ""

	// Test
	_, err := project.GetProject(context.Background())

	// Verify
	assert.Error(t, err)
}

// TestGeminiProject_GetProject_Success tests GetProject with a valid project ID
func TestGeminiProject_GetProject_Success(t *testing.T) {
	// Setup
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	project := setupTestGeminiProject(t, "test-project", "test-project-name")

	projectId := "proj-74syguzywy34fgsek7uas9"
	project.Resource.Status.ProjectId = projectId
	mockGcpClient := testingpkg.NewMockGcpProjectClient(ctrl)
	addMockedGcpProjectClientForGetProject(mockGcpClient, projectId,
		"test-project", "test-project-name", true)
	project.SetGcpProjectClient(mockGcpClient)
	// Test
	result, err := project.GetProject(context.Background())

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
}
