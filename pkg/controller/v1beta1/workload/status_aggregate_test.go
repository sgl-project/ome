package workload_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

// TestUniqueNodes locks the deterministic ordering + nil-preservation
// contract NodesOccupied callers rely on (status equality checks key on
// the rendered field; non-determinism would churn LastTransitionTime).
func TestUniqueNodes(t *testing.T) {
	pods := []*corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "a"}},
		{Spec: corev1.PodSpec{NodeName: "b"}},
		{Spec: corev1.PodSpec{NodeName: "a"}},
		{Spec: corev1.PodSpec{}}, // unscheduled — skipped
	}
	got := workload.UniqueNodes(pods)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("UniqueNodes: got %v want [a b]", got)
	}
	if got := workload.UniqueNodes(nil); got != nil {
		t.Errorf("nil input should return nil; got %v", got)
	}
}

// TestInstanceMeetsThreshold pins the surge-tolerant classifier: the
// canonical pods always count when their satisfying count meets the
// desired threshold, even if PodCount briefly exceeds desired during a
// surge.
func TestInstanceMeetsThreshold(t *testing.T) {
	tests := []struct {
		name     string
		observed int32
		sat      int32
		desired  int32
		want     bool
	}{
		{name: "all observed satisfying (no plan info)", observed: 2, sat: 2, desired: 0, want: true},
		{name: "fewer observed satisfying than total (no plan info)", observed: 2, sat: 1, desired: 0, want: false},
		{name: "no pods observed", observed: 0, sat: 0, desired: 1, want: false},
		{name: "surge: satisfying < observed but >= desired", observed: 2, sat: 1, desired: 1, want: true},
		{name: "surge: satisfying matches desired exactly", observed: 3, sat: 2, desired: 2, want: true},
		{name: "surge: satisfying short of desired", observed: 3, sat: 1, desired: 2, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workload.InstanceMeetsThreshold(tt.observed, tt.sat, tt.desired); got != tt.want {
				t.Errorf("InstanceMeetsThreshold(observed=%d sat=%d desired=%d): got %v want %v",
					tt.observed, tt.sat, tt.desired, got, tt.want)
			}
		})
	}
}

// TestDesiredFor pins the lookup-with-fallback semantic: present in the
// map → use the value; absent (or nil map) → fall back to observed.
func TestDesiredFor(t *testing.T) {
	m := map[int32]int32{0: 2, 1: 3}
	if got := workload.DesiredFor(m, 0, 7); got != 2 {
		t.Errorf("present: got %d want 2", got)
	}
	if got := workload.DesiredFor(m, 99, 7); got != 7 {
		t.Errorf("absent: got %d want fallback 7", got)
	}
	if got := workload.DesiredFor(nil, 0, 7); got != 7 {
		t.Errorf("nil map: got %d want fallback 7", got)
	}
}

// TestRolloutComplete pins the CurrentRevision promotion gate: every
// Instance must be Phase=Ready AND on the target revision; any deviation
// holds CurrentRevision in place.
func TestRolloutComplete(t *testing.T) {
	allReady := []workload.InstanceStatus{
		{Phase: workload.InstancePhaseReady, RunningRevision: "isvc-engine-aaaaaaaa"},
		{Phase: workload.InstancePhaseReady, RunningRevision: "isvc-engine-aaaaaaaa"},
	}
	if !workload.RolloutComplete(allReady, "isvc-engine-aaaaaaaa") {
		t.Errorf("all Ready on rev should be complete")
	}
	oneCreating := []workload.InstanceStatus{
		{Phase: workload.InstancePhaseCreating, RunningRevision: "isvc-engine-aaaaaaaa"},
		{Phase: workload.InstancePhaseReady, RunningRevision: "isvc-engine-aaaaaaaa"},
	}
	if workload.RolloutComplete(oneCreating, "isvc-engine-aaaaaaaa") {
		t.Errorf("Phase=Creating should keep rollout incomplete")
	}
	stale := []workload.InstanceStatus{
		{Phase: workload.InstancePhaseReady, RunningRevision: "isvc-engine-bbbbbbbb"},
	}
	if workload.RolloutComplete(stale, "isvc-engine-aaaaaaaa") {
		t.Errorf("stale RunningRevision should keep rollout incomplete")
	}
	if workload.RolloutComplete(nil, "isvc-engine-aaaaaaaa") {
		t.Errorf("empty instance list should not be complete")
	}
}

