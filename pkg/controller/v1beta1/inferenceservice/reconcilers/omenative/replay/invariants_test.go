package replay

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	types "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// revisionPod builds a pod of the engine on a revision hash: a default
// runner, Running, and when serving is set PodReady with the serving gate
// True.
func revisionPod(name, hash string, serving bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if serving {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{query.LabelRevisionHash: hash, query.LabelRunner: types.RunnerDefault},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: status},
				{Type: query.ServingConditionType, Status: status},
			},
		},
	}
}

func classesOf(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Class)
	}
	return out
}

func expectClasses(t *testing.T, got []Finding, want []string) {
	t.Helper()
	if classes := classesOf(got); !reflect.DeepEqual(classes, want) {
		t.Fatalf("findings %v, want %v\n%+v", classes, want, got)
	}
}

// TestCheckTrafficBacked holds I2: a traffic target with a share above
// zero names a revision with a pod that is Ready, serving and routable.
func TestCheckTrafficBacked(t *testing.T) {
	const stable, latest = "aaaa1111", "bbbb2222"
	service := func(targets ...v1beta1.ComponentTrafficTarget) *v1beta1.InferenceService {
		isvc := &v1beta1.InferenceService{}
		isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
			v1beta1.EngineComponent: {Traffic: targets},
		}
		return isvc
	}
	split := []v1beta1.ComponentTrafficTarget{
		{RevisionName: "svc-a-engine-rev-" + stable, Percent: 90},
		{RevisionName: "svc-a-engine-rev-" + latest, Percent: 10, LatestRevision: true},
	}
	worker := revisionPod("engine-1-worker-0", latest, true)
	worker.Labels[query.LabelRunner] = types.RunnerWorker
	deleting := revisionPod("engine-1-default-0", latest, true)
	deletion := metav1.NewTime(metav1.Now().Time)
	deleting.DeletionTimestamp = &deletion
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "both revisions serve",
			view: TickView{Service: service(split...), Pods: map[v1beta1.ComponentType][]*corev1.Pod{
				v1beta1.EngineComponent: {revisionPod("engine-0-default-0", stable, true), revisionPod("engine-1-default-0", latest, true)},
			}},
		},
		{
			name: "the latest revision's pod is not serving yet",
			view: TickView{Service: service(split...), Pods: map[v1beta1.ComponentType][]*corev1.Pod{
				v1beta1.EngineComponent: {revisionPod("engine-0-default-0", stable, true), revisionPod("engine-1-default-0", latest, false)},
			}},
			want: []string{"I2/traffic-to-dark-revision[latest,pods-not-serving]"},
		},
		{
			name: "the previous revision has no pod left",
			view: TickView{Service: service(split...), Pods: map[v1beta1.ComponentType][]*corev1.Pod{
				v1beta1.EngineComponent: {revisionPod("engine-1-default-0", latest, true)},
			}},
			want: []string{"I2/traffic-to-dark-revision[previous,no-pods]"},
		},
		{
			name: "a worker and a deleting pod back no traffic",
			view: TickView{Service: service(split...), Pods: map[v1beta1.ComponentType][]*corev1.Pod{
				v1beta1.EngineComponent: {revisionPod("engine-0-default-0", stable, true), worker, deleting},
			}},
			want: []string{"I2/traffic-to-dark-revision[latest,pods-not-serving]"},
		},
		{
			name: "a zero share asks for nothing",
			view: TickView{Service: service(v1beta1.ComponentTrafficTarget{RevisionName: "svc-a-engine-rev-" + stable, Percent: 100},
				v1beta1.ComponentTrafficTarget{RevisionName: "svc-a-engine-rev-" + latest, Percent: 0, LatestRevision: true}),
				Pods: map[v1beta1.ComponentType][]*corev1.Pod{v1beta1.EngineComponent: {revisionPod("engine-0-default-0", stable, true)}}},
		},
		{
			name: "no service, no finding",
			view: TickView{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckTrafficBacked(tc.view), tc.want)
		})
	}
}

