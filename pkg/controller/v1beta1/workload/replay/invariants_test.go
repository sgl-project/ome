package replay

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	types "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// servingPod builds a pod the kubelet reports Ready and the engine's gate
// reports serving.
func servingPod(name string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
	setPodCondition(pod, corev1.ContainersReady, corev1.ConditionTrue, traceStart)
	setPodCondition(pod, corev1.PodReady, corev1.ConditionTrue, traceStart)
	setPodCondition(pod, query.ServingConditionType, corev1.ConditionTrue, traceStart)
	return pod
}

// readyRow builds a settled Ready row publishing one pod.
func readyRow(index int32) types.InstanceStatus {
	return types.InstanceStatus{Index: index, Incarnation: 1, Phase: types.InstancePhaseReady, RunningRevision: "rev-a", PodCount: 1}
}

func intOrString(value string) *intstr.IntOrString {
	parsed := intstr.Parse(value)
	return &parsed
}

func classesOf(findings []Finding) []string {
	var out []string
	for _, finding := range findings {
		out = append(out, finding.Class)
	}
	return out
}

func expectClasses(t *testing.T, got []Finding, want []string) {
	t.Helper()
	if classes := classesOf(got); !reflect.DeepEqual(classes, want) {
		t.Fatalf("findings %v, want %v\n%+v", classes, want, got)
	}
}

// TestCheckCapacity holds I1 to its budget: the floor is replicas minus the
// engine's resolved maxUnavailable, counted over pods that are Ready,
// serving and in a routed rotation.
func TestCheckCapacity(t *testing.T) {
	twoReady := []types.InstanceStatus{readyRow(0), readyRow(1)}
	oneServing := map[int32][]*corev1.Pod{0: {servingPod("p-0")}, 1: {servingPod("p-1")}}
	routed := map[string]string{"p-0": "svc-rev-a"}
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "one of two serves under maxUnavailable one",
			view: TickView{Spec: TickSpec{Replicas: 2, MaxUnavailable: intOrString("1")}, Rows: twoReady, Pods: oneServing, Routed: routed},
		},
		{
			name: "one of two serves under maxUnavailable zero",
			view: TickView{Spec: TickSpec{Replicas: 2, MaxUnavailable: intOrString("0")}, Rows: twoReady, Pods: oneServing, Routed: routed},
			want: []string{"I1/below-floor[Ready]"},
		},
		{
			name: "a serving pod outside every routed rotation does not count",
			view: TickView{Spec: TickSpec{Replicas: 1, MaxUnavailable: intOrString("0")}, Rows: twoReady[:1], Pods: oneServing, Routed: nil},
			want: []string{"I1/below-floor[Ready]"},
		},
		{
			name: "an unset budget checks nothing",
			view: TickView{Spec: TickSpec{Replicas: 2}, Rows: twoReady, Pods: oneServing, Routed: routed},
		},
		{
			name: "a deleted owner checks nothing",
			view: TickView{Spec: TickSpec{Replicas: 2, MaxUnavailable: intOrString("0"), Teardown: true}, Rows: twoReady, Pods: oneServing, Routed: routed},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckCapacity(tc.view), tc.want)
		})
	}
}

// TestCheckBoundedWaits holds I3: an in-flight operation past its deadline
// names a wait or the pass recorded a hold; a settled row runs no clock.
func TestCheckBoundedWaits(t *testing.T) {
	now := traceStart.Add(time.Hour)
	overdue := metav1.NewTime(now.Add(-time.Minute))
	inFlight := func(waiting string) types.InstanceStatus {
		return types.InstanceStatus{Index: 0, Phase: types.InstancePhaseUpdating, Operation: &types.InstanceOperation{
			ID: "update-0-1", Type: types.InstanceOperationUpdate, Step: types.UpdateStepDrain, Deadline: overdue, Waiting: waiting,
		}}
	}
	failed := inFlight("")
	failed.Phase = types.InstancePhaseFailed
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "overdue with a waiting reason",
			view: TickView{Now: now, Rows: []types.InstanceStatus{inFlight(types.RejectionReasonQuotaExceeded)}},
		},
		{
			name: "overdue with a hold recorded",
			view: TickView{Now: now, Rows: []types.InstanceStatus{inFlight("")}, Holds: []types.RolloutHold{{Gate: types.RolloutHoldGateBudget}}},
		},
		{
			name: "overdue with nothing to explain it",
			view: TickView{Now: now, Rows: []types.InstanceStatus{inFlight("")}},
			want: []string{"I3/overdue-unexplained[Update/Drain]"},
		},
		{
			name: "a spent attempt on a Failed row runs no clock",
			view: TickView{Now: now, Rows: []types.InstanceStatus{failed}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckBoundedWaits(tc.view), tc.want)
		})
	}
}