// TestPodStuckPullFailure pins the wait-window + reason classifier:
// fires only on terminal kubelet waiting reasons, after grace, and
// not on freshly-created pods (which legitimately pass through
// transient ContainerCreating).
func TestPodStuckPullFailure(t *testing.T) {
	now := time.Now()
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	transient := corev1.ContainerStateWaiting{Reason: "ContainerCreating"}

	cases := []struct {
		name       string
		pod        *corev1.Pod
		wantStuck  bool
		wantReason string
	}{
		{
			name:      "nil pod returns (false, '')",
			pod:       nil,
			wantStuck: false,
		},
		{
			name: "pod with empty CreationTimestamp returns (false, '')",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
				},
			},
			wantStuck: false,
		},
		{
			name: "within grace: not stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
				},
			},
			wantStuck: false,
		},
		{
			name: "past grace, terminal reason: stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
				},
			},
			wantStuck:  true,
			wantReason: "ImagePullBackOff",
		},
		{
			name: "past grace, transient reason: not stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &transient}}},
				},
			},
			wantStuck: false,
		},
		{
			name: "init container stuck counts too",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
				},
			},
			wantStuck:  true,
			wantReason: "ImagePullBackOff",
		},
		// CrashLoopBackOff is the steady-state for a container
		// that pulls cleanly but exits immediately. kubelet's restart
		// backoff caps at ~5 min/attempt; without the carve-out the
		// rollout would sit at Phase=Updating until the Deadline fired.
		{
			name: "past grace, CrashLoopBackOff: stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}},
				},
			},
			wantStuck:  true,
			wantReason: "CrashLoopBackOff",
		},
		// RunContainerError fires on runtime-rejected starts (missing
		// .so, exec-format mismatch). Same permanence as ImagePullBackOff.
		{
			name: "past grace, RunContainerError: stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "RunContainerError"}}}},
				},
			},
			wantStuck:  true,
			wantReason: "RunContainerError",
		},
		{
			name: "past grace, CreateContainerError: stuck",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Second))},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerError"}}}},
				},
			},
			wantStuck:  true,
			wantReason: "CreateContainerError",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stuck, reason := workload.PodStuckPullFailure(tt.pod, now, 2*time.Second)
			if stuck != tt.wantStuck {
				t.Errorf("stuck: got %v want %v", stuck, tt.wantStuck)
			}
			if reason != tt.wantReason {
				t.Errorf("reason: got %q want %q", reason, tt.wantReason)
			}
		})
	}
}

// TestFirstStuckPodForInstance pins the per-Instance escalator probe:
// skips nil pods + pods marked for deletion + non-stuck pods, returns
// the first eligible stuck pod + reason.
func TestFirstStuckPodForInstance(t *testing.T) {
	now := time.Now()
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	mkPod := func(name string, age time.Duration, deleting bool, reason *corev1.ContainerStateWaiting) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				CreationTimestamp: metav1.NewTime(now.Add(-age)),
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: reason}}},
			},
		}
		if deleting {
			t := metav1.NewTime(now)
			p.ObjectMeta.DeletionTimestamp = &t
		}
		return p
	}

	pods := []*corev1.Pod{
		nil, // skipped
		mkPod("deleting", 10*time.Second, true, &terminal), // skipped (deletion)
		mkPod("ok", 10*time.Second, false, nil),            // skipped (no Waiting)
		mkPod("stuck", 10*time.Second, false, &terminal),   // first eligible
	}
	got, reason := workload.FirstStuckPodForInstance(pods, now, 2*time.Second)
	if got == nil || got.Name != "stuck" {
		t.Errorf("got %+v want pod 'stuck'", got)
	}
	if reason != "ImagePullBackOff" {
		t.Errorf("reason: got %q want ImagePullBackOff", reason)
	}

	// No stuck pods: returns (nil, "").
	none, noneReason := workload.FirstStuckPodForInstance(
		[]*corev1.Pod{mkPod("ok1", 10*time.Second, false, nil)},
		now, 2*time.Second,
	)
	if none != nil || noneReason != "" {
		t.Errorf("no stuck pods: got (%v, %q) want (nil, '')", none, noneReason)
	}
}