// TestCheckRunOpensOnChange holds the run layer to opening on a change: a
// diverged target, a diverged replica pair or a row off the update
// revision each justify an open; none of them is a run on an unchanged
// spec.
func TestCheckRunOpensOnChange(t *testing.T) {
	const current, next = "svc-a-engine-aaaa1111", "svc-a-engine-bbbb2222"
	replica := func(currentRev, updateRev string, rows ...v1beta1.OMENativeInstanceStatus) map[v1beta1.ComponentType]*v1beta1.InferenceReplica {
		ir := &v1beta1.InferenceReplica{}
		ir.Status.CurrentRevision, ir.Status.UpdateRevision = currentRev, updateRev
		ir.Status.InstanceStatuses = rows
		return map[v1beta1.ComponentType]*v1beta1.InferenceReplica{v1beta1.EngineComponent: ir}
	}
	row := func(running string) v1beta1.OMENativeInstanceStatus {
		return v1beta1.OMENativeInstanceStatus{Phase: v1beta1.OMENativeInstanceReady, RunningRevision: running}
	}
	target := func(revision, stable string) []v1beta1.RolloutRunTarget {
		return []v1beta1.RolloutRunTarget{{Component: v1beta1.EngineComponent, Revision: revision, StableRevision: stable}}
	}
	cases := []struct {
		name string
		view TickView
		want []string
	}{
		{
			name: "no run opened",
			view: TickView{Run: RunView{Ran: true}, Replicas: replica(current, current, row(current))},
		},
		{
			name: "a run opens on a diverged target",
			view: TickView{Run: RunView{Ran: true, Opened: true, Targets: target("bbbb2222", "aaaa1111")}, Replicas: replica(current, next, row(current))},
		},
		{
			name: "a run opens on a withdrawn current revision",
			view: TickView{Run: RunView{Ran: true, Opened: true, Targets: target("aaaa1111", "aaaa1111")}, Replicas: replica("", current, row(current))},
		},
		{
			name: "a run opens on a straggling row",
			view: TickView{Run: RunView{Ran: true, Opened: true, Targets: target("aaaa1111", "aaaa1111")}, Replicas: replica(current, current, row(current), row(next))},
		},
		{
			name: "a run opens with nothing to roll",
			view: TickView{Run: RunView{Ran: true, Opened: true, Targets: target("aaaa1111", "aaaa1111")}, Replicas: replica(current, current, row(current))},
			want: []string{"run/opens-on-unchanged-spec[opened]"},
		},
		{
			name: "a run adopted with nothing to roll",
			view: TickView{Run: RunView{Ran: true, Opened: true, Adopted: true, Targets: target("aaaa1111", "aaaa1111")}, Replicas: replica(current, current, row(current))},
			want: []string{"run/opens-on-unchanged-spec[adopted]"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectClasses(t, CheckRunOpensOnChange(tc.view), tc.want)
		})
	}
}

