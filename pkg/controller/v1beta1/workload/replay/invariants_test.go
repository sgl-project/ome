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