// TestHasWedgedPodAgainstCurrent pins the wedged-pod recovery probe:
// disagreement between pod's revision-hash label and currentHash flags
// the wedge; empty currentHash is the no-rollout-ever guard that
// suppresses false fires.
func TestHasWedgedPodAgainstCurrent(t *testing.T) {
	label := "ome.io/revision-hash"
	mk := func(hash string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{label: hash}}}
	}

	cases := []struct {
		name        string
		pods        []*corev1.Pod
		currentHash string
		labelKey    string
		want        bool
	}{
		{name: "empty currentHash never flags", pods: []*corev1.Pod{mk("anything")}, currentHash: "", labelKey: label, want: false},
		{name: "empty labelKey never flags", pods: []*corev1.Pod{mk("anything")}, currentHash: "x", labelKey: "", want: false},
		{name: "all pods match", pods: []*corev1.Pod{mk("x"), mk("x")}, currentHash: "x", labelKey: label, want: false},
		{name: "one pod disagrees", pods: []*corev1.Pod{mk("x"), mk("y")}, currentHash: "x", labelKey: label, want: true},
		{name: "pods missing label are skipped", pods: []*corev1.Pod{{ObjectMeta: metav1.ObjectMeta{}}}, currentHash: "x", labelKey: label, want: false},
		{name: "nil pods are skipped", pods: []*corev1.Pod{nil}, currentHash: "x", labelKey: label, want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := workload.HasWedgedPodAgainstCurrent(tt.pods, tt.currentHash, tt.labelKey); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

// TestShouldCheckForStuckPods pins the two-path qualifier: transient
// phase + Operation qualifies; wedged-pod (label-hash disagreement)
// qualifies; Failed / Deleting never qualify; empty currentHash
// suppresses the wedged branch.
func TestShouldCheckForStuckPods(t *testing.T) {
	label := "ome.io/revision-hash"
	podWithHash := func(hash string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{label: hash}}}
	}

	cases := []struct {
		name        string
		status      *workload.InstanceStatus
		pods        []*corev1.Pod
		currentHash string
		want        bool
	}{
		{name: "nil status", status: nil, want: false},
		{name: "Failed never qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseFailed},
			want:   false,
		},
		{name: "Deleting never qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseDeleting},
			want:   false,
		},
		{name: "Creating + Operation qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseCreating, Operation: &workload.InstanceOperation{}},
			want:   true,
		},
		{name: "Updating + Operation qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseUpdating, Operation: &workload.InstanceOperation{}},
			want:   true,
		},
		{name: "Restarting + Operation qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseRestarting, Operation: &workload.InstanceOperation{}},
			want:   true,
		},
		{name: "Migrating + Operation qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseMigrating, Operation: &workload.InstanceOperation{}},
			want:   true,
		},
		{name: "Creating without Operation falls to wedged-pod check",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseCreating},
			pods:   []*corev1.Pod{podWithHash("x")}, currentHash: "y",
			want: true,
		},
		{name: "Ready + wedged-pod qualifies",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseReady},
			pods:   []*corev1.Pod{podWithHash("y")}, currentHash: "x",
			want: true,
		},
		{name: "Ready + matching pod hash does not qualify",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseReady},
			pods:   []*corev1.Pod{podWithHash("x")}, currentHash: "x",
			want: false,
		},
		{name: "Ready + wedged pod but empty currentHash does not qualify",
			status: &workload.InstanceStatus{Phase: workload.InstancePhaseReady},
			pods:   []*corev1.Pod{podWithHash("y")}, currentHash: "",
			want: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := workload.ShouldCheckForStuckPods(tt.status, tt.pods, tt.currentHash, label); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestReachedDesiredShape(t *testing.T) {
	ready := func(rev string) workload.InstanceStatus {
		return workload.InstanceStatus{Phase: workload.InstancePhaseReady, RunningRevision: rev}
	}
	newRev, oldRev := "isvc-engine-newaaaaa", "isvc-engine-oldbbbbb"
	// 8 instances, partition=2 → want 6 on new + 2 on old, all Ready.
	staged := []workload.InstanceStatus{ready(newRev), ready(newRev), ready(newRev), ready(newRev),
		ready(newRev), ready(newRev), ready(oldRev), ready(oldRev)}
	if !workload.ReachedDesiredShape(staged, newRev, 2, 8) {
		t.Fatal("6 new + 2 held, all Ready → reached")
	}
	if workload.ReachedDesiredShape(staged, newRev, 0, 8) { // partition 0 wants all 8 new
		t.Fatal("partition 0 must require all on new")
	}
	degraded := append([]workload.InstanceStatus{}, staged...)
	degraded[7].Phase = workload.InstancePhaseUpdating
	if workload.ReachedDesiredShape(degraded, newRev, 2, 8) {
		t.Fatal("held instance not Ready → not reached")
	}
	allNew := []workload.InstanceStatus{ready(newRev), ready(newRev)}
	if workload.RolloutComplete(allNew, newRev) != workload.ReachedDesiredShape(allNew, newRev, 0, 2) {
		t.Fatal("RolloutComplete must equal ReachedDesiredShape(...,0,len)")
	}
}

// TestEscalateStuckPodFailures_EscalatesAndEmitsEvent pins the
// integration contract for the escalation pass's wedged-pod recovery
// branch: a wedged pod past grace whose revision-hash label disagrees
// with CurrentRevision flips the InstanceStatus Phase to Failed via
// MutateInstance and fires WarnInstanceFailed exactly once.
func TestEscalateStuckPodFailures_EscalatesAndEmitsEvent(t *testing.T) {
	grace := 1 * time.Millisecond

	now := time.Now()
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "newhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
		},
	}

	// Local mirror of the per-Instance status the closure mutates.
	insts := []workload.InstanceStatus{{Index: 0, Phase: workload.InstancePhaseReady, RunningRevision: "own-engine-oldhash"}}
	var eventFired struct {
		idx    int32
		pod    string
		reason string
		count  int
	}
	input := workload.ReconcileInput{
		StuckPodGrace: grace,
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			for i := range insts {
				if insts[i].Index == idx {
					mutate(&insts[i])
					return nil
				}
			}
			return nil
		},
		WarnInstanceFailed: func(idx int32, podName, reason string) {
			eventFired.idx = idx
			eventFired.pod = podName
			eventFired.reason = reason
			eventFired.count++
		},
	}
	input.ObservedState.InstanceStatuses = insts
	input.ObservedState.CurrentRevision = "own-engine-oldhash"

	err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
		map[int32][]*corev1.Pod{0: {stuck}})
	if err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if insts[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("Phase: got %q want Failed", insts[0].Phase)
	}
	if eventFired.count != 1 {
		t.Errorf("event count: got %d want 1", eventFired.count)
	}
	if eventFired.pod != stuck.Name {
		t.Errorf("event pod: got %q want %q", eventFired.pod, stuck.Name)
	}
	if eventFired.reason != "ImagePullBackOff" {
		t.Errorf("event reason: got %q want ImagePullBackOff", eventFired.reason)
	}
}

