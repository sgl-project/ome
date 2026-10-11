package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
)

const cordonTestLabel = "maintenance.example.com/start-at"

func cordonFixture(t *testing.T, mutate func(*corev1.Node)) (*MaintenanceCordoner, client.Client, *record.FakeRecorder, *snapshot.ClusterSnapshot, *config.Config) {
	t.Helper()
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "leaving", UID: "uid-1", Labels: map[string]string{cordonTestLabel: "1"}}}
	if mutate != nil {
		mutate(node)
	}
	cl := fake.NewClientBuilder().WithObjects(node).Build()
	recorder := record.NewFakeRecorder(4)
	c := &MaintenanceCordoner{Client: cl, Recorder: recorder, Log: logr.Discard(), Now: func() time.Time { return testNow }}
	cfg := config.Default()
	cfg.Mode = config.ModeExecute
	cfg.Policies.NodeHealth.Maintenance.Triggers = []config.MaintenanceTrigger{
		{Name: "de-schedule", Label: &config.MaintenanceLabel{Key: cordonTestLabel, ValueIsStartTime: true}, Cordon: true},
	}
	snap := &snapshot.ClusterSnapshot{Nodes: map[string]*snapshot.Node{"leaving": {
		Name: "leaving", UID: "uid-1",
		Maintenance: snapshot.NodeMaintenanceObservation{Requested: true, Triggers: []string{"de-schedule"}},
	}}}
	return c, cl, recorder, snap, cfg
}

func liveNode(t *testing.T, cl client.Client) corev1.Node {
	t.Helper()
	var node corev1.Node
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "leaving"}, &node); err != nil {
		t.Fatal(err)
	}
	return node
}

func TestMaintenanceCordonerCordonsNodesUnderCordonRules(t *testing.T) {
	c, cl, recorder, snap, cfg := cordonFixture(t, nil)
	c.Reconcile(context.Background(), snap, cfg)
	node := liveNode(t, cl)
	if !node.Spec.Unschedulable || node.Annotations[CordonedForMaintenanceAnnotation] != "de-schedule" {
		t.Fatalf("node was not cordoned and marked: %+v", node)
	}
	select {
	case event := <-recorder.Events:
		if event == "" {
			t.Fatal("empty event")
		}
	default:
		t.Fatal("no NodeCordonedForMaintenance event")
	}
}

func TestMaintenanceCordonerLeavesNodesAlone(t *testing.T) {
	tests := []struct {
		name   string
		node   func(*corev1.Node)
		adjust func(*snapshot.ClusterSnapshot, *config.Config)
	}{
		{"recommend-only mode", nil, func(_ *snapshot.ClusterSnapshot, cfg *config.Config) { cfg.Mode = config.ModeRecommendOnly }},
		{"rule without cordon", nil, func(_ *snapshot.ClusterSnapshot, cfg *config.Config) {
			cfg.Policies.NodeHealth.Maintenance.Triggers[0].Cordon = false
		}},
		{"maintenance not requested", nil, func(snap *snapshot.ClusterSnapshot, _ *config.Config) {
			snap.Nodes["leaving"].Maintenance = snapshot.NodeMaintenanceObservation{}
		}},
		{"start time not reached on the live node", func(n *corev1.Node) { n.Labels[cordonTestLabel] = "253402300799" }, nil},
		{"label removed since the snapshot", func(n *corev1.Node) { delete(n.Labels, cordonTestLabel) }, nil},
		{"node replaced since the snapshot", func(n *corev1.Node) { n.UID = "uid-2" }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, cl, _, snap, cfg := cordonFixture(t, tt.node)
			if tt.adjust != nil {
				tt.adjust(snap, cfg)
			}
			c.Reconcile(context.Background(), snap, cfg)
			node := liveNode(t, cl)
			if node.Spec.Unschedulable || node.Annotations[CordonedForMaintenanceAnnotation] != "" {
				t.Fatalf("node must stay untouched: %+v", node)
			}
		})
	}
}

func TestMaintenanceCordonerSkipsCordonedNodesAndNeverUncordons(t *testing.T) {
	// Already cordoned by someone else: no write, no annotation.
	c, cl, _, snap, cfg := cordonFixture(t, func(n *corev1.Node) { n.Spec.Unschedulable = true })
	c.Reconcile(context.Background(), snap, cfg)
	if node := liveNode(t, cl); node.Annotations[CordonedForMaintenanceAnnotation] != "" {
		t.Fatalf("an existing cordon must not be claimed: %+v", node)
	}

	// Cordoned by Alfred, then the rule stops matching: the cordon stays.
	c, cl, _, snap, cfg = cordonFixture(t, nil)
	c.Reconcile(context.Background(), snap, cfg)
	snap.Nodes["leaving"].Maintenance = snapshot.NodeMaintenanceObservation{}
	snap.Nodes["leaving"].Cordoned = true
	c.Reconcile(context.Background(), snap, cfg)
	if node := liveNode(t, cl); !node.Spec.Unschedulable {
		t.Fatalf("Alfred must never uncordon: %+v", node)
	}
}

