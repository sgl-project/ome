package acceleratorquota

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/backend"
	"sigs.k8s.io/ome/pkg/quota/tree"
	"sigs.k8s.io/ome/pkg/quota/usage"
)

var cmpQuantity = cmp.Comparer(func(a, b resource.Quantity) bool { return a.Cmp(b) == 0 })

// stubBackend materializes nothing and reports whatever the test hands it, so
// the wiring can be exercised without a Kueue.
type stubBackend struct {
	readings map[string]map[usage.Key]usage.Observed
	readErr  error
	reads    int
}

func (s *stubBackend) Name() string { return "stub" }

func (s *stubBackend) Materialize(context.Context, backend.Plan) (backend.Result, error) {
	return backend.Result{}, nil
}

func (s *stubBackend) ReadUsage(context.Context, []*tree.Node) (map[string]map[usage.Key]usage.Observed, error) {
	s.reads++
	if s.readErr != nil {
		return nil, s.readErr
	}
	return s.readings, nil
}

func tpu(name, flavor string) usage.Key {
	return usage.Key{ResourceName: name, ResourceFlavor: flavor}
}

// The status mirrors what an admin authored, not what the backend happened to
// report. Kueue also reports the cpu and memory cover every queue carries, and
// those are a ceiling rather than an allowance: surfacing them would bury the
// one line a tenant cares about under resources nobody manages.
func TestBudgetStatus(t *testing.T) {
	tests := []struct {
		name   string
		quota  *v1beta1.AcceleratorQuota
		totals map[usage.Key]usage.Total
		want   []v1beta1.AcceleratorBudgetStatus
	}{
		{
			name:  "an authored budget carries what is held against it",
			quota: leaf("team-a", "root", budget("64")),
			totals: map[usage.Key]usage.Total{
				tpu("google.com/tpu", "tpu7x"): {
					Admitted: resource.MustParse("40"),
					Reserved: resource.MustParse("56"),
					Borrowed: resource.MustParse("8"),
				},
			},
			want: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName:   "google.com/tpu",
				ResourceFlavor: "tpu7x",
				Nominal:        resource.MustParse("64"),
				Admitted:       resource.MustParse("40"),
				Reserved:       resource.MustParse("56"),
				Borrowed:       resource.MustParse("8"),
			}},
		},
		{
			// Present at zero rather than omitted: a budget nobody is using and
			// a budget that does not exist are different things to look at.
			name:   "a budget with nothing held is reported at zero",
			quota:  leaf("team-a", "root", budget("64")),
			totals: nil,
			want: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName:   "google.com/tpu",
				ResourceFlavor: "tpu7x",
				Nominal:        resource.MustParse("64"),
				Admitted:       resource.MustParse("0"),
				Reserved:       resource.MustParse("0"),
				Borrowed:       resource.MustParse("0"),
			}},
		},
		{
			// The cover resources every ClusterQueue funds come back in the
			// reading and must not become status lines.
			name:  "consumption of an unbudgeted resource is not reported",
			quota: leaf("team-a", "root", budget("64")),
			totals: map[usage.Key]usage.Total{
				tpu("google.com/tpu", "tpu7x"): {Admitted: resource.MustParse("40")},
				tpu("cpu", "tpu7x"):            {Admitted: resource.MustParse("900")},
				tpu("memory", "tpu7x"):         {Admitted: resource.MustParse("4Ti")},
			},
			want: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName:   "google.com/tpu",
				ResourceFlavor: "tpu7x",
				Nominal:        resource.MustParse("64"),
				Admitted:       resource.MustParse("40"),
				Reserved:       resource.MustParse("0"),
				Borrowed:       resource.MustParse("0"),
			}},
		},
		{
			// A grouping node authors no budget, so it has no budget status —
			// even though its subtree is holding plenty.
			name:  "a node with no authored budget reports none",
			quota: cohort("org", "root"),
			totals: map[usage.Key]usage.Total{
				tpu("google.com/tpu", "tpu7x"): {Admitted: resource.MustParse("40")},
			},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := &tree.Node{Quota: tc.quota}
			got := budgetStatus(node, tc.totals)
			if diff := cmp.Diff(tc.want, got, cmpQuantity); diff != "" {
				t.Errorf("budgetStatus() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Consumption is observational, so how a pass behaves when it cannot be read
// is the whole question: publishing zeros would report an idle fleet on the
// strength of a failed API call, which is exactly when somebody is looking.
func TestReconcileUsageReporting(t *testing.T) {
	held := map[string]map[usage.Key]usage.Observed{
		"team-a": {tpu("google.com/tpu", "tpu7x"): {
			Admitted: resource.MustParse("40"),
			Reserved: resource.MustParse("56"),
			Borrowed: resource.MustParse("8"),
		}},
	}

	tests := []struct {
		name string
		// backend is nil when the manager materializes nothing at all.
		backend  *stubBackend
		wantLeaf []v1beta1.AcceleratorBudgetStatus
		// wantRoot is the parent's rolled-up view of the same consumption.
		wantRoot  []v1beta1.AcceleratorBudgetStatus
		wantReads int
	}{
		{
			name:    "a reading is rolled onto the leaf and its ancestor",
			backend: &stubBackend{readings: held},
			wantLeaf: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Nominal:  resource.MustParse("64"),
				Admitted: resource.MustParse("40"),
				Reserved: resource.MustParse("56"),
				Borrowed: resource.MustParse("8"),
			}},
			wantRoot: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Nominal:  resource.MustParse("128"),
				Admitted: resource.MustParse("40"),
				Reserved: resource.MustParse("56"),
				// A leaf borrowing from a sibling is the parent's own
				// reshuffle, so the parent is not itself borrowing.
				Borrowed: resource.MustParse("0"),
			}},
			wantReads: 1,
		},
		{
			// Nothing materialized yet: the leaf is absent from the reading, so
			// its budget shows zero held rather than an invented figure.
			name:    "a leaf with no queue reports its budget unheld",
			backend: &stubBackend{},
			wantLeaf: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Nominal:  resource.MustParse("64"),
				Admitted: resource.MustParse("0"),
				Reserved: resource.MustParse("0"),
				Borrowed: resource.MustParse("0"),
			}},
			wantRoot: []v1beta1.AcceleratorBudgetStatus{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Nominal:  resource.MustParse("128"),
				Admitted: resource.MustParse("0"),
				Reserved: resource.MustParse("0"),
				Borrowed: resource.MustParse("0"),
			}},
			wantReads: 1,
		},
		{
			// The backend could not be reached. Leave the field unset rather
			// than writing zeros over what was true a moment ago.
			name:      "a failed read publishes no figures at all",
			backend:   &stubBackend{readErr: errors.New("apiserver unreachable")},
			wantLeaf:  nil,
			wantRoot:  nil,
			wantReads: 1,
		},
		{
			// No backend, so nothing to ask. The pass must still write the
			// tree positions and conditions it always did.
			name:      "a manager that materializes nothing reports nothing",
			backend:   nil,
			wantLeaf:  nil,
			wantRoot:  nil,
			wantReads: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, c := newReconciler(t,
				cohort(rootName, "", budget("128")),
				leaf("team-a", rootName, budget("64")),
			)
			if tc.backend != nil {
				r.Materialize = MaterializeOptions{Backend: tc.backend}
			}

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() = %v", err)
			}

			var leafQ, rootQ v1beta1.AcceleratorQuota
			if err := c.Get(context.Background(), client.ObjectKey{Name: "team-a"}, &leafQ); err != nil {
				t.Fatalf("get leaf: %v", err)
			}
			if err := c.Get(context.Background(), client.ObjectKey{Name: rootName}, &rootQ); err != nil {
				t.Fatalf("get root: %v", err)
			}

			if diff := cmp.Diff(tc.wantLeaf, leafQ.Status.Budgets, cmpQuantity); diff != "" {
				t.Errorf("leaf budgets mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantRoot, rootQ.Status.Budgets, cmpQuantity); diff != "" {
				t.Errorf("root budgets mismatch (-want +got):\n%s", diff)
			}
			if tc.backend != nil && tc.backend.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.backend.reads, tc.wantReads)
			}
		})
	}
}