// TestCheckStatusTruth holds I4: Ready means a serving pod, Failed means
// none unless the failure names a replacement, and PodCount is the pod
// count.
func TestCheckStatusTruth(t *testing.T) {
	failedRow := func(failedPod string) types.InstanceStatus {
		row := readyRow(0)
		row.Phase = types.InstancePhaseFailed
		if failedPod != "" {
			row.LastFailure = &types.InstanceTermination{PodName: failedPod, Reason: "Error"}
		}
		return row
	}
	servingAt0 := map[int32][]*corev1.Pod{0: {servingPod("p-0")}}
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "a Ready row with a serving pod",
			view: TickView{Rows: []types.InstanceStatus{readyRow(0)}, Pods: servingAt0},
		},
		{
			name: "a Ready row with no pod",
			view: TickView{Rows: []types.InstanceStatus{readyRow(0)}},
			want: []string{"I4/ready-row-dark[no-pods]", "I4/podcount-differs[Ready]"},
		},
		{
			name: "a Failed row whose failed pod serves",
			view: TickView{Rows: []types.InstanceStatus{failedRow("p-0")}, Pods: servingAt0},
			want: []string{"I4/failed-row-serving[no-operation]"},
		},
		{
			name: "a Failed row whose failure names a replacement beside the serving source",
			view: TickView{Rows: []types.InstanceStatus{failedRow("p-0-replacement")}, Pods: servingAt0},
		},
		{
			name: "a Failed row with no serving pod",
			view: TickView{Rows: []types.InstanceStatus{failedRow("")}, Pods: map[int32][]*corev1.Pod{0: {{ObjectMeta: metav1.ObjectMeta{Name: "p-0"}}}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckStatusTruth(tc.view), tc.want)
		})
	}
}

// TestCheckSingleOwner holds I5: one identity-changing write per index per
// tick, and one operation type per index per tick.
func TestCheckSingleOwner(t *testing.T) {
	ready := readyRow(0)
	opened := readyRow(0)
	opened.Phase = types.InstancePhaseUpdating
	opened.Operation = &types.InstanceOperation{ID: "update-0-1", Type: types.InstanceOperationUpdate, Step: types.UpdateStepInPlace, StartedAt: metav1.NewTime(traceStart)}
	refreshed := CloneRow(opened)
	refreshed.Operation.LastProgressAt = metav1.NewTime(traceStart.Add(time.Second))
	advanced := CloneRow(opened)
	advanced.Operation.Step = types.UpdateStepDrain
	restarted := readyRow(0)
	restarted.Phase = types.InstancePhaseRestarting
	restarted.Operation = &types.InstanceOperation{ID: "restart-0-1", Type: types.InstanceOperationRestart, Step: "Drain"}
	commit := func(before, after *types.InstanceStatus) RowCommit {
		return RowCommit{Index: 0, Outcome: PreconditionOK, Before: before, After: after}
	}
	cases := []struct {
		name    string
		commits []RowCommit
		want    []string
	}{
		{
			name:    "one opening write and one stamp",
			commits: []RowCommit{commit(&ready, &opened), commit(&opened, &refreshed)},
		},
		{
			name:    "a rejected second write committed nothing",
			commits: []RowCommit{commit(&ready, &opened), {Index: 0, Outcome: PreconditionRejected, Before: &opened}},
		},
		{
			name:    "two writes advancing the same operation",
			commits: []RowCommit{commit(&ready, &opened), commit(&opened, &advanced)},
			want:    []string{"I5/identity-writes[Ready>Update/InPlace>Update/Drain]"},
		},
		{
			name:    "one verb closes and another opens",
			commits: []RowCommit{commit(&opened, &ready), commit(&ready, &restarted)},
			want:    []string{"I5/identity-writes[Update/InPlace>Ready>Restart/Drain]", "I5/two-operation-types[Restart,Update]"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckSingleOwner(TickView{Commits: tc.commits}), tc.want)
		})
	}
}