// TestEscalateStuckPodFailures_GangSurgeAttributedToSource pins the
// gang-surge attribution: a SOURCE Instance whose in-flight surge lives at
// a SEPARATE index must escalate to Failed when the SURGE gang wedges, even
// though the source's own pods are healthy. Without attributing the surge
// index's stuck pods to the source, a bad-revision gang surge hangs the
// rollout at Phase=Updating forever.
func TestEscalateStuckPodFailures_GangSurgeAttributedToSource(t *testing.T) {
	grace := 1 * time.Millisecond

	now := time.Now()
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-leader-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "oldhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
		},
	}
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	surgeStuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-2-leader-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "badhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
		},
	}

	surgeIdx := int32(2)
	insts := []workload.InstanceStatus{{
		Index:           0,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "own-engine-oldhash",
		Operation: &workload.InstanceOperation{
			Type:       workload.InstanceOperationUpdate,
			Step:       "Surge",
			SurgeIndex: &surgeIdx,
		},
	}}
	var eventFired struct {
		idx   int32
		pod   string
		count int
	}
	input := workload.ReconcileInput{
		StuckPodGrace: grace,
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			for i := range insts {
				if insts[i].Index == idx {
					mutate(&insts[i])
					return nil
				}
			}
			return nil
		},
		WarnInstanceFailed: func(idx int32, podName, _ string) {
			eventFired.idx = idx
			eventFired.pod = podName
			eventFired.count++
		},
	}
	input.ObservedState.InstanceStatuses = insts
	input.ObservedState.CurrentRevision = "own-engine-oldhash"

	err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
		map[int32][]*corev1.Pod{0: {healthy}, 2: {surgeStuck}})
	if err != nil {
		t.Fatalf("escalation pass: %v", err)
	}
	if insts[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("source Phase: got %q want Failed", insts[0].Phase)
	}
	if eventFired.count != 1 || eventFired.idx != 0 {
		t.Errorf("event: got count=%d idx=%d want count=1 idx=0", eventFired.count, eventFired.idx)
	}
	if eventFired.pod != surgeStuck.Name {
		t.Errorf("event pod: got %q want %q (the wedged surge pod)", eventFired.pod, surgeStuck.Name)
	}
}

