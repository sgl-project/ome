package common

import (
	"context"
	"strings"
	"testing"

	testing_pkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
)

const (
	testCommonSecretName      = "common-secret"
	testCommonSecretNamespace = "ome"
)

func setupTestBytePlusServiceAccount(t *testing.T, objects ...client.Object) *BytePlusServiceAccount {
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-project",
		},
		Spec: v1beta1.ProjectSpec{
			Name: "test-project-name",
		},
	}

	testServiceAccount := &v1beta1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-sa",
			UID:        types.UID("e89674fe-af27-4fdd-91ed-34087115d191"),
			Generation: 1,
		},
		Spec: v1beta1.ServiceAccountSpec{
			Name: testing_pkg.StringPtr("test-sa-name"),
			ProjectRef: v1beta1.CrossReference{
				Name: "test-project",
			},
		},
	}

	commonSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCommonSecretName,
			Namespace: testCommonSecretNamespace,
		},
		Data: map[string][]byte{},
	}

	allObjects := []client.Object{testProject, testServiceAccount, commonSecret}
	allObjects = append(allObjects, objects...)

	fakeClient := cfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(allObjects...).
		WithStatusSubresource(testServiceAccount).
		Build()

	fakeClientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.AIPlatformConfigMapName,
			Namespace: constants.OMENamespace,
		},
		Data: map[string]string{
			"aiplatform-config": `{"secretConfig": {"secretName": "common-secret", "namespace": "ome"}}`,
		},
	})

	return NewBytePlusServiceAccount(fakeClient, fakeClientset, logr.Discard(), scheme, testServiceAccount)
}

func TestBytePlusServiceAccount_CreateUsesConfiguredCommonSecret(t *testing.T) {
	oldBytePlusCommonSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ome-byteplus-secrets",
			Namespace: "gs-dp-api-v2",
		},
		Data: map[string][]byte{},
	}
	serviceAccount := setupTestBytePlusServiceAccount(t, oldBytePlusCommonSecret)

	err := serviceAccount.Create(context.Background())
	require.NoError(t, err)
	require.NotNil(t, serviceAccount.Resource.Status.ServiceAccountId)
	serviceAccountID := *serviceAccount.Resource.Status.ServiceAccountId

	commonSecret := &corev1.Secret{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: testCommonSecretName, Namespace: testCommonSecretNamespace}, commonSecret)
	require.NoError(t, err)
	assert.Contains(t, commonSecret.Data, serviceAccountID)

	oldSecret := &corev1.Secret{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "ome-byteplus-secrets", Namespace: "gs-dp-api-v2"}, oldSecret)
	require.NoError(t, err)
	assert.Empty(t, oldSecret.Data)
}

func TestBytePlusServiceAccount_DeleteDeletesAPIKeySecretAndConfiguredCommonSecretEntry(t *testing.T) {
	serviceAccountID := strings.ToLower(GenerateId("user-", types.UID("e89674fe-af27-4fdd-91ed-34087115d191")))
	apiKeySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountID,
			Namespace: bytePlusSecretNamespace,
		},
		Data: map[string][]byte{
			serviceAccountID: []byte("test-api-key"),
		},
	}
	oldBytePlusCommonSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ome-byteplus-secrets",
			Namespace: "gs-dp-api-v2",
		},
		Data: map[string][]byte{
			serviceAccountID: []byte("old-test-api-key"),
		},
	}
	serviceAccount := setupTestBytePlusServiceAccount(t, apiKeySecret, oldBytePlusCommonSecret)
	serviceAccount.Resource.Status.ServiceAccountId = testing_pkg.StringPtr(serviceAccountID)
	serviceAccount.Resource.Status.APIKey = &v1beta1.APIKeySpec{
		Name:     testing_pkg.StringPtr("test-key"),
		APIKeyId: testing_pkg.StringPtr("test-key-id"),
		APIKeySecretRef: &v1beta1.SecretReference{
			Name:      serviceAccountID,
			Namespace: bytePlusSecretNamespace,
			Key:       serviceAccountID,
		},
	}

	commonSecret := &corev1.Secret{}
	err := serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: testCommonSecretName, Namespace: testCommonSecretNamespace}, commonSecret)
	require.NoError(t, err)
	commonSecret.Data = map[string][]byte{}
	commonSecret.Data[serviceAccountID] = []byte("test-api-key")
	commonSecret.Data["other-sa"] = []byte("other-api-key")
	require.NoError(t, serviceAccount.Client.Update(context.Background(), commonSecret))

	err = serviceAccount.Delete(context.Background())
	require.NoError(t, err)

	deletedSecret := &corev1.Secret{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: serviceAccountID, Namespace: bytePlusSecretNamespace}, deletedSecret)
	assert.True(t, apierrors.IsNotFound(err))

	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: testCommonSecretName, Namespace: testCommonSecretNamespace}, commonSecret)
	require.NoError(t, err)
	assert.NotContains(t, commonSecret.Data, serviceAccountID)
	assert.Contains(t, commonSecret.Data, "other-sa")

	oldSecret := &corev1.Secret{}
	err = serviceAccount.Client.Get(context.Background(), client.ObjectKey{Name: "ome-byteplus-secrets", Namespace: "gs-dp-api-v2"}, oldSecret)
	require.NoError(t, err)
	assert.Contains(t, oldSecret.Data, serviceAccountID)
}
