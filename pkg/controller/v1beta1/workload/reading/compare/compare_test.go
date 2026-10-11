package compare_test

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading/compare"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func readyRow(serving int32) *types.InstanceStatus {
	return &types.InstanceStatus{Phase: types.InstancePhaseReady, PodCount: serving, ServingPodCount: serving}
}

func failedRow(serving int32) *types.InstanceStatus {
	return &types.InstanceStatus{Phase: types.InstancePhaseFailed, PodCount: 1, ServingPodCount: serving}
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
	if ds := compare.Compare(set, 2, readyRow(2), ctx); len(ds) != 0 {
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
// class each disagreement must carry. The three evidence readers
// implement the same membership rule and must never appear.
func TestCompare_CraftedSetsDisagreePerReader(t *testing.T) {
	cases := []struct {
		name    string
		pods    []*corev1.Pod
		desired int32
		row     *types.InstanceStatus
		want    map[string]string // reader -> class
	}{
		{
			name:    "gate written this pass, PodReady not folded yet",
			pods:    pods(shape(podName, gateNotFolded)),
			desired: 1,
			row:     readyRow(1),
			want: map[string]string{
				compare.ReaderPodSetFullyServing:   "fullyServing/query=true/reading=false",
				compare.ReaderPersistedServingPods: "persistedServingCount/row>reading",
			},
		},
		{
			name:    "terminal pod whose conditions still read healthy",
			pods:    pods(shape(podName, failed)),
			desired: 1,
			row:     readyRow(1),
			want: map[string]string{
				compare.ReaderCountServingPods:     "servingCount/status>reading",
				compare.ReaderPersistedServingPods: "persistedServingCount/row>reading",
				compare.ReaderPodReadyAndServing:   "podReadyAndServing/coordination=false/podreadiness=true",
			},
		},
		{
			name:    "gate condition absent, PodReady true",
			pods:    pods(shape(podName, gateAbsent)),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderPodReadyAndServing: "podReadyAndServing/coordination=true/podreadiness=false",
			},
		},
		{
			name:    "Failed row whose persisted serving count trails the parked pod by a pass",
			pods:    pods(shape(podName, parkedIn("CrashLoopBackOff"))),
			desired: 1,
			row:     failedRow(1),
			want: map[string]string{
				compare.ReaderRecreateOfDarkRow:    "recreateOfDarkRow/row=false/reading=true",
				compare.ReaderPersistedServingPods: "persistedServingCount/row>reading",
			},
		},
		{
			name:    "pod came back a pass after the row persisted zero",
			pods:    pods(shape(podName)),
			desired: 1,
			row:     readyRow(0),
			want: map[string]string{
				compare.ReaderPersistedServingPods: "persistedServingCount/row<reading",
			},
		},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := compare.Compare(tc.pods, tc.desired, tc.row, base())
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
		compare.ReaderPodSetFullyServing,
		compare.ReaderRecreateOfDarkRow,
		compare.ReaderPersistedServingPods,
		compare.ReaderPodReadyAndServing,
	} {
		if !seen[reader] {
			t.Errorf("no crafted set made %s disagree", reader)
		}
	}
}

// The three evidence readers and the reading share one membership rule, so
// no pod set may split them. The sweep pairs every shape with every other,
// as leader and as worker, under every desired count and both graces.
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
		"failed":            {failed},
		"succeeded":         {succeeded},
		"terminating":       {terminating},
		"pending":           {pending},
		"no revision label": {noRevision},
	}
	runners := map[string]mod{"leader": asLeader, "worker": asWorker, "default": func(*corev1.Pod) {}}
	evidenceReaders := map[string]bool{
		compare.ReaderPodSetServesNothing: true,
		compare.ReaderPodSetOutOfRotation: true,
		compare.ReaderUnavailableOnParked: true,
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
							if evidenceReaders[d.Reader] {
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