// TestCheckSurgeCeiling holds the surge ceiling: live pods never exceed
// replicas plus the resolved maxSurge, in pods of one Instance each, or
// one replacement per Instance when the plan declares no maxSurge; a pod
// being deleted or in a terminal phase holds no slot; a deleted owner
// checks nothing.
func TestCheckSurgeCeiling(t *testing.T) {
	pod := func(name string) *corev1.Pod { return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}} }
	terminating := pod("p-0-old")
	deletion := metav1.NewTime(traceStart)
	terminating.DeletionTimestamp = &deletion
	failed := pod("p-0-dead")
	failed.Status.Phase = corev1.PodFailed
	pinned := readyRow(1)
	pinned.Phase = types.InstancePhaseFailed
	pinned.Operation = &types.InstanceOperation{ID: "migrate-1-1", Type: types.InstanceOperationMigrate, Step: "CreateSurge"}
	gangTarget := readyRow(1)
	gangTarget.Phase = types.InstancePhaseCreating
	gangTarget.Operation = &types.InstanceOperation{ID: "update-1-1", Type: types.InstanceOperationUpdate, Step: types.UpdateStepGangSurgeTarget}
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "a replacement inside the allowance",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1, MaxSurge: intOrString("1")},
				Rows: []types.InstanceStatus{readyRow(0), pinned},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0")}, 1: {pod("p-1")}},
			},
		},
		{
			name: "a third pod beside a migration pair",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1, MaxSurge: intOrString("1")},
				Rows: []types.InstanceStatus{readyRow(0), pinned},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0")}, 1: {pod("p-1")}, 2: {pod("p-2")}},
			},
			want: []string{"ceiling/live-pods-over[maxSurge][Failed,no-row]"},
		},
		{
			name: "a deleting pod and a dead pod hold no slot",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1, MaxSurge: intOrString("0")},
				Rows: []types.InstanceStatus{readyRow(0)},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0"), terminating, failed}},
			},
		},
		{
			name: "a gang counts its members",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 2, MaxSurge: intOrString("1")},
				Rows: []types.InstanceStatus{readyRow(0), gangTarget},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0-l"), pod("p-0-w")}, 1: {pod("p-1-l"), pod("p-1-w"), pod("p-1-w2")}},
			},
			want: []string{"ceiling/live-pods-over[maxSurge][Update/GangSurgeTarget]"},
		},
		{
			name: "no maxSurge allows one replacement per Instance",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1},
				Rows: []types.InstanceStatus{readyRow(0), pinned},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0")}, 1: {pod("p-1")}},
			},
		},
		{
			name: "no maxSurge and a third pod beside a migration pair",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1},
				Rows: []types.InstanceStatus{readyRow(0), pinned},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0")}, 1: {pod("p-1")}, 2: {pod("p-2")}},
			},
			want: []string{"ceiling/live-pods-over[unbudgeted][Failed,no-row]"},
		},
		{
			name: "a deleted owner checks nothing",
			view: TickView{
				Spec: TickSpec{Replicas: 1, PodsPerInstance: 1, MaxSurge: intOrString("0"), Teardown: true},
				Rows: []types.InstanceStatus{readyRow(0)},
				Pods: map[int32][]*corev1.Pod{0: {pod("p-0")}, 1: {pod("p-1")}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckSurgeCeiling(tc.view), tc.want)
		})
	}
}