// TestEscalateStuckPodFailures_GangSurgeIgnoresFailedSource pins corrective
// rollout behavior: once a replacement gang is in flight, a terminal failure
// on the old source must not be attributed to that surge. Otherwise the gang
// recovery path abandons each new replacement before it can start.
func TestEscalateStuckPodFailures_GangSurgeIgnoresFailedSource(t *testing.T) {
	grace := 1 * time.Millisecond

	now := time.Now()
	oldStuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-leader-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "oldhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}},
		},
	}
	replacementHealthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-2-leader-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "newhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	replacementStuck := replacementHealthy.DeepCopy()
	replacementStuck.Name = "engine-2-worker-0"
	replacementStuck.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
	}

	for _, tc := range []struct {
		name         string
		replacement  []*corev1.Pod
		wantPhase    workload.InstancePhase
		wantEventPod string
	}{
		{name: "replacement pods not created yet", wantPhase: workload.InstancePhaseUpdating},
		{name: "replacement pods healthy", replacement: []*corev1.Pod{replacementHealthy}, wantPhase: workload.InstancePhaseUpdating},
		{name: "replacement pod also stuck", replacement: []*corev1.Pod{replacementStuck}, wantPhase: workload.InstancePhaseFailed, wantEventPod: replacementStuck.Name},
	} {
		t.Run(tc.name, func(t *testing.T) {
			surgeIdx := int32(2)
			insts := []workload.InstanceStatus{{
				Index:           0,
				Phase:           workload.InstancePhaseUpdating,
				RunningRevision: "own-engine-oldhash",
				Operation: &workload.InstanceOperation{
					Type:       workload.InstanceOperationUpdate,
					Step:       "Surge",
					SurgeIndex: &surgeIdx,
				},
			}}
			var eventPod string
			input := workload.ReconcileInput{
				StuckPodGrace: grace,
				MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
					for i := range insts {
						if insts[i].Index == idx {
							mutate(&insts[i])
						}
					}
					return nil
				},
				WarnInstanceFailed: func(_ int32, podName, _ string) { eventPod = podName },
			}
			input.ObservedState.InstanceStatuses = insts
			input.ObservedState.CurrentRevision = "own-engine-newhash"

			err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
				map[int32][]*corev1.Pod{
					0: {oldStuck},
					2: tc.replacement,
				})
			if err != nil {
				t.Fatalf("escalation pass: %v", err)
			}
			if insts[0].Phase != tc.wantPhase {
				t.Errorf("source Phase: got %q want %q", insts[0].Phase, tc.wantPhase)
			}
			if eventPod != tc.wantEventPod {
				t.Errorf("event pod: got %q want %q", eventPod, tc.wantEventPod)
			}
		})
	}
}

