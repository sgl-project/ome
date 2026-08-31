package acceleratorquota

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/backend"
)

// recordingBackend captures what the reconciler asked of a backend and in which
// order. Ordering is the point: the reconciler's correctness here is not what it
// writes but when, relative to what it reaps.
type recordingBackend struct {
	calls []string

	// wrote is every node handed to Materialize, in order.
	wrote []string
	// retained is every node the plan asked to leave alone.
	retained sets.Set[string]
	// swept is the keep-set of each Sweep call, in order.
	swept []sets.Set[string]

	outcomes  map[string]backend.NodeOutcome
	failWith  error
	sweepFail error
}

func (b *recordingBackend) Name() string { return "recording" }

func (b *recordingBackend) Materialize(_ context.Context, plan backend.Plan) (backend.Result, error) {
	b.calls = append(b.calls, "materialize")
	for _, n := range plan.Write {
		b.wrote = append(b.wrote, n.Name())
	}
	b.retained = plan.Retain
	if b.failWith != nil {
		return backend.Result{}, b.failWith
	}
	return backend.Result{Nodes: b.outcomes}, nil
}

func (b *recordingBackend) Sweep(_ context.Context, keep sets.Set[string]) (int, error) {
	b.calls = append(b.calls, "sweep")
	b.swept = append(b.swept, keep)
	if b.sweepFail != nil {
		return 0, b.sweepFail
	}
	return 0, nil
}

func materializingReconciler(t *testing.T, b backend.Backend, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	r, c := newReconciler(t, objs...)
	r.Materialize = MaterializeOptions{Backend: b}
	return r, c
}

func deleting(q *v1beta1.AcceleratorQuota) *v1beta1.AcceleratorQuota {
	now := metav1.Now()
	q.DeletionTimestamp = &now
	q.Finalizers = append(q.Finalizers, v1beta1.AcceleratorQuotaFinalizer)
	return q
}

func withFinalizer(q *v1beta1.AcceleratorQuota) *v1beta1.AcceleratorQuota {
	q.Finalizers = append(q.Finalizers, v1beta1.AcceleratorQuotaFinalizer)
	return q
}

