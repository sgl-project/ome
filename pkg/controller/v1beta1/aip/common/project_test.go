package common

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	testingpkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

// setupTestWithMockServer creates a test environment with a mocked OpenAI API server
func setupTestWithMockServer(t *testing.T) (*Project, *httptest.Server) {
	// Setup mock server
	server := testingpkg.MockOpenAIServer()

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
			Vendor: testingpkg.StringPtr("openai"),
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
			ProjectID: "proj-123",
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret, testProject).
		WithStatusSubresource(testProject).
		Build()

	// Create project handler
	project := NewProject(
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testProject,
	)

	// Create a custom client that uses the mock server
	customClient := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Set the client on the project
	project.SetOpenAIClient(customClient)

	return project, server
}

// TestProject_GetOrganizationRef tests the GetOrganizationRef method
func TestProject_GetOrganizationRef(t *testing.T) {
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

	project := NewProject(nil, nil, logr.Discard(), nil, testProject)

	// Test
	ref := project.GetOrganizationRef()
	assert.Equal(t, "test-org", ref.Name)
}

// TestProject_GetOrganization tests the GetOrganization method
func TestProject_GetOrganization(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: testingpkg.StringPtr("openai"),
		},
	}

	testProject := &v1beta1.Project{
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg).
		Build()

	project := NewProject(fakeClient, nil, logr.Discard(), scheme, testProject)

	// Test
	org, err := project.GetOrganization(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-org", org.Name)
	assert.Equal(t, "openai", *org.Spec.Vendor)
}

// TestProject_GetOrganization_Error tests the error case for GetOrganization
func TestProject_GetOrganization_Error(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "non-existent-org",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	project := NewProject(fakeClient, nil, logr.Discard(), scheme, testProject)

	// Test
	_, err := project.GetOrganization(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

// TestProject_GetOpenAIClient_Error tests the error case for GetOpenAIClient
func TestProject_GetOpenAIClient_Error(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "non-existent-org",
			},
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	project := NewProject(fakeClient, nil, logr.Discard(), scheme, testProject)

	// Test
	_, err := project.GetOpenAIClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize client")
}

// TestProject_SetOpenAIClient tests the SetOpenAIClient method
func TestProject_SetOpenAIClient(t *testing.T) {
	// Setup
	project := NewProject(nil, nil, logr.Discard(), nil, &v1beta1.Project{})
	mockClient := &openaisdk.Client{}

	// Test
	project.SetOpenAIClient(mockClient)
	client, err := project.GetOpenAIClient(context.Background())
	require.NoError(t, err)
	assert.Equal(t, mockClient, client)
}

// TestProject_Create tests the Create method
func TestProject_Create(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	defer server.Close()

	// Test
	err := project.Create(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check status fields
	assert.Equal(t, "proj-123", updatedProject.Status.ProjectID)
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

// TestProject_Create_Error tests the error case for Create
func TestProject_Create_Error(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	server.Close() // Close the server to force an error

	// Test
	err := project.Create(context.Background())
	require.Error(t, err)

	// Verify the project status was updated with error condition
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestProject_Update tests the Update method
func TestProject_Update(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	defer server.Close()

	// Test
	err := project.Update(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check status fields
	assert.NotNil(t, updatedProject.Status.LastUpdatedTime)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusUpdated), condition.Reason)
	assert.Equal(t, "Project successfully updated", condition.Message)
}

// TestProject_Update_Error tests the error case for Update
func TestProject_Update_Error(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	server.Close() // Close the server to force an error

	// Test
	err := project.Update(context.Background())
	require.Error(t, err)

	// Verify the project status was updated with error condition
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestProject_GetProject tests the GetProject method
func TestProject_GetProject(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	defer server.Close()

	// Test
	result, err := project.GetProject(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "proj-123", result.ID)
	assert.Equal(t, "test-project-name", result.Name)
}

// TestProject_GetProject_Error tests the error case for GetProject
func TestProject_GetProject_Error(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	server.Close() // Close the server to force an error

	// Test
	_, err := project.GetProject(context.Background())
	require.Error(t, err)
}

// TestProject_Delete tests the Delete method
func TestProject_Delete(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	defer server.Close()

	// Test
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

// TestProject_Delete_Error tests the error case for Delete
func TestProject_Delete_Error(t *testing.T) {
	// Setup
	project, server := setupTestWithMockServer(t)
	server.Close() // Close the server to force an error

	// Test
	err := project.Delete(context.Background())
	require.Error(t, err)

	// Verify the project status was updated with error condition
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedProject.Status.Conditions)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestProject_updateCondition tests the updateCondition method
func TestProject_updateCondition(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-project",
			Generation: 1,
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject).
		WithStatusSubresource(testProject).
		Build()

	project := NewProject(fakeClient, nil, logr.Discard(), scheme, testProject)

	// Test
	err := project.updateCondition(context.Background(), v1beta1.ProjectStatusCreated)
	require.NoError(t, err)

	// Verify the condition was added
	updatedProject := &v1beta1.Project{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	require.Len(t, updatedProject.Status.Conditions, 1)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusCreated), condition.Reason)
	assert.Equal(t, "Project successfully created", condition.Message)
}

// TestProject_updateConditionWithError tests the updateConditionWithError method
func TestProject_updateConditionWithError(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-project",
			Generation: 1,
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject).
		WithStatusSubresource(testProject).
		Build()

	project := NewProject(fakeClient, nil, logr.Discard(), scheme, testProject)

	// Test
	originalErr := errors.New("original error")
	err := project.updateConditionWithError(context.Background(), v1beta1.ProjectStatusAPIError, originalErr)
	require.Error(t, err)
	assert.Equal(t, originalErr, err)

	// Verify the condition was added
	updatedProject := &v1beta1.Project{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	require.Len(t, updatedProject.Status.Conditions, 1)
	condition := updatedProject.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ProjectStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestProject_getStatusMessage tests the getStatusMessage method
func TestProject_getStatusMessage(t *testing.T) {
	// Setup
	project := NewProject(nil, nil, logr.Discard(), nil, &v1beta1.Project{})

	// Test
	tests := []struct {
		status   v1beta1.ProjectStatusReason
		expected string
	}{
		{v1beta1.ProjectStatusCreated, "Project successfully created"},
		{v1beta1.ProjectStatusUpdated, "Project successfully updated"},
		{v1beta1.ProjectStatusArchived, "Project successfully archived"},
		{v1beta1.ProjectStatusInitError, "Failed to initialize project"},
		{v1beta1.ProjectStatusAPIError, "API operation failed"},
		{v1beta1.ProjectStatusOrgError, "Organization operation failed"},
		{"unknown", "Unknown status"},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			message := project.getStatusMessage(test.status)
			assert.Equal(t, test.expected, message)
		})
	}
}
