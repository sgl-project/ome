package replay

import (
	"reflect"
	"testing"

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