// TestClosedRunWatch holds R: once a run closed Completed with its pinned
// Component Stable, a run toward the same revision or a traffic move with
// the service at the same generation is a finding; a spec change or a run
// toward another revision releases the watch.
func TestClosedRunWatch(t *testing.T) {
	const stable, latest, other = "aaaa1111", "bbbb2222", "cccc3333"
	stamp := func(minute int) *metav1.Time {
		ts := metav1.NewTime(time.Date(2024, 1, 1, 0, minute, 0, 0, time.UTC))
		return &ts
	}
	closed := func(openedMin, closedMin int, outcome v1beta1.RolloutRunOutcome, revision string) *v1beta1.RolloutRunRecord {
		return &v1beta1.RolloutRunRecord{Outcome: outcome, OpenedAt: stamp(openedMin), ClosedAt: stamp(closedMin),
			TargetRevisions: []v1beta1.RolloutRunTarget{{Component: v1beta1.EngineComponent, Revision: revision, StableRevision: stable}}}
	}
	open := func(revision string) *v1beta1.RolloutRun {
		return &v1beta1.RolloutRun{RunID: "r2", OpenedAt: *stamp(12),
			TargetRevisions: []v1beta1.RolloutRunTarget{{Component: v1beta1.EngineComponent, Revision: revision, StableRevision: latest}}}
	}
	view := func(generation int64, active *v1beta1.RolloutRun, last *v1beta1.RolloutRunRecord, phase v1beta1.RolloutPhase, traffic ...v1beta1.ComponentTrafficTarget) TickView {
		isvc := &v1beta1.InferenceService{}
		isvc.Generation = generation
		isvc.Status.Rollout = &v1beta1.RolloutStatus{ActiveRun: active, LastRun: last}
		isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
			v1beta1.EngineComponent: {RolloutPhase: phase, Traffic: traffic},
		}
		return TickView{Service: isvc}
	}
	all := v1beta1.ComponentTrafficTarget{RevisionName: "svc-a-engine-rev-" + latest, Percent: 100, LatestRevision: true}
	split := []v1beta1.ComponentTrafficTarget{
		{RevisionName: "svc-a-engine-rev-" + stable, Percent: 10},
		{RevisionName: "svc-a-engine-rev-" + latest, Percent: 90, LatestRevision: true},
	}
	settled := view(2, nil, closed(0, 11, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhaseStable, all)
	cases := []struct {
		name  string
		ticks []TickView
		want  []string
	}{
		{
			name:  "a run toward the same revision after the close, the spec unchanged",
			ticks: []TickView{settled, view(2, open(latest), nil, v1beta1.RolloutPhasePending, all)},
			want:  []string{"R/run-reopens-after-completed-close[open]"},
		},
		{
			name:  "a run that opened and closed between two ticks",
			ticks: []TickView{settled, view(2, nil, closed(12, 13, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhaseStable, all)},
			want:  []string{"R/run-reopens-after-completed-close[closed]"},
		},
		{
			name:  "a run after a spec change is the next operation",
			ticks: []TickView{settled, view(3, open(latest), nil, v1beta1.RolloutPhasePending, all)},
		},
		{
			name:  "a run toward another revision is the next operation",
			ticks: []TickView{settled, view(2, open(other), nil, v1beta1.RolloutPhasePending, all)},
		},
		{
			name:  "traffic that moves after the close with no run open",
			ticks: []TickView{settled, view(2, nil, closed(0, 11, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhaseStable, split...)},
			want:  []string{"R/traffic-moves-after-completed-close"},
		},
		{
			name: "an entry at zero percent is no move",
			ticks: []TickView{settled, view(2, nil, closed(0, 11, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhaseStable, all,
				v1beta1.ComponentTrafficTarget{RevisionName: "svc-a-engine-rev-" + stable, Percent: 0})},
		},
		{
			name:  "a record that is not a Completed close with a Stable unit arms nothing",
			ticks: []TickView{view(2, nil, closed(0, 11, v1beta1.RolloutRunRolledBack, latest), v1beta1.RolloutPhaseRolledBack, all), view(2, open(latest), nil, v1beta1.RolloutPhasePending, all)},
		},
		{
			name:  "a unit still Paused at the close arms nothing",
			ticks: []TickView{view(2, nil, closed(0, 11, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhasePaused, all), view(2, open(latest), nil, v1beta1.RolloutPhasePending, all)},
		},
		{
			name:  "a service with no rollout status makes no observation",
			ticks: []TickView{{Service: &v1beta1.InferenceService{}}, {}},
		},
		{
			name:  "the reopen is reported once and the next close arms the watch again",
			ticks: []TickView{settled, view(2, open(latest), nil, v1beta1.RolloutPhasePending, all), view(2, open(latest), nil, v1beta1.RolloutPhasePending, all), view(2, nil, closed(12, 14, v1beta1.RolloutRunCompleted, latest), v1beta1.RolloutPhaseStable, all), view(2, open(latest), nil, v1beta1.RolloutPhasePending, all)},
			want:  []string{"R/run-reopens-after-completed-close[open]", "R/run-reopens-after-completed-close[open]"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			watch := &ClosedRunWatch{}
			var got []Finding
			for _, tick := range tc.ticks {
				got = append(got, watch.Observe(tick)...)
			}
			expectClasses(t, got, tc.want)
		})
	}
	// The detail carries the revisions, the close and the generation.
	watch := &ClosedRunWatch{}
	watch.Observe(settled)
	got := watch.Observe(view(2, open(latest), nil, v1beta1.RolloutPhasePending, all))
	if len(got) != 1 || got[0].Detail != "a run opened toward [engine=bbbb2222] after the run that closed Completed on the same revisions at 2024-01-01T00:11:00Z, with the service still at generation 2" {
		t.Fatalf("detail: %+v", got)
	}
}
