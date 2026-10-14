package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sptr "k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func resetTrafficMapEntryMetrics(t *testing.T) {
	t.Helper()
	trafficMapEntryWeight.Reset()
	trafficMapEntryHealthy.Reset()
	t.Cleanup(func() {
		trafficMapEntryWeight.Reset()
		trafficMapEntryHealthy.Reset()
	})
}

// Reading the registry must not create a missing series while checking cleanup.
func assertTrafficMapEntryMetrics(t *testing.T, namespace, isvc string, want map[string][2]float64) {
	t.Helper()
	families, err := ctrlmetrics.Registry.Gather()
	require.NoError(t, err)
	for column, name := range []string{"ome_trafficmap_entry_weight", "ome_trafficmap_entry_healthy"} {
		got := map[string]float64{}
		for _, family := range families {
			if family.GetName() != name {
				continue
			}
			for _, metric := range family.Metric {
				labels := map[string]string{}
				for _, label := range metric.Label {
					labels[label.GetName()] = label.GetValue()
				}
				if labels["namespace"] == namespace && labels["isvc"] == isvc {
					assert.Len(t, labels, 3, "the gauges must join placement metrics on the same labels")
					got[labels["cluster"]] = metric.GetGauge().GetValue()
				}
			}
		}
		expected := make(map[string]float64, len(want))
		for cluster, values := range want {
			expected[cluster] = values[column]
		}
		assert.Equal(t, expected, got, name)
	}
}

func TestTrafficMapEntryMetricsFollowSavedReadiness(t *testing.T) {
	resetTrafficMapEntryMetrics(t)
	isvc := splitISVC([]string{"member-a", "member-b"}, []int32{4, 2}, []int32{4, 2})
	for i := range isvc.Status.Placement.Candidates {
		isvc.Status.Placement.Candidates[i].ObservationKnown = true
	}
	r, c := newReconciler(t, controllerTestConfig(), isvc)
	reconcile(t, r)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
		"member-a": {2, 1}, "member-b": {1, 1},
	})
	registered, err := testutil.GatherAndCount(ctrlmetrics.Registry,
		"ome_trafficmap_entry_weight", "ome_trafficmap_entry_healthy")
	require.NoError(t, err)
	assert.Equal(t, 4, registered, "two gauges must expose both member entries")

	current := &v1beta1.InferenceService{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(isvc), current))
	current.Status.Placement.Candidates[1].ObservationKnown = false
	current.Status.Placement.Candidates[1].ObservationFailingSince = k8sptr.To(metav1.Now())
	require.NoError(t, c.Status().Update(t.Context(), current))
	reconcile(t, r)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
		"member-a": {2, 1}, "member-b": {1, 1},
	})

	current.Status.Placement.Candidates[1].ReadyReplicas = 0
	require.NoError(t, c.Status().Update(t.Context(), current))
	reconcile(t, r)
	tm, found := getTrafficMap(t, c)
	require.True(t, found)
	require.Len(t, tm.Spec.Entries, 2)
	assert.Equal(t, int32(0), tm.Spec.Entries[1].Weight)
	assert.False(t, tm.Spec.Entries[1].Healthy)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
		"member-a": {1, 1}, "member-b": {0, 0},
	})
}

func TestTrafficMapEntryMetricsKeepHealthSeparateFromDrainWeight(t *testing.T) {
	resetTrafficMapEntryMetrics(t)
	isvc := splitISVC([]string{"member-a", "member-b"}, []int32{4, 2}, []int32{4, 2})
	isvc.Annotations = map[string]string{
		constants.TrafficDrainAnnotation: `{"maintenance":{"cluster":"member-b","reason":"maintenance"}}`,
	}
	r, _ := newReconciler(t, controllerTestConfig(), isvc)
	reconcile(t, r)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
		"member-a": {2, 1}, "member-b": {0, 1},
	})
}

func TestTrafficMapEntryMetricsPreserveSavedValuesOnFailedWrites(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			resetTrafficMapEntryMetrics(t)
			isvc := splitISVC([]string{"member-a", "member-b"}, []int32{4, 2}, []int32{4, 2})
			r, c := newReconciler(t, controllerTestConfig(), isvc)
			if operation == "update" {
				reconcile(t, r)
				current := &v1beta1.InferenceService{}
				require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(isvc), current))
				current.Status.Placement.Candidates[1].ReadyReplicas = 0
				require.NoError(t, c.Status().Update(t.Context(), current))
			}
			writeErr := errors.New("TrafficMap spec write rejected")
			interceptors := interceptor.Funcs{
				Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*v1beta1.TrafficMap); ok {
						return writeErr
					}
					return cl.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*v1beta1.TrafficMap); ok {
						return writeErr
					}
					return cl.Update(ctx, obj, opts...)
				},
			}
			r.Client = interceptor.NewClient(c.(client.WithWatch), interceptors)
			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(isvc)})
			require.ErrorIs(t, err, writeErr)
			tm, found := getTrafficMap(t, c)
			if operation == "create" {
				assert.False(t, found)
				assertTrafficMapEntryMetrics(t, testNS, testName, nil)
				return
			}
			require.True(t, found)
			assert.Equal(t, int32(1), tm.Spec.Entries[1].Weight)
			assert.True(t, tm.Spec.Entries[1].Healthy)
			assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
				"member-a": {2, 1}, "member-b": {1, 1},
			})
		})
	}
}