// TestCheckOwnership holds the ownership form of I5: one operation owner
// per Instance per tick across its row, the retry record charged in the
// same status write and its migration record; and a Migrate pin that is
// dropped only in a pass that also writes the record it names.
func TestCheckOwnership(t *testing.T) {
	const uuid = "request-1"
	surge := int32(1)
	source := readyRow(0)
	source.Phase = types.InstancePhaseMigrating
	source.Operation = &types.InstanceOperation{ID: "migrate-0-1", Type: types.InstanceOperationMigrate, Step: "CreateSurge", RequestUUID: uuid, SurgeIndex: &surge}
	sourceFailed := CloneRow(source)
	sourceFailed.Phase = types.InstancePhaseFailed
	released := readyRow(0)
	draining := readyRow(0)
	draining.Phase = types.InstancePhaseUpdating
	draining.Operation = &types.InstanceOperation{ID: "update-0-1", Type: types.InstanceOperationUpdate, Step: types.UpdateStepSurgeDrain, TargetRevision: "rev-b"}
	retired := CloneRow(draining)
	retired.Phase = types.InstancePhaseDeleting
	retired.Operation = &types.InstanceOperation{ID: "delete-0-1", Type: types.InstanceOperationDelete, Step: "Drain"}
	created := readyRow(1)
	created.Phase = types.InstancePhaseCreating
	created.Operation = &types.InstanceOperation{ID: "create-1-1", Type: types.InstanceOperationCreate, Step: "CreatePods"}
	open := types.MigrationRecord{RequestUUID: uuid, Phase: types.MigrationPhaseSurgePending, SourceInstance: 0, SurgeInstance: &surge}
	closed := open
	closed.Phase = types.MigrationPhaseFailed
	commit := func(index int32, batch int, before, after *types.InstanceStatus) RowCommit {
		return RowCommit{Index: index, Outcome: PreconditionOK, Batch: batch, Before: before, After: after}
	}
	block := func(batch int) RetryBlockCommit {
		return RetryBlockCommit{TargetRevision: "rev-b", Disposition: RetryBlockPersisted, Batch: batch, After: &types.RetryBlock{TargetRevision: "rev-b", AttemptsStarted: 1}}
	}
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "a scale-down charges the ladder of the attempt it retires",
			view: TickView{Commits: []RowCommit{commit(0, 3, &draining, &retired)}, RetryCommits: []RetryBlockCommit{block(3)}},
			want: []string{"I5/two-owners[Delete,Update][Update/SurgeDrain>Delete/Drain]"},
		},
		{
			name: "a ladder write in another status write charges nothing here",
			view: TickView{Commits: []RowCommit{commit(0, 3, &draining, &retired)}, RetryCommits: []RetryBlockCommit{block(4)}},
		},
		{
			name: "the create pass fills a row the migration record names",
			view: TickView{
				Commits:         []RowCommit{commit(1, 1, nil, &created)},
				MigrationWrites: []MigrationCommit{{RequestUUID: uuid, After: &open}},
				Migrations:      []types.MigrationRecord{open},
			},
			want: []string{"I5/two-owners[Create,Migrate][Empty>Create/CreatePods]"},
		},
		{
			name: "a pin dropped while its record stays open and unwritten",
			view: TickView{Commits: []RowCommit{commit(0, 1, &source, &released)}, Migrations: []types.MigrationRecord{open}},
			want: []string{"I5/pin-dropped-record-open[Migrate/CreateSurge>Ready]"},
		},
		{
			name: "a pin dropped by the pass that closes the record",
			view: TickView{
				Commits:         []RowCommit{commit(0, 1, &source, &released)},
				Migrations:      []types.MigrationRecord{closed},
				MigrationWrites: []MigrationCommit{{RequestUUID: uuid, Before: &open, After: &closed}},
			},
		},
		{
			name: "a pin dropped after its record closed",
			view: TickView{Commits: []RowCommit{commit(0, 1, &source, &released)}, Migrations: []types.MigrationRecord{closed}},
		},
		{
			name: "a pin kept through a Failed stamp",
			view: TickView{Commits: []RowCommit{commit(0, 1, &source, &sourceFailed)}, Migrations: []types.MigrationRecord{open}},
		},
		{
			name: "a deleted owner's rows belong to the teardown",
			view: TickView{Spec: TickSpec{Teardown: true}, Commits: []RowCommit{commit(0, 1, &source, nil)}, Migrations: []types.MigrationRecord{open}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckOwnership(tc.view), tc.want)
		})
	}
}
