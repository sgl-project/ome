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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// setupTestXAIProject creates a test environment for xai project
func setupTestXAIProject(t *testing.T) *XAIProject {

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
			Vendor: vendorPtr(v1beta1.VendorXAI),
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
			UID:        types.UID("e89674fe-af27-4fdd-91ed-34087115d191"),
			Generation: 1,
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
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

	// Create project handler
	project := NewXAIProject(
		fakeClient,
		nil,
		logr.Discard(),
		scheme,
		testProject,
	)

	return project
}

// TestProject_GetOrganizationRef tests the GetOrganizationRef method
func TestXAIProject_GetOrganizationRef(t *testing.T) {
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

	project := NewXAIProject(nil, nil, logr.Discard(), nil, testProject)

	// Test
	ref := project.GetOrganizationRef(project.Resource)
	assert.Equal(t, "test-org", ref.Name)
}

// TestXAIProject_Create tests the Create method
func TestXAIProject_Create(t *testing.T) {
	// Setup
	project := setupTestXAIProject(t)

	// Test
	err := project.Create(context.Background())
	require.NoError(t, err)

	// Verify the project status was updated
	updatedProject := &v1beta1.Project{}
	err = project.Client.Get(context.Background(), client.ObjectKey{Name: "test-project"}, updatedProject)
	require.NoError(t, err)

	// Check status fields
	assert.Equal(t, "proj_74syguzYwy34fGSeK7uaS9", updatedProject.Status.ProjectId)
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

// TestXAIProject_Delete tests the Delete method
func TestXAIProject_Delete(t *testing.T) {
	// Setup
	project := setupTestXAIProject(t)

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
