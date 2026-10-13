package compare_test

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading/compare"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// revisionA is the ControllerRevision name whose hash the fixture pods
// carry, so a row running it owns them.
const revisionA = "engine-" + hashA

// readyRow and failedRow are rows running revisionA whose persisted serving
// and available counts agree with each other, as a publication over a set
// with no availability window writes them.
func readyRow(serving int32) *types.InstanceStatus {
	return &types.InstanceStatus{Phase: types.InstancePhaseReady, RunningRevision: revisionA, PodCount: serving, ServingPodCount: serving, AvailablePodCount: serving}
}

func failedRow(serving int32) *types.InstanceStatus {
	return &types.InstanceStatus{Phase: types.InstancePhaseFailed, RunningRevision: revisionA, PodCount: 1, ServingPodCount: serving, AvailablePodCount: serving}
}

// enteredReady anchors the row's entry into Ready at the given instant.
func enteredReady(row *types.InstanceStatus, at time.Time) *types.InstanceStatus {
	t := metav1.NewTime(at)
	row.ReadySince = &t
	return row
}

// unrevisioned is a row that never recorded a running revision.
func unrevisioned(row *types.InstanceStatus) *types.InstanceStatus {
	row.RunningRevision = ""
	return row
}

// updatingRow is a row with an Update attempt in flight, carrying the
// given waiting token, whose publication one pass ago counted the given
// serving pods.
func updatingRow(serving int32, waiting string) *types.InstanceStatus {
	return &types.InstanceStatus{
		Phase:             types.InstancePhaseUpdating,
		RunningRevision:   revisionA,
		PodCount:          1,
		ServingPodCount:   serving,
		AvailablePodCount: serving,
		Operation:         &types.InstanceOperation{Type: types.InstanceOperationUpdate, Waiting: waiting},
	}
}

func classes(ds []compare.Disagreement) string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Reader+" "+d.Class+" ("+d.Detail+")")
	}
	return strings.Join(out, "\n")
}

func TestCompare_PlainFullyServingSetAgrees(t *testing.T) {
	ctx := base()
	ctx.Rotation = reading.Rotation{"a": {}, "b": {}}
	ctx.AvailabilityWindow = 10 * time.Minute
	set := pods(shape("a"), shape("b"))
	if ds := compare.Compare(set, 2, enteredReady(readyRow(2), now.Add(-30*time.Minute)), ctx); len(ds) != 0 {
		t.Fatalf("plain fully-serving set disagrees:\n%s", classes(ds))
	}
	if ds := compare.Compare(set, 2, nil, ctx); len(ds) != 0 {
		t.Fatalf("plain fully-serving set without a row disagrees:\n%s", classes(ds))
	}
	gang := pods(shape("a", asLeader), shape("b", asWorker))
	if ds := compare.Compare(gang, 2, readyRow(2), ctx); len(ds) != 0 {
		t.Fatalf("plain fully-serving gang disagrees:\n%s", classes(ds))
	}
}

