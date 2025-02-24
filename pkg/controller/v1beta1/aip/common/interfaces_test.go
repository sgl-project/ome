package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
)

// mockResource implements OrganizationScoped, ProjectScoped, and ResourceOperation
type mockResource struct {
	ResourceBase
	orgRef  *v1beta1.CrossReference
	projRef *v1beta1.CrossReference
}

func (m *mockResource) GetOrganizationRef() *v1beta1.CrossReference {
	return m.orgRef
}

func (m *mockResource) GetOrganization(ctx context.Context) (*v1beta1.Organization, error) {
	org := &v1beta1.Organization{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: m.orgRef.Name}, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (m *mockResource) GetProjectRef() *v1beta1.CrossReference {
	return m.projRef
}

func (m *mockResource) GetProject(ctx context.Context) (*v1beta1.Project, error) {
	proj := &v1beta1.Project{}
	if err := m.Client.Get(ctx, client.ObjectKey{Name: m.projRef.Name}, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (m *mockResource) Create(ctx context.Context) error {
	return nil
}

func (m *mockResource) Update(ctx context.Context) error {
	return nil
}

func (m *mockResource) Delete(ctx context.Context) error {
	return nil
}

func TestOrganizationScoped(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			SecretRef: v1beta1.SecretReference{
				Name:      "test-secret",
				Namespace: "default",
				Key:       "api-key",
			},
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

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret).
		Build()

	// Create mock resource
	mock := &mockResource{
		ResourceBase: ResourceBase{
			Client:    fakeClient,
			Clientset: kfake.NewSimpleClientset(),
		},
		orgRef: &v1beta1.CrossReference{
			Name: "test-org",
		},
	}

	t.Run("GetOrganizationRef", func(t *testing.T) {
		ref := mock.GetOrganizationRef()
		assert.Equal(t, "test-org", ref.Name)
	})

	t.Run("GetOrganization", func(t *testing.T) {
		org, err := mock.GetOrganization(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "test-org", org.Name)
		assert.Equal(t, "test-secret", org.Spec.SecretRef.Name)
	})
}

func TestProjectScoped(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	// Create test project
	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project",
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testProject).
		Build()

	// Create mock resource
	mock := &mockResource{
		ResourceBase: ResourceBase{
			Client:    fakeClient,
			Clientset: kfake.NewSimpleClientset(),
		},
		projRef: &v1beta1.CrossReference{
			Name: "test-project",
		},
	}

	t.Run("GetProjectRef", func(t *testing.T) {
		ref := mock.GetProjectRef()
		assert.Equal(t, "test-project", ref.Name)
	})

	t.Run("GetProject", func(t *testing.T) {
		proj, err := mock.GetProject(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "test-project", proj.Name)
		assert.Equal(t, "test-org", proj.Spec.OrganizationRef.Name)
	})
}

func TestResourceBase_InitializeClient(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	// Create test organization
	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			SecretRef: v1beta1.SecretReference{
				Name:      "test-secret",
				Namespace: "default",
				Key:       "api-key",
			},
			Vendor: stringPtr("openai"),
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

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg, testSecret).
		Build()

	// Create resource base
	rb := ResourceBase{
		Client:    fakeClient,
		Clientset: kfake.NewSimpleClientset(),
	}

	t.Run("InitializeClient", func(t *testing.T) {
		client, err := rb.InitializeClient(context.Background(), testOrg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeClient_NoSecret", func(t *testing.T) {
		// Create org with non-existent secret
		orgNoSecret := testOrg.DeepCopy()
		orgNoSecret.Spec.SecretRef.Name = "non-existent"

		_, err := rb.InitializeClient(context.Background(), orgNoSecret)
		assert.Error(t, err)
	})

	t.Run("InitializeClient_NoVendor", func(t *testing.T) {
		// Create org with no vendor
		orgNoVendor := testOrg.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeClient(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeClient_UnsupportedVendor", func(t *testing.T) {
		// Create org with unsupported vendor
		orgUnsupported := testOrg.DeepCopy()
		orgUnsupported.Spec.Vendor = stringPtr("unsupported")

		_, err := rb.InitializeClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}

func stringPtr(s string) *string {
	return &s
}