// Consumption is the only part of status that moves on its own. Every other
// field settles after the first pass, so if the change check does not look at
// the figures then a pass carrying nothing but new consumption is taken for an
// idle one and skipped — and the numbers written at startup never move again.
func TestReconcileRewritesChangedConsumption(t *testing.T) {
	stub := &stubBackend{readings: map[string]map[usage.Key]usage.Observed{
		"team-a": {tpu("google.com/tpu", "tpu7x"): {Admitted: resource.MustParse("10")}},
	}}

	r, c := newReconciler(t,
		cohort(rootName, "", budget("128")),
		leaf("team-a", rootName, budget("64")),
	)
	r.Materialize = MaterializeOptions{Backend: stub}

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first Reconcile() = %v", err)
	}

	// Only the consumption changes. Path, parent, generation and every
	// condition are already settled by the pass above.
	stub.readings = map[string]map[usage.Key]usage.Observed{
		"team-a": {tpu("google.com/tpu", "tpu7x"): {Admitted: resource.MustParse("50")}},
	}
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second Reconcile() = %v", err)
	}

	for _, tc := range []struct {
		node string
		want string
	}{
		{node: "team-a", want: "50"},
		{node: rootName, want: "50"},
	} {
		var q v1beta1.AcceleratorQuota
		if err := c.Get(ctx, client.ObjectKey{Name: tc.node}, &q); err != nil {
			t.Fatalf("get %s: %v", tc.node, err)
		}
		if len(q.Status.Budgets) != 1 {
			t.Fatalf("%s has %d budget statuses, want 1", tc.node, len(q.Status.Budgets))
		}
		if got := q.Status.Budgets[0].Admitted; got.Cmp(resource.MustParse(tc.want)) != 0 {
			t.Errorf("%s admitted = %s, want %s (the second pass was skipped)",
				tc.node, got.String(), tc.want)
		}
	}
}

