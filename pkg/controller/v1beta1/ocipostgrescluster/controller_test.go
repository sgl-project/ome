package ocipostgrescluster

import (
	"context"
	"testing"

	ocipostgresdbsystem "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ocidbsystem"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	PostgreSQLUtil "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/ocipostgrescluster/utils"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHashCreateDetails_DeterministicAndSensitiveToFields(t *testing.T) {
	d1 := psql.CreateDbSystemDetails{
		DisplayName:   common.String("db-1"),
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		NetworkDetails: &psql.NetworkDetails{
			SubnetId: common.String("ocid1.subnet.oc1..subnetA"),
		},
	}

	d2 := psql.CreateDbSystemDetails{
		DisplayName:   common.String("db-1"),
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		NetworkDetails: &psql.NetworkDetails{
			SubnetId: common.String("ocid1.subnet.oc1..subnetA"),
		},
	}

	d3 := psql.CreateDbSystemDetails{
		DisplayName:   common.String("db-2"), // different
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		NetworkDetails: &psql.NetworkDetails{
			SubnetId: common.String("ocid1.subnet.oc1..subnetA"),
		},
	}

	h1 := hashCreateDetails(d1)
	h2 := hashCreateDetails(d2)
	h3 := hashCreateDetails(d3)

	assert.NotEmpty(t, h1)
	assert.Equal(t, h1, h2, "same inputs should yield same hash")
	assert.NotEqual(t, h1, h3, "changing DisplayName should change hash")
}

func TestAddFinalizerIfNeeded_AddsWhenMissing(t *testing.T) {
	cr := &v1beta1.OciPostgresCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "ns",
			Finalizers: []string{},
		},
	}

	changed := addFinalizerIfNeeded(cr)
	assert.True(t, changed)
	assert.Contains(t, cr.Finalizers, finalizerName)
}

func TestAddFinalizerIfNeeded_NoChangeIfAlreadyPresent(t *testing.T) {
	cr := &v1beta1.OciPostgresCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Finalizers: []string{
				finalizerName,
			},
		},
	}

	changed := addFinalizerIfNeeded(cr)
	assert.False(t, changed)
	assert.Len(t, cr.Finalizers, 1)
	assert.Equal(t, finalizerName, cr.Finalizers[0])
}

func TestRemoveFinalizer_RemovesOnlyOurFinalizer(t *testing.T) {
	cr := &v1beta1.OciPostgresCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Finalizers: []string{
				"other/finalizer",
				finalizerName,
			},
		},
	}

	removeFinalizer(cr)

	assert.Len(t, cr.Finalizers, 1)
	assert.Equal(t, "other/finalizer", cr.Finalizers[0])
}

func TestGetOrCreateAdminCred_CreateWhenNotFound(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	fc := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &DBClusterReconciler{
		Client: fc,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	ns := "db-namespace"
	name := "pg-admin"

	creds, err := r.getOrCreateAdminCred(ctx, ns, name)
	require.NoError(t, err)
	require.NotNil(t, creds)

	// We only care that the function returns sensible values
	assert.Equal(t, "admin", creds.Username)
	assert.NotEmpty(t, creds.Password)
	assert.Equal(t, name, creds.SecretName)

	// And that a Secret object exists
	secret := &corev1.Secret{}
	err = fc.Get(ctx, ktypes.NamespacedName{Namespace: ns, Name: name}, secret)
	require.NoError(t, err)

	// Do NOT check secret.Data[...] here, because fake client does not convert
	// StringData -> Data like the real API server.
	assert.Equal(t, name, secret.Name)
	assert.Equal(t, ns, secret.Namespace)
	assert.Equal(t, "postgresql-cluster", secret.Labels["cluster"])
}

func TestGetOrCreateAdminCred_UsesExistingSecret(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	ns := "db-namespace"
	name := "pg-admin"

	existingUser := "existing-admin"
	existingPass := "existing-pass"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			adminSecretUserKey:     []byte(existingUser),
			adminSecretPasswordKey: []byte(existingPass),
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	r := &DBClusterReconciler{
		Client: fc,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	creds, err := r.getOrCreateAdminCred(ctx, ns, name)
	require.NoError(t, err)
	require.NotNil(t, creds)

	assert.Equal(t, existingUser, creds.Username)
	assert.Equal(t, existingPass, creds.Password)
	assert.Equal(t, name, creds.SecretName)
}

func TestReconcileDelete_RemovesSecretAndFinalizer_WhenDbSystemIdEmpty(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme)) // IMPORTANT: register CRD

	// Cluster name is used as the namespace for the secret in reconcileDelete
	clusterName := "cluster-ns"

	cluster := &v1beta1.OciPostgresCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "ignored-ns",
			Finalizers: []string{
				finalizerName,
				"other/finalizer",
			},
		},
		Status: v1beta1.OciPostgresClusterStatus{
			DbSystemId: "", // ensures DeleteDbSystem is NOT called
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultAdminSecretName,
			Namespace: clusterName,
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, cluster).
		Build()

	r := &DBClusterReconciler{
		Client: fc,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	var pgClientStub ocipostgresdbsystem.OciPostgresClient

	res, err := r.reconcileDelete(ctx, r.Log, cluster, &pgClientStub)
	require.NoError(t, err)

	// No requeue requested
	assert.Equal(t, ctrl.Result{}, res)

	// Secret should be deleted
	s := &corev1.Secret{}
	err = fc.Get(ctx, ktypes.NamespacedName{
		Namespace: clusterName,
		Name:      defaultAdminSecretName,
	}, s)
	assert.Error(t, err, "expected secret to be deleted")

	// Finalizer should be removed from stored cluster object
	updated := &v1beta1.OciPostgresCluster{}
	err = fc.Get(ctx, ktypes.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}, updated)
	require.NoError(t, err)

	for _, f := range updated.Finalizers {
		assert.NotEqual(t, finalizerName, f, "our finalizer should have been removed")
	}
}

func TestGenerateRandomPassword_BasicContract(t *testing.T) {
	pwd := PostgreSQLUtil.GenerateRandomPassword(15)
	assert.Len(t, pwd, 15)
}