func TestNilMaintenanceCordonerIsANoOp(t *testing.T) {
	var c *MaintenanceCordoner
	c.Reconcile(context.Background(), &snapshot.ClusterSnapshot{}, config.Default())
	if err := c.Fenced(); err != nil {
		t.Fatalf("a nil cordoner fences nothing: %v", err)
	}
}

// TestMaintenanceCordonerFencesFailedCordons checks a failed or conflicting
// cordon is reported by Fenced, naming the node and rule, until a later pass
// cordons the node.
func TestMaintenanceCordonerFencesFailedCordons(t *testing.T) {
	for name, failure := range map[string]error{
		"forbidden": apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, "leaving", errors.New("no patch")),
		"conflict":  apierrors.NewConflict(schema.GroupResource{Resource: "nodes"}, "leaving", errors.New("changed")),
	} {
		t.Run(name, func(t *testing.T) {
			c, _, _, snap, cfg := cordonFixture(t, nil)
			fail := true
			cl := interceptor.NewClient(c.Client.(client.WithWatch), interceptor.Funcs{
				Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					if fail {
						return failure
					}
					return cl.Patch(ctx, obj, patch, opts...)
				},
			})
			c.Client = cl
			c.Reconcile(context.Background(), snap, cfg)
			err := c.Fenced()
			if err == nil || !strings.Contains(err.Error(), "leaving") || !strings.Contains(err.Error(), "de-schedule") {
				t.Fatalf("Fenced must name the node and rule of the failed cordon: %v", err)
			}
			if name == "conflict" && !apierrors.IsConflict(err) {
				t.Fatalf("Fenced must keep the cause: %v", err)
			}
			if node := liveNode(t, cl); node.Spec.Unschedulable {
				t.Fatalf("node must not be cordoned by a failed patch: %+v", node)
			}

			fail = false
			c.Reconcile(context.Background(), snap, cfg)
			if err := c.Fenced(); err != nil {
				t.Fatalf("a later successful cordon must lift the fence: %v", err)
			}
			if node := liveNode(t, cl); !node.Spec.Unschedulable {
				t.Fatalf("node was not cordoned on retry: %+v", node)
			}
		})
	}
}

// TestMaintenanceCordonerReadsThroughReader checks the read before the write
// uses Reader, not the client cache that fed the snapshot.
func TestMaintenanceCordonerReadsThroughReader(t *testing.T) {
	// The cache still shows the label; the API server no longer does.
	c, cl, _, snap, cfg := cordonFixture(t, nil)
	c.Reader = fake.NewClientBuilder().WithObjects(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "leaving", UID: "uid-1"}}).Build()
	c.Reconcile(context.Background(), snap, cfg)
	if node := liveNode(t, cl); node.Spec.Unschedulable {
		t.Fatalf("a node whose label is gone on the API server must not be cordoned: %+v", node)
	}
	if err := c.Fenced(); err != nil {
		t.Fatalf("a rule that no longer matches is not a failure: %v", err)
	}

	// The node is gone from the API server: nothing to fence.
	c, _, _, snap, cfg = cordonFixture(t, nil)
	c.Reader = fake.NewClientBuilder().Build()
	c.Reconcile(context.Background(), snap, cfg)
	if err := c.Fenced(); err != nil {
		t.Fatalf("a deleted node is not a failure: %v", err)
	}
}

// TestMaintenanceFenceWithholdsDispatch checks that no request is submitted
// while a required cordon is failing, and that dispatch resumes once it holds.
func TestMaintenanceFenceWithholdsDispatch(t *testing.T) {
	d, cl, snap, c, cfg, a := dispatchFixture(t, false)
	fence := errors.New("cordon node leaving for maintenance rule \"de-schedule\": forbidden")
	d.MaintenanceFence = func() error { return fence }
	_, decisions := d.Execute(context.Background(), snap, []policy.Candidate{c}, cfg, a)
	got := decisionFor(t, decisions, "prod/a")
	if got.DispatchStatus != "withheld" || got.DispatchReason != "MaintenanceCordonFailed" || cl.patches != 0 {
		t.Fatalf("a failing cordon must withhold dispatch: %+v patches=%d", got, cl.patches)
	}

	fence = nil
	_, decisions = d.Execute(context.Background(), snap, []policy.Candidate{c}, cfg, &Arbiter{Ledger: NewLedger()})
	if got := decisionFor(t, decisions, "prod/a"); got.DispatchStatus != "submitted" || cl.patches != 1 {
		t.Fatalf("dispatch must resume once the fence holds: %+v patches=%d", got, cl.patches)
	}
}
