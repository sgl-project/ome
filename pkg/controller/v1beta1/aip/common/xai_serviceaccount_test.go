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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	testing_pkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

// setupTestXAIServiceAccount creates a test environment for xai service account testing
func setupTestXAIServiceAccount(t *testing.T) (*XAIServiceAccount, kubernetes.Interface) {
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

	// Create test service account
	testServiceAccount := &v1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sa",
			UID:        "e89674fe-af27-4fdd-91ed-34087115d191",
			Generation: 1,
		},
		Spec: v1beta1.ServiceAccountSpec{
			Name: testing_pkg.StringPtr("test-sa-name"),
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
	serviceAccount := NewXAIServiceAccount(
		fakeClient,
		fakeClientset,
		logr.Discard(),
		scheme,
		testServiceAccount,
	)

	return serviceAccount, fakeClientset
}

// TestXAIServiceAccount_GetProject tests the GetProject method
func TestXAIServiceAccount_GetProject(t *testing.T) {
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

	serviceAccount := NewXAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	project, err := serviceAccount.GetProject(context.Background(), serviceAccount.Resource)
	require.NoError(t, err)
	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "test-project-name", project.Spec.Name)
}

// TestXAIServiceAccount_GetProject_Error tests the error case for GetProject
func TestXAIServiceAccount_GetProject_Error(t *testing.T) {
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

	serviceAccount := NewXAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetProject(context.Background(), serviceAccount.Resource)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

// TestXAIServiceAccount_GetOrganization tests the GetOrganization method
func TestXAIServiceAccount_GetOrganization(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorXAI),
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

	serviceAccount := NewXAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	org, err := serviceAccount.GetOrganization(context.Background(), serviceAccount.Resource)
	require.NoError(t, err)
	assert.Equal(t, "test-org", org.Name)
	assert.Equal(t, v1beta1.VendorXAI, *org.Spec.Vendor)
}

// TestXAIServiceAccount_GetOrganization_Error tests the error case for GetOrganization
func TestXAIServiceAccount_GetOrganization_Error(t *testing.T) {
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

	serviceAccount := NewXAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetOrganization(context.Background(), serviceAccount.Resource)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

// TestXAIServiceAccount_Create tests the Create method
func TestXAIServiceAccount_Create(t *testing.T) {
	// Setup
	serviceAccount, _ := setupTestXAIServiceAccount(t)

	// Test
	err := serviceAccount.Create(context.Background())
	require.NoError(t, err)

	// Verify the service account status was updated
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check status fields
	assert.NotNil(t, updatedServiceAccount.Status.ServiceAccountID)
	assert.Equal(t, "user_ZTg5Njc0ZmUtYWYyNy00ZmRkLTkxZWQtMzQwODcxMTVkMTkx", *updatedServiceAccount.Status.ServiceAccountID)
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

// TestXAIServiceAccount_Delete tests the Delete method
func TestXAIServiceAccount_Delete(t *testing.T) {
	// Setup
	serviceAccount, _ := setupTestXAIServiceAccount(t)

	// Set up service account with an ID for deletion
	serviceAccount.Resource.Status.ServiceAccountID = testing_pkg.StringPtr("sa-123")

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
