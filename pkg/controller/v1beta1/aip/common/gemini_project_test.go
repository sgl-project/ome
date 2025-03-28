package common

import (
	"context"
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
)

// setupTest creates a test environment for gemini project
func setupTestGeminiProject(t *testing.T) *GeminiProject {

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
			ProjectID: "test-project",
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret, testProject).
		WithStatusSubresource(testProject).
		Build()

	// Create project handler
	project := NewGeminiProject(
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testProject,
	)

	return project
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
	project := setupTestGeminiProject(t)

	// Test
	err := project.Create(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check status fields
	assert.Equal(t, "test-project-name", updatedProject.Status.ProjectID)
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
	project := setupTestGeminiProject(t)

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
