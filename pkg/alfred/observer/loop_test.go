package observer

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/metrics"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

var loopNow = time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)

func newTestLoop(t *testing.T) (*Loop, *metrics.Metrics) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{
			"nvidia.com/gpu.product": "NVIDIA-H100-80GB-HBM3",
		}},
		Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{
			"nvidia.com/gpu": resource.MustParse("8"),
		}},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "occupant"},
		Spec: corev1.PodSpec{NodeName: "node1", Containers: []corev1.Container{{
			Name: "main",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("3"),
			}},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "pending"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "main",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("8"),
			}},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(node, pod, pending).Build()

	m := metrics.New(prometheus.NewRegistry())
	loop := &Loop{
		Reader:  fakeClient,
		Store:   config.NewStore(),
		Metrics: m,
		Log:     logr.Discard(),
		Now:     func() time.Time { return loopNow },
	}
	return loop, m
}

func TestLoopRunsOnEveryReplica(t *testing.T) {
	loop, _ := newTestLoop(t)
	if loop.NeedLeaderElection() {
		t.Fatal("observation loop must run on every replica")
	}
}

func TestStartRunsInitialPassAndStopsOnCancel(t *testing.T) {
	loop, _ := newTestLoop(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- loop.Start(ctx) }()

	deadline := time.After(5 * time.Second)
	for loop.Latest() == nil {
		select {
		case <-deadline:
			t.Fatal("Start never completed its initial pass")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start returned error on shutdown: %v", err)
	}
}

func TestRunOncePublishesSnapshotGauges(t *testing.T) {
	loop, m := newTestLoop(t)

	if loop.Latest() != nil {
		t.Fatal("Latest should be nil before the first pass")
	}
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	snap := loop.Latest()
	if snap == nil {
		t.Fatal("Latest should be set after a pass")
	}

	get := func(vec *prometheus.GaugeVec, labels ...string) float64 {
		return promtestutil.ToFloat64(vec.WithLabelValues(labels...))
	}
	if got := get(m.GPUCapacity, "node1", "total"); got != 8 {
		t.Fatalf("gpu_capacity total = %v, want 8", got)
	}
	if got := get(m.GPUCapacity, "node1", "allocated"); got != 3 {
		t.Fatalf("gpu_capacity allocated = %v, want 3", got)
	}
	if got := get(m.GPUCapacity, "node1", "free"); got != 5 {
		t.Fatalf("gpu_capacity free = %v, want 5", got)
	}
	if got := promtestutil.ToFloat64(m.PendingPodCount); got != 1 {
		t.Fatalf("pending_pod_count = %v, want 1", got)
	}
	if got := get(m.PendingPodGPURequirements, "8"); got != 1 {
		t.Fatalf("pending_pod_gpu_requirements{8} = %v, want 1", got)
	}
	if got := get(m.SurgeHeadroomGPUs, "NVIDIA-H100-80GB-HBM3"); got != 5 {
		t.Fatalf("surge_headroom_gpus{pool} = %v, want 5 (largest free block)", got)
	}
}

func TestRefreshWiresNodeSuspicionWindow(t *testing.T) {
	if got := config.Default().NodeSuspicionWindow(); got != snapshot.DefaultNodeSuspicionWindow {
		t.Fatalf("config default window = %v, snapshot default = %v", got, snapshot.DefaultNodeSuspicionWindow)
	}
	loop, _ := newTestLoop(t)
	base, ok := loop.Reader.(client.WithWatch)
	if !ok {
		t.Fatalf("test reader type = %T, want client.WithWatch", loop.Reader)
	}
	transitioned := loopNow.Add(-10 * time.Minute)
	loop.Reader = interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if err := c.List(ctx, list, opts...); err != nil {
				return err
			}
			nodes, ok := list.(*corev1.NodeList)
			if !ok {
				return nil
			}
			for i := range nodes.Items {
				nodes.Items[i].Status.Conditions = []corev1.NodeCondition{
					{
						Type:               "GpuUnhealthy",
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(transitioned),
					},
					{
						Type:               "AcceleratorDegraded",
						Status:             corev1.ConditionFalse,
						LastTransitionTime: metav1.NewTime(transitioned),
					},
				}
			}
			return nil
		},
	})

	if outcome, err := loop.Store.Update([]byte(`
schemaVersion: 1
policies:
  nodeHealth:
    enabled: false
    triggerConditions: [AcceleratorDegraded]
    nodeSuspicionWindowMinutes: 5
`)); err != nil || outcome != config.OutcomeSuccess {
		t.Fatalf("configure five-minute window: outcome=%q error=%v", outcome, err)
	}
	if err := loop.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got := loop.Latest().Nodes["node1"].Health; got.State != snapshot.NodeHealthClear ||
		len(got.Conditions) != 1 || got.Conditions[0].Type != "AcceleratorDegraded" ||
		got.Conditions[0].Status != corev1.ConditionFalse {
		t.Fatalf("five-minute window health = %+v, want Clear", got)
	}

	if outcome, err := loop.Store.Update([]byte(`
schemaVersion: 1
policies:
  nodeHealth:
    enabled: false
    triggerConditions: [AcceleratorDegraded]
    nodeSuspicionWindowMinutes: 15
`)); err != nil || outcome != config.OutcomeSuccess {
		t.Fatalf("configure fifteen-minute window: outcome=%q error=%v", outcome, err)
	}
	if err := loop.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	wantUntil := transitioned.Add(15 * time.Minute)
	if got := loop.Latest().Nodes["node1"].Health; got.State != snapshot.NodeHealthSuspect ||
		got.SuspectUntil == nil || !got.SuspectUntil.Equal(wantUntil) ||
		len(got.Conditions) != 1 || got.Conditions[0].Type != "AcceleratorDegraded" ||
		got.Conditions[0].Status != corev1.ConditionFalse {
		t.Fatalf("fifteen-minute window health = %+v, want Suspect until %v", got, wantUntil)
	}
}