// A node being deleted must not be rendered. It is on its way out of the tree,
// and writing its objects in the same pass that reaps them makes the reap look
// like it failed -- and, worse, can leave the objects behind.
func TestReconcileExcludesDeletingNodesFromTheWrite(t *testing.T) {
	tests := []struct {
		name      string
		quotas    []client.Object
		wantWrote []string
	}{
		{
			name: "a live tree writes every node",
			quotas: []client.Object{
				cohort(rootName, ""),
				leaf("team-a", rootName, budget("8")),
			},
			wantWrote: []string{"root", "team-a"},
		},
		{
			name: "a deleting leaf is not written",
			quotas: []client.Object{
				cohort(rootName, ""),
				deleting(leaf("team-a", rootName, budget("8"))),
			},
			wantWrote: []string{"root"},
		},
		{
			name: "a deleting leaf does not stop its sibling being written",
			quotas: []client.Object{
				cohort(rootName, ""),
				deleting(leaf("team-a", rootName, budget("8"))),
				leaf("team-b", rootName, budget("4")),
			},
			wantWrote: []string{"root", "team-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &recordingBackend{}
			r, _ := materializingReconciler(t, b, tc.quotas...)

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			got := append([]string{}, b.wrote...)
			sort.Strings(got)
			want := append([]string{}, tc.wantWrote...)
			sort.Strings(want)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("written nodes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Reaping a departing node has to happen before applying the live tree. The two
// can name the same object transiently -- a leaf deleted and recreated under a
// different parent, say -- and reaping second deletes what the pass just wrote.
func TestReconcileReapsBeforeItApplies(t *testing.T) {
	tests := []struct {
		name      string
		quotas    []client.Object
		wantCalls []string
	}{
		{
			// No departing node, so there is nothing to reap first; the sweep
			// after the apply still runs to collect orphans.
			name: "with nothing deleting, apply then sweep",
			quotas: []client.Object{
				cohort(rootName, ""),
				leaf("team-a", rootName, budget("8")),
			},
			wantCalls: []string{"materialize", "sweep"},
		},
		{
			name: "with a node deleting, the reap sweep comes first",
			quotas: []client.Object{
				cohort(rootName, ""),
				deleting(leaf("team-a", rootName, budget("8"))),
			},
			wantCalls: []string{"sweep", "materialize", "sweep"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &recordingBackend{}
			r, _ := materializingReconciler(t, b, tc.quotas...)

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			if diff := cmp.Diff(tc.wantCalls, b.calls); diff != "" {
				t.Errorf("backend call order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// The freeze rule, from the reconciler's side: a node whose subtree violates an
// invariant is retained rather than written, and the sweep is told to keep it.
// Sweeping it would turn one misauthored parent into a quota outage for every
// tenant beneath it.
func TestReconcileRetainsFrozenNodesAndKeepsTheirObjects(t *testing.T) {
	tests := []struct {
		name         string
		quotas       []client.Object
		wantWritten  []string
		wantRetained []string
	}{
		{
			name: "a clean tree retains nothing",
			quotas: []client.Object{
				cohort(rootName, "", budget("16")),
				leaf("team-a", rootName, budget("8")),
			},
			wantWritten:  []string{"root", "team-a"},
			wantRetained: []string{},
		},
		{
			// Children busting their parent freezes the parent and its subtree.
			// Neither is written, and both must survive the sweep.
			name: "children busting the parent freeze it and are retained",
			quotas: []client.Object{
				cohort(rootName, "", budget("8")),
				leaf("team-a", rootName, budget("8")),
				leaf("team-b", rootName, budget("8")),
			},
			wantWritten:  []string{},
			wantRetained: []string{"root", "team-a", "team-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &recordingBackend{}
			r, _ := materializingReconciler(t, b, tc.quotas...)

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			written := append([]string{}, b.wrote...)
			sort.Strings(written)
			if diff := cmp.Diff(tc.wantWritten, written); diff != "" {
				t.Errorf("written nodes mismatch (-want +got):\n%s", diff)
			}

			retained := b.retained.UnsortedList()
			sort.Strings(retained)
			if diff := cmp.Diff(tc.wantRetained, retained); diff != "" {
				t.Errorf("retained nodes mismatch (-want +got):\n%s", diff)
			}

			// Whatever was frozen must also be in the sweep's keep set, or the
			// sweep collects the very objects the freeze is protecting.
			if len(b.swept) == 0 {
				t.Fatalf("no sweep ran")
			}
			keep := b.swept[len(b.swept)-1]
			for _, name := range tc.wantRetained {
				if !keep.Has(name) {
					t.Errorf("frozen node %q was not in the sweep keep-set %v", name, keep.UnsortedList())
				}
			}
		})
	}
}

// The finalizer is what makes deletion reap rather than orphan. Without it a
// deleted CR leaves its ClusterQueue behind, still admitting workloads against a
// budget nobody can see.
func TestReconcileManagesTheFinalizer(t *testing.T) {
	tests := []struct {
		name          string
		quotas        []client.Object
		materializing bool
		node          string
		wantFinalizer bool
	}{
		{
			name:          "a live node is claimed",
			quotas:        []client.Object{cohort(rootName, ""), leaf("team-a", rootName, budget("8"))},
			materializing: true,
			node:          "team-a",
			wantFinalizer: true,
		},
		{
			// Nothing was written, so there is nothing to reap and no reason to
			// gate anyone's delete.
			name:          "a node is not claimed when materialization is off",
			quotas:        []client.Object{cohort(rootName, ""), leaf("team-a", rootName, budget("8"))},
			materializing: false,
			node:          "team-a",
			wantFinalizer: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var r *Reconciler
			var c client.Client
			if tc.materializing {
				r, c = materializingReconciler(t, &recordingBackend{}, tc.quotas...)
			} else {
				r, c = newReconciler(t, tc.quotas...)
			}

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			var q v1beta1.AcceleratorQuota
			if err := c.Get(context.Background(), client.ObjectKey{Name: tc.node}, &q); err != nil {
				t.Fatalf("get %s: %v", tc.node, err)
			}
			got := false
			for _, f := range q.Finalizers {
				if f == v1beta1.AcceleratorQuotaFinalizer {
					got = true
				}
			}
			if got != tc.wantFinalizer {
				t.Errorf("finalizer present = %v, want %v (finalizers: %v)", got, tc.wantFinalizer, q.Finalizers)
			}
		})
	}
}

// A departing node's finalizer is released only once its objects are reaped, and
// the reap must not take the surviving tree's objects with it.
func TestReconcileReleasesTheFinalizerAfterReaping(t *testing.T) {
	b := &recordingBackend{}
	r, c := materializingReconciler(t, b,
		cohort(rootName, ""),
		deleting(leaf("team-a", rootName, budget("8"))),
		withFinalizer(leaf("team-b", rootName, budget("4"))),
	)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// The fake client removes an object once its last finalizer goes, so the
	// departing node being gone is how the release is observable.
	var gone v1beta1.AcceleratorQuota
	err := c.Get(context.Background(), client.ObjectKey{Name: "team-a"}, &gone)
	if err == nil && len(gone.Finalizers) > 0 {
		t.Errorf("the finalizer was not released: %v", gone.Finalizers)
	}

	if len(b.swept) == 0 {
		t.Fatalf("no sweep ran")
	}
	// The reap's keep-set is the surviving tree. team-a must be absent from it
	// (so its objects go) and team-b present (so its objects stay).
	reap := b.swept[0]
	if reap.Has("team-a") {
		t.Errorf("the departing node was kept, so its objects would never be reaped")
	}
	for _, survivor := range []string{rootName, "team-b"} {
		if !reap.Has(survivor) {
			t.Errorf("surviving node %q was not kept; the reap would delete its objects", survivor)
		}
	}
}

// A backend problem on one node is reported on that node and nowhere else. One
// tenant's typo must not hold every other tenant's quota hostage.
func TestReconcileReportsPerNodeMaterializationOutcome(t *testing.T) {
	tests := []struct {
		name        string
		outcomes    map[string]backend.NodeOutcome
		wantStatus  map[string]metav1.ConditionStatus
		wantReasons map[string]string
	}{
		{
			name:        "a clean pass marks every node Materialized",
			outcomes:    nil,
			wantStatus:  map[string]metav1.ConditionStatus{rootName: metav1.ConditionTrue, "team-a": metav1.ConditionTrue},
			wantReasons: map[string]string{rootName: v1beta1.AcceleratorQuotaReasonAdmitted, "team-a": v1beta1.AcceleratorQuotaReasonAdmitted},
		},
		{
			name: "a missing flavor is reported on that node only",
			outcomes: map[string]backend.NodeOutcome{
				"team-a": {Reason: v1beta1.AcceleratorQuotaReasonFlavorMissing, Message: "no ResourceFlavor named [typo]"},
			},
			wantStatus:  map[string]metav1.ConditionStatus{rootName: metav1.ConditionTrue, "team-a": metav1.ConditionFalse},
			wantReasons: map[string]string{rootName: v1beta1.AcceleratorQuotaReasonAdmitted, "team-a": v1beta1.AcceleratorQuotaReasonFlavorMissing},
		},
		{
			name: "a refused adoption is reported as a conflict, not a failure",
			outcomes: map[string]backend.NodeOutcome{
				"team-a": {Reason: v1beta1.AcceleratorQuotaReasonObjectConflict, Message: "already exists"},
			},
			wantStatus:  map[string]metav1.ConditionStatus{rootName: metav1.ConditionTrue, "team-a": metav1.ConditionFalse},
			wantReasons: map[string]string{rootName: v1beta1.AcceleratorQuotaReasonAdmitted, "team-a": v1beta1.AcceleratorQuotaReasonObjectConflict},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &recordingBackend{outcomes: tc.outcomes}
			r, c := materializingReconciler(t, b,
				cohort(rootName, ""),
				leaf("team-a", rootName, budget("8")),
			)

			if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
				t.Fatalf("Reconcile() error = %v", err)
			}

			gotStatus := map[string]metav1.ConditionStatus{}
			gotReasons := map[string]string{}
			for name := range tc.wantStatus {
				var q v1beta1.AcceleratorQuota
				if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &q); err != nil {
					t.Fatalf("get %s: %v", name, err)
				}
				cond := apimeta.FindStatusCondition(q.Status.Conditions, v1beta1.AcceleratorQuotaMaterialized)
				if cond == nil {
					t.Fatalf("node %s has no Materialized condition", name)
				}
				gotStatus[name] = cond.Status
				gotReasons[name] = cond.Reason
			}

			if diff := cmp.Diff(tc.wantStatus, gotStatus); diff != "" {
				t.Errorf("Materialized status mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantReasons, gotReasons); diff != "" {
				t.Errorf("Materialized reason mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// equalStatus decides whether a status write happens at all. It compared only
// Ready and Degraded, so a pass where nothing moved except Materialized -- or the
// freeze bookkeeping -- was judged a no-op and the condition never reached the
// API at all.
func TestEqualStatusSeesMaterializationChanges(t *testing.T) {
	materialized := func(status metav1.ConditionStatus, reason string) metav1.Condition {
		return metav1.Condition{Type: v1beta1.AcceleratorQuotaMaterialized, Status: status, Reason: reason}
	}

	tests := []struct {
		name string
		a, b v1beta1.AcceleratorQuotaStatus
		want bool
	}{
		{
			name: "identical status is equal",
			a:    v1beta1.AcceleratorQuotaStatus{Conditions: []metav1.Condition{materialized(metav1.ConditionTrue, "Admitted")}},
			b:    v1beta1.AcceleratorQuotaStatus{Conditions: []metav1.Condition{materialized(metav1.ConditionTrue, "Admitted")}},
			want: true,
		},
		{
			name: "the Materialized condition flipping is a change",
			a:    v1beta1.AcceleratorQuotaStatus{Conditions: []metav1.Condition{materialized(metav1.ConditionTrue, "Admitted")}},
			b:    v1beta1.AcceleratorQuotaStatus{Conditions: []metav1.Condition{materialized(metav1.ConditionFalse, "FlavorMissing")}},
			want: false,
		},
		{
			name: "gaining the Materialized condition is a change",
			a:    v1beta1.AcceleratorQuotaStatus{},
			b:    v1beta1.AcceleratorQuotaStatus{Conditions: []metav1.Condition{materialized(metav1.ConditionTrue, "Admitted")}},
			want: false,
		},
		{
			name: "the freeze flag turning on is a change",
			a:    v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{Frozen: false}},
			b:    v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{Frozen: true}},
			want: false,
		},
		{
			name: "a newly applied generation is a change",
			a:    v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{LastAppliedGeneration: 3}},
			b:    v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{LastAppliedGeneration: 4}},
			want: false,
		},
		{
			// LastAppliedTime is deliberately excluded: a clean re-apply restamps
			// it every pass, so including it would make an idle fleet write status
			// on every resync tick and spin its own watch.
			name: "only the applied timestamp moving is not a change",
			a: v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{
				LastAppliedGeneration: 3, LastAppliedTime: ptrTime(metav1.Unix(100, 0))}},
			b: v1beta1.AcceleratorQuotaStatus{Materialization: &v1beta1.AcceleratorQuotaMaterialization{
				LastAppliedGeneration: 3, LastAppliedTime: ptrTime(metav1.Unix(999, 0))}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := equalStatus(tc.a, tc.b); got != tc.want {
				t.Errorf("equalStatus() = %v, want %v", got, tc.want)
			}
		})
	}
}

func ptrTime(t metav1.Time) *metav1.Time { return &t }

// A backend that cannot run at all fails the pass, so it is retried with
// backoff rather than leaving every node reporting a stale condition.
func TestReconcileSurfacesBackendFailure(t *testing.T) {
	tests := []struct {
		name    string
		backend *recordingBackend
		wantErr string
	}{
		{
			name:    "a materialize failure fails the pass",
			backend: &recordingBackend{failWith: errors.New("apiserver is unhappy")},
			wantErr: "apiserver is unhappy",
		},
		{
			name:    "a sweep failure fails the pass",
			backend: &recordingBackend{sweepFail: errors.New("cannot list")},
			wantErr: "cannot list",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := materializingReconciler(t, tc.backend,
				cohort(rootName, ""),
				leaf("team-a", rootName, budget("8")),
			)

			_, err := r.Reconcile(context.Background(), ctrl.Request{})
			if err == nil {
				t.Fatalf("Reconcile() succeeded, want an error mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}
