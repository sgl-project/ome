package ocirediscluster

import (
	"context"
	"testing"

	ocirediscluster "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociredis"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHashRedisCreateDetails_DeterministicAndSensitiveToFields(t *testing.T) {
	d1 := redis.CreateRedisClusterDetails{
		DisplayName:   common.String("redis-1"),
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		SubnetId:      common.String("ocid1.subnet.oc1..subnetA"),
	}

	d2 := redis.CreateRedisClusterDetails{
		DisplayName:   common.String("redis-1"),
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		SubnetId:      common.String("ocid1.subnet.oc1..subnetA"),
	}

	d3 := redis.CreateRedisClusterDetails{
		DisplayName:   common.String("redis-2"), // different
		CompartmentId: common.String("ocid1.compartment.oc1..example"),
		SubnetId:      common.String("ocid1.subnet.oc1..subnetA"),
	}

	h1 := hashRedisCreateDetails(d1)
	h2 := hashRedisCreateDetails(d2)
	h3 := hashRedisCreateDetails(d3)

	assert.NotEmpty(t, h1)
	assert.Equal(t, h1, h2, "same inputs should yield same hash")
	assert.NotEqual(t, h1, h3, "changing DisplayName should change hash")
}

func TestAddRedisFinalizerIfNeeded_AddsWhenMissing(t *testing.T) {
	cr := &v1beta1.OciRedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "ns",
			Finalizers: []string{},
		},
	}

	changed := addRedisFinalizerIfNeeded(cr)
	assert.True(t, changed)
	assert.Contains(t, cr.Finalizers, redisClusterFinalizerName)
}

func TestAddRedisFinalizerIfNeeded_NoChangeIfAlreadyPresent(t *testing.T) {
	cr := &v1beta1.OciRedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Finalizers: []string{
				redisClusterFinalizerName,
			},
		},
	}

	changed := addRedisFinalizerIfNeeded(cr)
	assert.False(t, changed)
	assert.Len(t, cr.Finalizers, 1)
	assert.Equal(t, redisClusterFinalizerName, cr.Finalizers[0])
}

func TestRemoveRedisFinalizer_RemovesOnlyOurFinalizer(t *testing.T) {
	cr := &v1beta1.OciRedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Finalizers: []string{
				"other/finalizer",
				redisClusterFinalizerName,
			},
		},
	}

	removeRedisFinalizer(cr)

	assert.Len(t, cr.Finalizers, 1)
	assert.Equal(t, "other/finalizer", cr.Finalizers[0])
}

func TestRedisCluster_ReconcileDelete_RemovesFinalizer_WhenClusterIdEmpty(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1beta1.AddToScheme(scheme)) // register CRD types

	cluster := &v1beta1.OciRedisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-cluster",
			Namespace: "ignored-ns", // namespace doesn't matter for delete logic
			Finalizers: []string{
				redisClusterFinalizerName,
				"other/finalizer",
			},
		},
		Status: v1beta1.OciRedisClusterStatus{
			// Empty RedisClusterId means DeleteRedisCluster is NOT called
			RedisClusterId: "",
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		Build()

	r := &RedisClusterReconciler{
		Client: fc,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	// redisClient won't be used because RedisClusterId == ""
	var redisClientStub *ocirediscluster.OciRedisClient = nil

	res, err := r.reconcileDelete(ctx, r.Log, cluster, redisClientStub)
	require.NoError(t, err)

	// No requeue requested
	assert.Equal(t, ctrl.Result{}, res)

	// Finalizer should be removed from stored cluster object
	updated := &v1beta1.OciRedisCluster{}
	err = fc.Get(ctx, ktypes.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}, updated)
	require.NoError(t, err)

	for _, f := range updated.Finalizers {
		assert.NotEqual(t, redisClusterFinalizerName, f, "our finalizer should have been removed")
	}
}
