package common

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
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
			SecretRef: &v1beta1.SecretReference{
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
			SecretRef: &v1beta1.SecretReference{
				Name:      "openai-admin-key",
				Namespace: "default",
				Key:       "api-key",
			},
			Vendor: vendorPtr(v1beta1.VendorOpenAI),
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg).
		Build()

	// Create resource base
	rb := ResourceBase{
		Client:    fakeClient,
		Clientset: kfake.NewSimpleClientset(),
	}

	err := os.MkdirAll("/tmp/secrets-store", 0755)
	require.NoError(t, err)

	err = os.WriteFile("/tmp/secrets-store/openai-admin-key", []byte("test-api-key"), 0644)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Remove("/tmp/secrets-store/openai-admin-key")
		os.Remove("/tmp/secrets-store")
	})

	t.Run("InitializeClient", func(t *testing.T) {
		client, err := rb.InitializeClient(context.Background(), testOrg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeClient_CustomSecretRef", func(t *testing.T) {
		proxyOrg := testOrg.DeepCopy()
		proxyOrg.Spec.SecretRef.Name = "openai-proxy-admin-key"

		err := os.WriteFile("/tmp/secrets-store/openai-proxy-admin-key", []byte("proxy-api-key"), 0644)
		require.NoError(t, err)
		defer os.Remove("/tmp/secrets-store/openai-proxy-admin-key")

		adminSecret, err := rb.GetOpenAiAdminSecret(proxyOrg)
		require.NoError(t, err)
		assert.Equal(t, "proxy-api-key", string(adminSecret))

		client, err := rb.InitializeClient(context.Background(), proxyOrg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeClient_NoSecretFile", func(t *testing.T) {
		err := os.Remove("/tmp/secrets-store/openai-admin-key")
		require.NoError(t, err)

		_, err = rb.InitializeClient(context.Background(), testOrg)
		assert.Error(t, err)

		err = os.WriteFile("/tmp/secrets-store/openai-admin-key", []byte("test-api-key"), 0644)
		require.NoError(t, err)
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
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}

func TestResourceBase_InitializeXaiClient(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-xai-org",
		},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr(v1beta1.VendorXAI),
		},
	}

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(testOrg).
		Build()

	rb := ResourceBase{
		Client:    fakeClient,
		Clientset: kfake.NewSimpleClientset(),
	}

	err := os.MkdirAll("/tmp/secrets-store", 0755)
	require.NoError(t, err)

	err = os.WriteFile("/tmp/secrets-store/xai-admin-key", []byte("test-xai-api-key"), 0644)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Remove("/tmp/secrets-store/xai-admin-key")
		os.Remove("/tmp/secrets-store")
	})

	t.Run("InitializeXaiClient", func(t *testing.T) {
		client, err := rb.InitializeXaiClient(context.Background(), testOrg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeXaiClient_NoSecretFile", func(t *testing.T) {
		err := os.Remove("/tmp/secrets-store/xai-admin-key")
		require.NoError(t, err)

		_, err = rb.InitializeXaiClient(context.Background(), testOrg)
		assert.Error(t, err)

		err = os.WriteFile("/tmp/secrets-store/xai-admin-key", []byte("test-xai-api-key"), 0644)
		require.NoError(t, err)
	})

	t.Run("InitializeXaiClient_NoVendor", func(t *testing.T) {
		orgNoVendor := testOrg.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeXaiClient(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeXaiClient_UnsupportedVendor", func(t *testing.T) {
		orgUnsupported := testOrg.DeepCopy()
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeXaiClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}

func generateFakePKCS1PrivateKey() []byte {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		},
	)

	return privateKeyPEM
}

func generateGcpTestData(t *testing.T) (ResourceBase, *v1beta1.Organization) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	fakePrivateKey := string(generateFakePKCS1PrivateKey())
	fakePrivateKey = strings.Replace(fakePrivateKey, "\n", "\\n", -1)

	gcpKey := fmt.Sprintf(`{
  "type": "service_account",
  "project_id": "fake-project",
  "private_key_id": "some-key-id",
  "private_key": "%s",
  "client_email": "fake@fake-project.iam.gserviceaccount.com",
  "client_id": "fake-client-id",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/fake@fake-project.iam.gserviceaccount.com"
}`, fakePrivateKey)
	gcpKeyData := []byte(gcpKey)

	// Create the directory if it doesn't exist
	err := os.MkdirAll("/tmp/secrets-store", 0755)
	require.NoError(t, err)

	// Write the GCP key data to the file
	err = os.WriteFile("/tmp/secrets-store/google-sa-file", gcpKeyData, 0644)
	require.NoError(t, err)

	// Clean up the file after the test
	t.Cleanup(func() {
		os.Remove("/tmp/secrets-store/google-sa-file")
		os.Remove("/tmp/secrets-store")
	})

	org := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{Name: "genai-google-0001"},
		Spec: v1beta1.OrganizationSpec{
			Vendor: vendorPtr("google"),
		},
	}

	// Create fake client
	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(org).
		Build()

	// Create resource base
	rb := ResourceBase{
		Client:    fakeClient,
		Clientset: kfake.NewSimpleClientset(),
	}

	return rb, org
}