func TestPublishSurgeHeadroomExcludesQuarantinedHealth(t *testing.T) {
	loop, m := newTestLoop(t)
	snap := &snapshot.ClusterSnapshot{Nodes: map[string]*snapshot.Node{
		"clear": {
			Name: "clear", GPUPool: "h100", TotalGPUs: 8, FreeGPUs: 2,
			Health: snapshot.NodeHealthObservation{State: snapshot.NodeHealthClear},
		},
		"suspect": {
			Name: "suspect", GPUPool: "h100", TotalGPUs: 8, FreeGPUs: 6,
			Health: snapshot.NodeHealthObservation{State: snapshot.NodeHealthSuspect},
		},
		"unknown": {
			Name: "unknown", GPUPool: "h100", TotalGPUs: 8, FreeGPUs: 7,
			Health: snapshot.NodeHealthObservation{State: snapshot.NodeHealthUnknown},
		},
		"unhealthy": {
			Name: "unhealthy", GPUPool: "h100", TotalGPUs: 8, FreeGPUs: 8,
			Health: snapshot.NodeHealthObservation{State: snapshot.NodeHealthUnhealthy},
		},
	}}

	loop.publish(snap, config.Default())
	if got := promtestutil.ToFloat64(m.SurgeHeadroomGPUs.WithLabelValues("h100")); got != 2 {
		t.Fatalf("surge headroom = %v, want 2 from the only clear node", got)
	}
}

func TestRunOnceInvokesScorerHook(t *testing.T) {
	loop, _ := newTestLoop(t)

	var scored *snapshot.ClusterSnapshot
	loop.Scorer = func(s *snapshot.ClusterSnapshot, cfg *config.Config, m *metrics.Metrics) {
		scored = s
		m.ClusterFragmentationScore.Set(0.42)
	}
	if err := loop.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if scored == nil || scored != loop.Latest() {
		t.Fatal("scorer must receive the freshly built snapshot")
	}
	if got := promtestutil.ToFloat64(loop.Metrics.ClusterFragmentationScore); got != 0.42 {
		t.Fatalf("scorer gauge write lost: %v", got)
	}
}

func TestRunOnceRecordsOMENativeExecutorState(t *testing.T) {
	tests := []struct {
		name     string
		supplier func(context.Context) snapshot.OMENativeExecutorState
		want     snapshot.OMENativeExecutorState
		gauge    float64
	}{
		{name: "nil supplier", want: snapshot.OMENativeExecutorState{}, gauge: 1},
		{
			name: "explicit unavailable",
			supplier: func(context.Context) snapshot.OMENativeExecutorState {
				return snapshot.OMENativeExecutorState{Reason: "lease-stale"}
			},
			want:  snapshot.OMENativeExecutorState{Reason: "lease-stale"},
			gauge: 1,
		},
		{
			name: "explicit available",
			supplier: func(context.Context) snapshot.OMENativeExecutorState {
				return snapshot.OMENativeExecutorState{
					Available:   true,
					WireVersion: "v2",
					RenewTime:   loopNow.Add(-time.Minute),
					Reason:      "ready",
				}
			},
			want: snapshot.OMENativeExecutorState{
				Available:   true,
				WireVersion: "v2",
				RenewTime:   loopNow.Add(-time.Minute),
				Reason:      "ready",
			},
			gauge: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loop, m := newTestLoop(t)
			loop.OMENativeExecutor = tc.supplier
			if err := loop.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := loop.Latest().OMENativeExecutor; got != tc.want {
				t.Fatalf("executor state = %+v, want %+v", got, tc.want)
			}
			if got := promtestutil.ToFloat64(m.OMENativeUnavailable); got != tc.gauge {
				t.Fatalf("omenative_unavailable = %v, want %v", got, tc.gauge)
			}
		})
	}
}

func TestRefreshHoldsSerializationLockDuringBuildAndPublication(t *testing.T) {
	loop, _ := newTestLoop(t)
	base, ok := loop.Reader.(client.WithWatch)
	if !ok {
		t.Fatalf("test reader type = %T, want client.WithWatch", loop.Reader)
	}

	assertLocked := func(phase string) {
		t.Helper()
		if loop.refreshMu.TryLock() {
			loop.refreshMu.Unlock()
			t.Errorf("refresh serialization lock is not held during %s", phase)
		}
	}
	checkedBuild := false
	checkedPublication := false
	loop.Reader = interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*corev1.NodeList); !ok {
				return c.List(ctx, list, opts...)
			}
			assertLocked("snapshot build")
			checkedBuild = true
			return c.List(ctx, list, opts...)
		},
	})
	loop.Scorer = func(*snapshot.ClusterSnapshot, *config.Config, *metrics.Metrics) {
		assertLocked("snapshot publication")
		checkedPublication = true
	}

	if err := loop.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !checkedBuild {
		t.Error("snapshot build lock assertion was not reached")
	}
	if !checkedPublication {
		t.Error("snapshot publication lock assertion was not reached")
	}
}