func TestTrafficMapEntryMetricsRecordSavedSpecWhenStatusWriteFails(t *testing.T) {
	resetTrafficMapEntryMetrics(t)
	isvc := splitISVC([]string{"member-a", "member-b"}, []int32{4, 2}, []int32{4, 2})
	r, c := newReconciler(t, controllerTestConfig(), isvc)
	reconcile(t, r)
	current := &v1beta1.InferenceService{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(isvc), current))
	current.Status.Placement.Candidates[1].ReadyReplicas = 0
	require.NoError(t, c.Status().Update(t.Context(), current))
	statusErr := errors.New("TrafficMap status write rejected")
	r.Client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		SubResourceApply: func(context.Context, client.Client, string, runtime.ApplyConfiguration, ...client.SubResourceApplyOption) error {
			return statusErr
		},
	})
	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(isvc)})
	require.ErrorIs(t, err, statusErr)
	tm, found := getTrafficMap(t, c)
	require.True(t, found)
	assert.Equal(t, int32(0), tm.Spec.Entries[1].Weight)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{
		"member-a": {1, 1}, "member-b": {0, 0},
	})
}

func TestTrafficMapEntryMetricsRemoveDroppedMembers(t *testing.T) {
	resetTrafficMapEntryMetrics(t)
	isvc := splitISVC([]string{"member-a", "member-b"}, []int32{4, 2}, []int32{4, 2})
	r, c := newReconciler(t, controllerTestConfig(), isvc)
	reconcile(t, r)
	current := &v1beta1.InferenceService{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(isvc), current))
	current.Status.Placement.Candidates = current.Status.Placement.Candidates[:1]
	require.NoError(t, c.Status().Update(t.Context(), current))
	reconcile(t, r)
	assertTrafficMapEntryMetrics(t, testNS, testName, map[string][2]float64{"member-a": {1, 1}})

	current.Status.Placement.Candidates = nil
	require.NoError(t, c.Status().Update(t.Context(), current))
	reconcile(t, r)
	assertTrafficMapEntryMetrics(t, testNS, testName, nil)
}

func TestTrafficMapEntryMetricsCleanupIsScopedToSource(t *testing.T) {
	for _, cause := range []string{"source absent", "source deleting", "routing disabled", "routing opted out", "placement removed"} {
		t.Run(cause, func(t *testing.T) {
			resetTrafficMapEntryMetrics(t)
			isvc := splitISVC([]string{"member-a"}, []int32{4}, []int32{4})
			if cause == "source deleting" {
				isvc.Finalizers = []string{"example.com/cleanup"}
			}
			otherName := isvc.DeepCopy()
			otherName.Name = "other-service"
			otherName.UID = "other-service-uid"
			otherNamespace := isvc.DeepCopy()
			otherNamespace.Namespace = "team-b"
			otherNamespace.UID = "other-namespace-uid"
			r, c := newReconciler(t, controllerTestConfig(), isvc, otherName, otherNamespace)
			for _, source := range []*v1beta1.InferenceService{isvc, otherName, otherNamespace} {
				_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(source)})
				require.NoError(t, err)
			}
			tm, found := getTrafficMap(t, c)
			require.True(t, found)
			tm.UID = types.UID("traffic-map-uid")
			require.NoError(t, c.Update(t.Context(), tm))
			current := &v1beta1.InferenceService{}
			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(isvc), current))
			switch cause {
			case "source absent", "source deleting":
				require.NoError(t, c.Delete(t.Context(), current))
			case "routing disabled":
				r.Config.Enabled = false
			case "routing opted out":
				current.Spec.Routing = &v1beta1.RoutingSpec{Enabled: k8sptr.To(false)}
				require.NoError(t, c.Update(t.Context(), current))
			case "placement removed":
				current.Spec.Placement = nil
				require.NoError(t, c.Update(t.Context(), current))
			}
			reconcile(t, r)
			assertTrafficMapEntryMetrics(t, testNS, testName, nil)
			assertTrafficMapEntryMetrics(t, testNS, otherName.Name, map[string][2]float64{"member-a": {1, 1}})
			assertTrafficMapEntryMetrics(t, otherNamespace.Namespace, testName, map[string][2]float64{"member-a": {1, 1}})
		})
	}
}