func TestResourceBase_InitializeGcpProjectClient(t *testing.T) {
	rb, org := generateGcpTestData(t)

	t.Run("InitializeGcpProjectClient", func(t *testing.T) {

		client, err := rb.InitializeGcpProjectClient(context.Background(), org)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeGcpProjectClient_NoSecret", func(t *testing.T) {
		// Create org with non-existent secret
		orgNoSecret := org.DeepCopy()
		// Remove the file to ensure we get the expected error
		os.Remove("/tmp/secrets-store/google-sa-file")

		_, err := rb.InitializeGcpProjectClient(context.Background(), orgNoSecret)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpProjectClient_NoVendor", func(t *testing.T) {
		// Create org with no vendor
		orgNoVendor := org.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeGcpProjectClient(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpProjectClient_UnsupportedVendor", func(t *testing.T) {
		// Create org with unsupported vendor
		orgUnsupported := org.DeepCopy()
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeGcpProjectClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}
func TestResourceBase_InitializeGcpBillingClient(t *testing.T) {
	rb, org := generateGcpTestData(t)

	t.Run("InitializeGcpBillingClient", func(t *testing.T) {
		client, err := rb.InitializeGcpBillingClient(context.Background(), org)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeGcpBillingClient_NoSecret", func(t *testing.T) {
		// Create org with non-existent secret
		orgNoSecret := org.DeepCopy()

		// Remove the file to ensure we get the expected error
		os.Remove("/tmp/secrets-store/google-sa-file")

		_, err := rb.InitializeGcpBillingClient(context.Background(), orgNoSecret)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpBillingClient_NoVendor", func(t *testing.T) {
		// Create org with no vendor
		orgNoVendor := org.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeGcpBillingClient(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpBillingClient_UnsupportedVendor", func(t *testing.T) {
		// Create org with unsupported vendor
		orgUnsupported := org.DeepCopy()
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeGcpBillingClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}
func TestResourceBase_InitializeGcpIamClient(t *testing.T) {
	rb, org := generateGcpTestData(t)

	t.Run("InitializeGcpIamClient", func(t *testing.T) {
		client, err := rb.InitializeGcpIamClient(context.Background(), org)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeGcpIamClient_NoSecret", func(t *testing.T) {
		// Create org with non-existent secret
		orgNoSecret := org.DeepCopy()

		// Remove the file to ensure we get the expected error
		os.Remove("/tmp/secrets-store/google-sa-file")

		_, err := rb.InitializeGcpIamClient(context.Background(), orgNoSecret)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpIamClient_NoVendor", func(t *testing.T) {
		// Create org with no vendor
		orgNoVendor := org.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeGcpIamClient(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpIamClient_UnsupportedVendor", func(t *testing.T) {
		// Create org with unsupported vendor
		orgUnsupported := org.DeepCopy()
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeGcpIamClient(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}
func TestResourceBase_InitializeGcpServiceUsage(t *testing.T) {
	rb, org := generateGcpTestData(t)

	t.Run("InitializeGcpServiceUsage", func(t *testing.T) {
		client, err := rb.InitializeGcpServiceUsage(context.Background(), org)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("InitializeGcpServiceUsage_NoSecret", func(t *testing.T) {
		// Create org with non-existent secret
		orgNoSecret := org.DeepCopy()

		// Remove the file to ensure we get the expected error
		os.Remove("/tmp/secrets-store/google-sa-file")

		_, err := rb.InitializeGcpServiceUsage(context.Background(), orgNoSecret)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpServiceUsage_NoVendor", func(t *testing.T) {
		// Create org with no vendor
		orgNoVendor := org.DeepCopy()
		orgNoVendor.Spec.Vendor = nil

		_, err := rb.InitializeGcpServiceUsage(context.Background(), orgNoVendor)
		assert.Error(t, err)
	})

	t.Run("InitializeGcpServiceUsage_UnsupportedVendor", func(t *testing.T) {
		// Create org with unsupported vendor
		orgUnsupported := org.DeepCopy()
		orgUnsupported.Spec.Vendor = vendorPtr(v1beta1.VendorUnsupported)

		_, err := rb.InitializeGcpServiceUsage(context.Background(), orgUnsupported)
		assert.Error(t, err)
	})
}

func vendorPtr(v v1beta1.Vendor) *v1beta1.Vendor {
	return &v
}