// TestEscalateStuckPodFailures_AlreadyFailedNoOp pins the idempotency:
// the escalator must not re-fire on an already-Failed Instance even
// though the wedged pod is still in front of it.
func TestEscalateStuckPodFailures_AlreadyFailedNoOp(t *testing.T) {
	grace := 1 * time.Millisecond

	now := time.Now()
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "newhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
		},
	}

	insts := []workload.InstanceStatus{{Index: 0, Phase: workload.InstancePhaseFailed}}
	var eventFired int
	input := workload.ReconcileInput{
		StuckPodGrace: grace,
		MutateInstance: func(_ context.Context, _ int32, _ func(*workload.InstanceStatus) bool) error {
			t.Errorf("MutateInstance must not be called for already-Failed Instance")
			return nil
		},
		WarnInstanceFailed: func(_ int32, _, _ string) { eventFired++ },
	}
	input.ObservedState.InstanceStatuses = insts
	input.ObservedState.CurrentRevision = "own-engine-oldhash"

	err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
		map[int32][]*corev1.Pod{0: {stuck}})
	if err != nil {
		t.Fatalf("escalation pass: %v", err)
	}
	if eventFired != 0 {
		t.Errorf("event count: got %d want 0 (already-Failed must not re-fire)", eventFired)
	}
}

// TestEscalateStuckPodFailures_NoOpWarnCallback pins the panic-on-nil
// contract escape hatch: callers that don't wire a real recorder
// (workload-side unit tests, adapters with no event surface) set an
// explicit no-op closure and the Phase=Failed mutation still lands.
func TestEscalateStuckPodFailures_NoOpWarnCallback(t *testing.T) {
	now := time.Now()
	terminal := corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &terminal}}},
		},
	}
	insts := []workload.InstanceStatus{{Index: 0, Phase: workload.InstancePhaseUpdating, Operation: &workload.InstanceOperation{}}}
	input := workload.ReconcileInput{
		StuckPodGrace: 1 * time.Millisecond,
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			for i := range insts {
				if insts[i].Index == idx {
					mutate(&insts[i])
				}
			}
			return nil
		},
		// Explicit no-op stub — the panic-on-nil contract requires a
		// non-nil callback; adapters that don't emit events wire this
		// shape (see workload.ReconcileInput docs).
		WarnInstanceFailed: func(_ int32, _, _ string) {},
	}
	input.ObservedState.InstanceStatuses = insts
	if err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
		map[int32][]*corev1.Pod{0: {stuck}}); err != nil {
		t.Fatalf("escalation pass with no-op WarnInstanceFailed: %v", err)
	}
	if insts[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("Phase: got %q want Failed (escalation must land even with no-op warn callback)", insts[0].Phase)
	}
}