// Each crafted set names the readers that must disagree on it and the
// class each disagreement must carry. The three evidence readers and the
// gate admission implement the reading's membership rule and must never
// appear.
func TestCompare_CraftedSetsDisagreePerReader(t *testing.T) {
	// The persisted counts of a row whose pod served and then stopped: the
	// publication one pass ago counted nothing serving.
	darkRow := func() *types.InstanceStatus { return enteredReady(readyRow(0), now.Add(-30*time.Minute)) }
	cases := []struct {
		name    string
		pods    []*corev1.Pod
		desired int32
		row     *types.InstanceStatus
		window  time.Duration
		// rotation replaces the base rotation when set.
		rotation reading.Rotation
		want     map[string]string // reader -> class
	}{
		{
			name:    "gate written this pass, PodReady not folded yet",
			pods:    pods(shape(podName, gateNotFolded)),
			desired: 1,
			row:     readyRow(1),
			want: map[string]string{
				compare.ReaderPodSetFullyServing:     "fullyServing/query=true/reading=false",
				compare.ReaderPersistedServingPods:   "persistedServingCount/row>reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row>reading[routed-not-serving]",
				compare.ReaderReadyReplicas:          "readyReplicas/status=true/reading=false[not-serving]",
				compare.ReaderIsServing:              "gateWritten/podreadiness=true/reading=false[not-folded]",
			},
		},
		{
			name:    "terminal pod whose conditions still read healthy",
			pods:    pods(shape(podName, failed)),
			desired: 1,
			row:     readyRow(1),
			want: map[string]string{
				compare.ReaderCountServingPods:       "servingCount/status>reading",
				compare.ReaderCountReadyPods:         "readyCount/status>reading",
				compare.ReaderPersistedServingPods:   "persistedServingCount/row>reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row>reading",
				compare.ReaderPodReadyAndServing:     "podReadyAndServing/coordination=false/podreadiness=true",
				compare.ReaderIsPodAvailable:         "podAvailable/podreadiness=true/reading=false[not-live]",
				compare.ReaderPodCount:               "podCount/status>reading[terminal]",
				compare.ReaderReadyReplicas:          "readyReplicas/status=true/reading=false[dead-pod]",
				compare.ReaderIsServing:              "gateWritten/podreadiness=true/reading=false[not-live]",
			},
		},
		{
			name:    "gate condition absent, PodReady true",
			pods:    pods(shape(podName, gateAbsent)),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderPodReadyAndServing: "podReadyAndServing/coordination=true/podreadiness=false",
				compare.ReaderIsPodAvailable:     "podAvailable/podreadiness=true/reading=false[not-serving]",
				compare.ReaderReadyReplicas:      "readyReplicas/status=true/reading=false[not-serving]",
				compare.ReaderPodSetPromotable:   "promotable/query=true/reading=false[not-serving]",
			},
		},
		{
			name:    "Failed row whose persisted serving count trails the parked pod by a pass",
			pods:    pods(shape(podName, parkedIn("CrashLoopBackOff"))),
			desired: 1,
			row:     failedRow(1),
			want: map[string]string{
				compare.ReaderRecreateOfDarkRow:      "recreateOfDarkRow/row=false/reading=true",
				compare.ReaderPersistedServingPods:   "persistedServingCount/row>reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row>reading[routed-not-serving]",
				compare.ReaderIsServing:              "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
			},
		},
		{
			name:    "Failed row whose pod came back a pass after the row persisted zero",
			pods:    pods(shape(podName)),
			desired: 1,
			row:     failedRow(0),
			want: map[string]string{
				compare.ReaderRecreateOfDarkRow:      "recreateOfDarkRow/row=true/reading=false",
				compare.ReaderPersistedServingPods:   "persistedServingCount/row<reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row<reading",
			},
		},
		{
			// The counter and the reading both read a pod still coming up
			// as not serving; the grace the reading waits out before it
			// calls a set out of service plays no part in a dark row.
			name:    "Failed row dark on a pod still coming up",
			pods:    pods(shape(podName, neverReady)),
			desired: 1,
			row:     failedRow(0),
			want:    map[string]string{},
		},
		{
			// A Failed row whose source pod still serves is not dark, by
			// the counter it published and by the pods.
			name:    "Failed row whose source still serves beside a failed replacement",
			pods:    pods(shape(podName)),
			desired: 1,
			row:     failedRow(1),
			want:    map[string]string{},
		},
		{
			name:    "pod came back a pass after the row persisted zero",
			pods:    pods(shape(podName)),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderPersistedServingPods:   "persistedServingCount/row<reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row<reading",
			},
		},
		{
			// A pod inside its availability window: PodReady for less
			// than the window, serving on the gate. Both availability
			// readers wait out the window; the persisted count, written
			// before the window elapsed, agrees.
			name:    "pod serving inside its availability window",
			pods:    pods(shape(podName, readySince(now.Add(-time.Second)))),
			desired: 1,
			row:     readyRow(1),
			window:  time.Minute,
			want: map[string]string{
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row>reading",
			},
		},
		{
			// A serving pod no Service publishes yet: the publication
			// counts available pods out of the headless rotation, the
			// reading out of the Serving level.
			name:     "serving pod outside every rotation",
			pods:     pods(shape(podName)),
			desired:  1,
			row:      readyRow(0),
			rotation: reading.Rotation{},
			want: map[string]string{
				compare.ReaderPersistedServingPods:   "persistedServingCount/row<reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row<reading[unrouted]",
			},
		},
		{
			// A pod being deleted keeps its conditions: the publication's
			// pod and ready counts, its availability predicate and the
			// presence count still read it, podreadiness reads the gate,
			// and the reading reads a deleting pod at no level, as
			// coordination does.
			name:    "pod being deleted with healthy conditions",
			pods:    pods(shape(podName, terminating)),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderCountReadyPods:     "readyCount/status>reading",
				compare.ReaderPodReadyAndServing: "podReadyAndServing/coordination=false/podreadiness=true",
				compare.ReaderIsPodAvailable:     "podAvailable/podreadiness=true/reading=false[not-live]",
				compare.ReaderPodCount:           "podCount/status>reading[terminating]",
				compare.ReaderReadyReplicas:      "readyReplicas/status=true/reading=false[dead-pod]",
				compare.ReaderLivePods:           "liveCount/query>reading",
				compare.ReaderIsServing:          "gateWritten/podreadiness=true/reading=false[not-live]",
			},
		},
		{
			// The pods read fully serving while the operation reports the
			// source out of the rotation under another hand: the
			// escalation reads no capacity, the reading reads the pods.
			name:    "surge source taken out of the rotation under another hand",
			pods:    pods(shape(podName)),
			desired: 1,
			row:     updatingRow(1, types.WaitingReasonSourceUnrouted),
			want: map[string]string{
				compare.ReaderPodSetIsCapacity: "podSetIsCapacity/escalation=false/reading=true[source-unrouted]",
			},
		},
		{
			// The runner restarted after Ready and has failed its
			// readiness past the grace since: the roll reads a crash,
			// which the repair owns, so the Instance keeps its slot; the
			// composition reads a dark pod. The detector and the reading
			// agree the pod is broken.
			name:    "rolled Instance dark but for a runner restarted since Ready",
			pods:    pods(shape(podName, restartedAt(now.Add(-5*time.Minute)), unreadySince(now.Add(-2*time.Minute)))),
			desired: 1,
			row:     darkRow(),
			want: map[string]string{
				compare.ReaderRolledInstanceDark: "rolledInstanceDark/ops=false/reading=true[crashed]",
				compare.ReaderIsServing:          "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
			},
		},
		{
			// The runner exited and the kubelet has not started it again:
			// the roll reads no dark pod, the kubelet having named the
			// failure; the composition reads a pod unready past the
			// grace, and so does the reading beside the detector, which
			// reads no restart on a container that has not run again.
			name:    "rolled Instance dark but for a runner that exited",
			pods:    pods(shape(podName, exitedAt(now.Add(-2*time.Minute)))),
			desired: 1,
			row:     darkRow(),
			want: map[string]string{
				compare.ReaderRolledInstanceDark: "rolledInstanceDark/ops=false/reading=true[exited]",
				compare.ReaderIsServing:          "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
				compare.ReaderRunnerRestarted:    "runnerRestarted/ops=false/reading=out-of-service[no-restart]",
			},
		},
		{
			// A dark pod under an update attempt the roll continues: the
			// attempt's own counts hold its slot, so the roll reads no
			// dark Instance.
			name:    "dark pod under an update attempt in flight",
			pods:    pods(shape(podName, unreadySince(now.Add(-2*time.Minute)))),
			desired: 1,
			row:     updatingRow(0, ""),
			want: map[string]string{
				compare.ReaderRolledInstanceDark: "rolledInstanceDark/ops=false/reading=true[continuation]",
				compare.ReaderIsServing:          "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
			},
		},
		{
			// A promoted pod that failed its readiness past the grace
			// with its container running and no restart: the roll and
			// the composition both read the Instance dark. The detector
			// has no Ready anchor on the row and reads no restart.
			name:    "dark pod the roll and the reading agree on",
			pods:    pods(shape(podName, unreadySince(now.Add(-2*time.Minute)))),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderIsServing:       "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
				compare.ReaderRunnerRestarted: "runnerRestarted/ops=false/reading=out-of-service[no-anchor]",
			},
		},
		{
			name:    "runner restarted since Ready and serving again",
			pods:    pods(shape(podName, restartedAt(now.Add(-5*time.Minute)), readySince(now.Add(-4*time.Minute)))),
			desired: 1,
			row:     enteredReady(readyRow(1), now.Add(-30*time.Minute)),
			want: map[string]string{
				compare.ReaderRunnerRestarted: "runnerRestarted/ops=true/reading=serving",
			},
		},
		{
			name:    "runner restarted since Ready, coming back inside the grace",
			pods:    pods(shape(podName, restartedAt(now.Add(-5*time.Minute)), unreadySince(now.Add(-10*time.Second)))),
			desired: 1,
			row:     darkRow(),
			want: map[string]string{
				compare.ReaderRunnerRestarted: "runnerRestarted/ops=true/reading=coming-up",
				compare.ReaderIsServing:       "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
			},
		},
		{
			// A pod created after the row entered Ready is a rebuild, not
			// one of the pods that were Ready then: the detector reads no
			// restart on it, however it is parked.
			name:    "pod rebuilt after the row entered Ready, parked in a crash loop",
			pods:    pods(shape(podName, createdAt(now.Add(-10*time.Minute)), parkedIn("CrashLoopBackOff"))),
			desired: 1,
			row:     darkRow(),
			want: map[string]string{
				compare.ReaderRunnerRestarted: "runnerRestarted/ops=false/reading=out-of-service[rebuilt]",
				compare.ReaderIsServing:       "gateWritten/podreadiness=true/reading=false[containers-not-ready]",
			},
		},
		{
			name:    "Failed pod of a Ready row whose runner died after Ready",
			pods:    pods(shape(podName, restartedAt(now.Add(-5*time.Minute)), failed)),
			desired: 1,
			row:     enteredReady(readyRow(1), now.Add(-30*time.Minute)),
			want: map[string]string{
				compare.ReaderCountServingPods:       "servingCount/status>reading",
				compare.ReaderCountReadyPods:         "readyCount/status>reading",
				compare.ReaderPersistedServingPods:   "persistedServingCount/row>reading",
				compare.ReaderPersistedAvailablePods: "persistedAvailableCount/row>reading",
				compare.ReaderPodReadyAndServing:     "podReadyAndServing/coordination=false/podreadiness=true",
				compare.ReaderIsPodAvailable:         "podAvailable/podreadiness=true/reading=false[not-live]",
				compare.ReaderPodCount:               "podCount/status>reading[terminal]",
				compare.ReaderReadyReplicas:          "readyReplicas/status=true/reading=false[dead-pod]",
				compare.ReaderIsServing:              "gateWritten/podreadiness=true/reading=false[not-live]",
				compare.ReaderRunnerRestarted:        "runnerRestarted/ops=true/reading=not-live",
			},
		},
		{
			// Adoption reads a set ready on its containers; the gate is
			// written and not folded yet, so nothing serves.
			name:    "Failed row with no running revision, pods ready on their containers alone",
			pods:    pods(shape(podName, gateNotFolded)),
			desired: 1,
			row:     unrevisioned(failedRow(0)),
			want: map[string]string{
				compare.ReaderAdoption:           "adopt/query=true/reading=false[not-serving]",
				compare.ReaderPodSetFullyServing: "fullyServing/query=true/reading=false",
				compare.ReaderReadyReplicas:      "readyReplicas/status=true/reading=false[not-serving]",
				compare.ReaderIsServing:          "gateWritten/podreadiness=true/reading=false[not-folded]",
			},
		},
		{
			name:    "Failed row with no running revision, pods serving inside the window",
			pods:    pods(shape(podName, readySince(now.Add(-time.Second)))),
			desired: 1,
			row:     unrevisioned(&types.InstanceStatus{Phase: types.InstancePhaseFailed, PodCount: 1, ServingPodCount: 1}),
			window:  time.Minute,
			want: map[string]string{
				compare.ReaderAdoption: "adopt/query=true/reading=false[window]",
			},
		},
		{
			// A shape no kubelet writes, pinning the classes of the
			// readers that read ContainersReady where the reading reads
			// Serving.
			name:    "containers stopped answering while PodReady and the gate read True",
			pods:    pods(shape(podName, containersUnready)),
			desired: 1,
			row:     readyRow(1),
			want: map[string]string{
				compare.ReaderPodSetFullyServing: "fullyServing/query=false/reading=true",
				compare.ReaderReadyReplicas:      "readyReplicas/status=false/reading=true",
				compare.ReaderPodSetPromotable:   "promotable/query=false/reading=true[containers-not-ready]",
			},
		},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := base()
			ctx.AvailabilityWindow = tc.window
			if tc.rotation != nil {
				ctx.Rotation = tc.rotation
			}
			ds := compare.Compare(tc.pods, tc.desired, tc.row, ctx)
			got := map[string]string{}
			for _, d := range ds {
				got[d.Reader] = d.Class
				seen[d.Reader] = true
				if d.Detail == "" {
					t.Fatalf("disagreement without detail: %+v", d)
				}
			}
			for reader, class := range tc.want {
				if got[reader] != class {
					t.Fatalf("reader %s: class %q, want %q; all:\n%s", reader, got[reader], class, classes(ds))
				}
			}
			for reader := range got {
				if _, ok := tc.want[reader]; !ok {
					t.Fatalf("unexpected disagreement from %s:\n%s", reader, classes(ds))
				}
			}
		})
	}
	for _, reader := range []string{
		compare.ReaderCountServingPods,
		compare.ReaderCountReadyPods,
		compare.ReaderPodSetFullyServing,
		compare.ReaderRecreateOfDarkRow,
		compare.ReaderPersistedServingPods,
		compare.ReaderPersistedAvailablePods,
		compare.ReaderPodReadyAndServing,
		compare.ReaderIsPodAvailable,
		compare.ReaderPodCount,
		compare.ReaderReadyReplicas,
		compare.ReaderLivePods,
		compare.ReaderRolledInstanceDark,
		compare.ReaderPodSetIsCapacity,
		compare.ReaderRunnerRestarted,
		compare.ReaderAdoption,
		compare.ReaderIsServing,
		compare.ReaderPodSetPromotable,
	} {
		if !seen[reader] {
			t.Errorf("no crafted set made %s disagree", reader)
		}
	}
}