// A read that fails after figures were published must leave them standing.
// Blanking them would turn a transient apiserver blip into a fleet that looks
// idle, and the next successful pass is the only thing that should move them.
func TestReconcileKeepsFiguresWhenAReadFails(t *testing.T) {
	held := map[string]map[usage.Key]usage.Observed{
		"team-a": {tpu("google.com/tpu", "tpu7x"): {Admitted: resource.MustParse("40")}},
	}
	stub := &stubBackend{readings: held}

	r, c := newReconciler(t,
		cohort(rootName, "", budget("128")),
		leaf("team-a", rootName, budget("64")),
	)
	r.Materialize = MaterializeOptions{Backend: stub}

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first Reconcile() = %v", err)
	}

	var before v1beta1.AcceleratorQuota
	if err := c.Get(ctx, client.ObjectKey{Name: "team-a"}, &before); err != nil {
		t.Fatalf("get leaf: %v", err)
	}
	if len(before.Status.Budgets) != 1 || before.Status.Budgets[0].Admitted.Cmp(resource.MustParse("40")) != 0 {
		t.Fatalf("the first pass did not publish figures: %+v", before.Status.Budgets)
	}

	stub.readErr = errors.New("apiserver unreachable")
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second Reconcile() = %v", err)
	}

	var after v1beta1.AcceleratorQuota
	if err := c.Get(ctx, client.ObjectKey{Name: "team-a"}, &after); err != nil {
		t.Fatalf("get leaf: %v", err)
	}
	if diff := cmp.Diff(before.Status.Budgets, after.Status.Budgets, cmpQuantity); diff != "" {
		t.Errorf("figures moved on a failed read (-before +after):\n%s", diff)
	}
}