func TestTakeInlineV1PublicationCountersIgnorePersistedObservationFields(t *testing.T) {
	persisted := []workload.InstanceStatus{
		{Index: 0, PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, AvailablePodCount: 1, RunningRevision: "rev-a"},
		{Index: 1, PodCount: 1, ReadyPodCount: 0, ServingPodCount: 0, AvailablePodCount: 0, RunningRevision: "rev-b"},
	}
	desired := map[int32]int32{0: 1, 1: 1}

	tests := []struct {
		name   string
		target string
	}{
		{name: "without target revision"},
		{name: "with target revision", target: "rev-a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, err := workload.NewOwnedPublicationObservation(
				append([]workload.InstanceStatus(nil), persisted...),
				workload.NewCachedSelectorPodObservation(nil, nil),
				nil,
			)
			if err != nil {
				t.Fatalf("NewOwnedPublicationObservation: %v", err)
			}
			statuses, got, err := observation.TakeInlineV1Publication(desired, test.target)
			if err != nil {
				t.Fatalf("TakeInlineV1Publication: %v", err)
			}
			if got.Replicas != 2 || got.ReadyReplicas != 0 || got.ServingReplicas != 0 || got.AvailableReplicas != 0 {
				t.Errorf("current counters must come from the empty Pod observation: %+v", got)
			}
			if test.target == "" && (got.UpdatedReplicas != 0 || got.UpdatedReadyReplicas != 0) {
				t.Errorf("Updated* should be zero with empty target: %+v", got)
			}
			if test.target != "" && (got.UpdatedReplicas != 1 || got.UpdatedReadyReplicas != 0) {
				t.Errorf("rev-a Updated*: got (%d,%d) want (1,0)", got.UpdatedReplicas, got.UpdatedReadyReplicas)
			}
			for _, status := range statuses {
				if status.PodCount != 0 || status.ReadyPodCount != 0 || status.ServingPodCount != 0 || status.AvailablePodCount != 0 {
					t.Errorf("status %d retained stale observation fields: %+v", status.Index, status)
				}
			}
		})
	}
}

// TestReconcileInput_Compiles is a compile-time pin on the
// ReconcileInput struct shape and the MutateInstance callback
// signature. The struct must be value-constructible with no adapter
// scaffolding, and the MutateInstance signature must accept a
// workload-owned InstanceStatus mutator.
func TestReconcileInput_Compiles(t *testing.T) {
	_ = workload.ReconcileInput{
		MutateInstance: func(ctx context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			// Exercise the mutate callback shape with a no-op caller.
			var s workload.InstanceStatus
			_ = mutate(&s)
			_ = ctx
			_ = idx
			return nil
		},
	}
}

// Stuck-pod escalation must preserve the wedged pod's diagnostics
// into InstanceStatus.LastFailure when it flips the Instance to Failed —
// the escalation is followed by a recreate / teardown that deletes the
// wedged pod, so this is the surviving trace operators read.
func TestEscalateStuckPodFailures_CapturesLastFailure(t *testing.T) {
	grace := 1 * time.Millisecond

	now := time.Now()
	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-leader-0",
			CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
			Labels:            map[string]string{"ome.io/revision-hash": "newhash"},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "back-off pulling image"}},
			}},
		},
	}

	insts := []workload.InstanceStatus{{Index: 0, Phase: workload.InstancePhaseUpdating, RunningRevision: "own-engine-oldhash", Operation: &workload.InstanceOperation{Type: workload.InstanceOperationUpdate, Step: "Drain"}}}
	input := workload.ReconcileInput{
		StuckPodGrace: grace,
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			for i := range insts {
				if insts[i].Index == idx {
					mutate(&insts[i])
					return nil
				}
			}
			return nil
		},
		WarnInstanceFailed: func(_ int32, _, _ string) {},
	}
	input.ObservedState.InstanceStatuses = insts
	input.ObservedState.CurrentRevision = "own-engine-oldhash"

	if err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{},
		map[int32][]*corev1.Pod{0: {stuck}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if insts[0].Phase != workload.InstancePhaseFailed {
		t.Fatalf("Phase: got %q want Failed", insts[0].Phase)
	}
	lf := insts[0].LastFailure
	if lf == nil {
		t.Fatalf("LastFailure: nil, want the wedged pod's diagnostics")
	}
	if lf.PodName != stuck.Name {
		t.Errorf("LastFailure.PodName: got %q want %q", lf.PodName, stuck.Name)
	}
	if lf.Reason != "ImagePullBackOff" {
		t.Errorf("LastFailure.Reason: got %q want ImagePullBackOff", lf.Reason)
	}
	if lf.ContainerName != "main" {
		t.Errorf("LastFailure.ContainerName: got %q want main", lf.ContainerName)
	}
	// A stuck waiting-state wedge never ran a process → no exit code.
	if lf.ExitCode != nil {
		t.Errorf("LastFailure.ExitCode: got %v want nil for a waiting-state wedge", lf.ExitCode)
	}
}