// The three evidence readers and the writers' gate admission share one
// membership rule with the reading, so no pod set may split them. The
// sweep pairs every shape with every other, as leader and as worker, under
// every desired count and both graces.
func TestCompare_EvidenceReadersNeverDisagree(t *testing.T) {
	shapes := map[string][]mod{
		"serving":           nil,
		"gate absent":       {gateAbsent},
		"gate not folded":   {gateNotFolded},
		"in-place drained":  {gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1")},
		"surge drained":     {gateHeldBy(podreadiness.WriterUpdateSurgeDrain, "0-1")},
		"node lost":         {nodeLost},
		"unready past":      {unreadySince(now.Add(-2 * time.Minute))},
		"unready inside":    {unreadySince(now.Add(-time.Second))},
		"never ready":       {neverReady},
		"parked":            {parkedIn("CrashLoopBackOff")},
		"parked, deleting":  {parkedIn("CrashLoopBackOff"), terminating},
		"restarted":         {restartedAt(now.Add(-5 * time.Minute))},
		"exited":            {exitedAt(now.Add(-2 * time.Minute))},
		"failed":            {failed},
		"succeeded":         {succeeded},
		"terminating":       {terminating},
		"pending":           {pending},
		"no revision label": {noRevision},
	}
	runners := map[string]mod{"leader": asLeader, "worker": asWorker, "default": func(*corev1.Pod) {}}
	sameRule := map[string]bool{
		compare.ReaderPodSetServesNothing: true,
		compare.ReaderPodSetOutOfRotation: true,
		compare.ReaderUnavailableOnParked: true,
		compare.ReaderAllPodsRuntimeReady: true,
	}
	for grace := range map[time.Duration]struct{}{0: {}, time.Minute: {}} {
		ctx := base()
		ctx.UnreadyGrace = grace
		for aName, a := range shapes {
			for bName, b := range shapes {
				for rName, runner := range runners {
					set := pods(shape("a", a...), shape("b", append([]mod{runner}, b...)...))
					for _, desired := range []int32{0, 1, 2, 3} {
						for _, d := range compare.Compare(set, desired, nil, ctx) {
							if sameRule[d.Reader] {
								t.Fatalf("grace %s, %q beside %q as %s, desired %d: %s %s (%s)",
									grace, aName, bName, rName, desired, d.Reader, d.Class, d.Detail)
							}
						}
					}
				}
			}
		}
	}
}
