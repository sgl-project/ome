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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	testing_pkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

// setupTestWithMockServerForServiceAccount creates a test environment with a mocked OpenAI API server for service account testing
func setupTestWithMockServerForServiceAccount(t *testing.T) (*OpenAIServiceAccount, *httptest.Server, kubernetes.Interface) {
	// Setup mock server
	server := testing_pkg.MockOpenAIServer()

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
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
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
	serviceAccount := NewOpenAIServiceAccount(
		fakeClient,
		fakeClientset,
		logr.Discard(),
		scheme,
		testServiceAccount,
	)

	// Create a custom client that uses the mock server
	customClient := openaisdk.NewClient(
		option.WithAPIKey("test-api-key"),
		option.WithBaseURL(server.URL),
	)

	// Set the client on the service account
	serviceAccount.SetOpenAIClient(customClient)

	return serviceAccount, server, fakeClientset
}

// TestServiceAccount_GetProject tests the GetProject method
func TestServiceAccount_GetProject(t *testing.T) {
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

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	project, err := serviceAccount.GetProject(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "test-project-name", project.Spec.Name)
}

// TestServiceAccount_GetProject_Error tests the error case for GetProject
func TestServiceAccount_GetProject_Error(t *testing.T) {
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

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetProject(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

// TestServiceAccount_GetOrganization tests the GetOrganization method
func TestServiceAccount_GetOrganization(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
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

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	org, err := serviceAccount.GetOrganization(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-org", org.Name)
	assert.Equal(t, v1beta1.VendorOpenAI, *org.Spec.Vendor)
}

// TestServiceAccount_GetOrganization_Error tests the error case for GetOrganization
func TestServiceAccount_GetOrganization_Error(t *testing.T) {
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

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetOrganization(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

// TestServiceAccount_GetOpenAIClient_Error tests the error case for GetOpenAIClient
func TestServiceAccount_GetOpenAIClient_Error(t *testing.T) {
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

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	_, err := serviceAccount.GetOpenAIClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize client")
}

// TestServiceAccount_SetOpenAIClient tests the SetOpenAIClient method
func TestServiceAccount_SetOpenAIClient(t *testing.T) {
	// Setup
	serviceAccount := NewOpenAIServiceAccount(nil, nil, logr.Discard(), nil, &v1beta1.ServiceAccount{})
	mockClient := &openaisdk.Client{}

	// Test
	serviceAccount.SetOpenAIClient(mockClient)
	client, err := serviceAccount.GetOpenAIClient(context.Background())
	require.NoError(t, err)
	assert.Equal(t, mockClient, client)
}

// TestServiceAccount_Create tests the Create method
func TestServiceAccount_Create(t *testing.T) {
	// Setup
	serviceAccount, server, _ := setupTestWithMockServerForServiceAccount(t)
	defer server.Close()

	// Test
	err := serviceAccount.Create(context.Background())
	require.NoError(t, err)

	// Verify the service account status was updated
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check status fields
	assert.NotNil(t, updatedServiceAccount.Status.ServiceAccountID)
	assert.Equal(t, "sa-123", *updatedServiceAccount.Status.ServiceAccountID)
	assert.NotNil(t, updatedServiceAccount.Status.CreationTime)
	assert.NotNil(t, updatedServiceAccount.Status.APIKey)
	assert.Equal(t, "key-123", *updatedServiceAccount.Status.APIKey.APIKeyId)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusCreated), condition.Reason)
	assert.Equal(t, "Service account successfully created", condition.Message)
}

// TestServiceAccount_Create_Error tests the error case for Create
func TestServiceAccount_Create_Error(t *testing.T) {
	// Setup
	serviceAccount, server, _ := setupTestWithMockServerForServiceAccount(t)
	server.Close() // Close the server to force an error

	// Test
	err := serviceAccount.Create(context.Background())
	require.Error(t, err)

	// Verify the service account status was updated with error condition
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestServiceAccount_Delete tests the Delete method
func TestServiceAccount_Delete(t *testing.T) {
	// Setup
	serviceAccount, server, _ := setupTestWithMockServerForServiceAccount(t)
	defer server.Close()

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

// TestServiceAccount_Delete_Error tests the error case for Delete
func TestServiceAccount_Delete_Error(t *testing.T) {
	// Setup
	serviceAccount, server, _ := setupTestWithMockServerForServiceAccount(t)
	server.Close() // Close the server to force an error

	// Set up service account with an ID for deletion
	serviceAccount.Resource.Status.ServiceAccountID = testing_pkg.StringPtr("sa-123")

	// Test
	err := serviceAccount.Delete(context.Background())
	require.Error(t, err)

	// Verify the service account status was updated with error condition
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	// Check conditions
	require.NotEmpty(t, updatedServiceAccount.Status.Conditions)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestServiceAccount_updateServiceAccountCondition tests the updateServiceAccountCondition method
func TestServiceAccount_updateServiceAccountCondition(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testServiceAccount := &v1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sa",
			Generation: 1,
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testServiceAccount).
		WithStatusSubresource(testServiceAccount).
		Build()

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	err := serviceAccount.updateServiceAccountCondition(context.Background(), v1beta1.ServiceAccountStatusCreated)
	require.NoError(t, err)

	// Verify the condition was added
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	require.Len(t, updatedServiceAccount.Status.Conditions, 1)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeReady, condition.Type)
	assert.Equal(t, metav1.ConditionTrue, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusCreated), condition.Reason)
	assert.Equal(t, "Service account successfully created", condition.Message)
}

// TestServiceAccount_updateServiceAccountConditionWithError tests the updateServiceAccountConditionWithError method
func TestServiceAccount_updateServiceAccountConditionWithError(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))

	testServiceAccount := &v1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sa",
			Generation: 1,
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testServiceAccount).
		WithStatusSubresource(testServiceAccount).
		Build()

	serviceAccount := NewOpenAIServiceAccount(fakeClient, nil, logr.Discard(), scheme, testServiceAccount)

	// Test
	originalErr := errors.New("original error")
	err := serviceAccount.updateServiceAccountConditionWithError(context.Background(), v1beta1.ServiceAccountStatusAPIError, originalErr)
	require.Error(t, err)
	assert.Equal(t, originalErr, err)

	// Verify the condition was added
	updatedServiceAccount := &v1beta1.ServiceAccount{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-sa"}, updatedServiceAccount)
	require.NoError(t, err)

	require.Len(t, updatedServiceAccount.Status.Conditions, 1)
	condition := updatedServiceAccount.Status.Conditions[0]
	assert.Equal(t, v1beta1.ConditionTypeError, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, string(v1beta1.ServiceAccountStatusAPIError), condition.Reason)
	assert.Equal(t, "API operation failed", condition.Message)
}

// TestServiceAccount_getStatusMessage tests the getStatusMessage method
func TestServiceAccount_getStatusMessage(t *testing.T) {
	// Setup
	serviceAccount := NewOpenAIServiceAccount(nil, nil, logr.Discard(), nil, &v1beta1.ServiceAccount{})

	// Test
	tests := []struct {
		status   v1beta1.ServiceAccountStatusReason
		expected string
	}{
		{v1beta1.ServiceAccountStatusCreated, "Service account successfully created"},
		{v1beta1.ServiceAccountStatusDeleted, "Service account successfully deleted"},
		{v1beta1.ServiceAccountStatusProjectError, "Failed to get project information"},
		{v1beta1.ServiceAccountStatusInitError, "Failed to initialize service account"},
		{v1beta1.ServiceAccountStatusAPIError, "API operation failed"},
		{v1beta1.ServiceAccountStatusSecretError, "Secret operation failed"},
		{v1beta1.ServiceAccountStatusConfigError, "Configuration error occurred"},
		{"unknown", "Unknown status"},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			message := serviceAccount.getStatusMessage(test.status)
			assert.Equal(t, test.expected, message)
		})
	}
}
