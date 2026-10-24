package replay

import (
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading/compare"
	types "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// readablePod is a pod every reader agrees serves: Running, on a revision,
// the default runner, ContainersReady, PodReady and the serving gate True
// since the trace start.
func readablePod(name string) *corev1.Pod {
	pod := servingPod(name)
	pod.Labels = map[string]string{query.LabelRevisionHash: "rev-a", query.LabelRunner: types.RunnerDefault}
	pod.Status.Phase = corev1.PodRunning
	return pod
}

// gateNotFoldedPod is the pass that wrote the gate: the gate reads True but
// the kubelet has not folded it into PodReady yet.
func gateNotFoldedPod(name string) *corev1.Pod {
	pod := readablePod(name)
	setPodCondition(pod, corev1.PodReady, corev1.ConditionFalse, traceStart.Add(time.Second))
	return pod
}

// terminalPod is a Failed pod whose conditions were never updated after
// the phase moved, so they still read healthy.
func terminalPod(name string) *corev1.Pod {
	pod := readablePod(name)
	pod.Status.Phase = corev1.PodFailed
	return pod
}

// gateAbsentPod never had its serving condition written while the kubelet
// reports PodReady True.
func gateAbsentPod(name string) *corev1.Pod {
	pod := readablePod(name)
	kept := pod.Status.Conditions[:0]
	for _, c := range pod.Status.Conditions {
		if c.Type != query.ServingConditionType {
			kept = append(kept, c)
		}
	}
	pod.Status.Conditions = kept
	return pod
}

func readingClasses(findings []ReadingFinding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Class)
	}
	return out
}

// TestCompareReadings_CraftedPodsDisagreePerReader pins, per crafted pod,
// which reader disagrees with the reading and under which class, and that
// a pod every reader agrees on makes no finding.
func TestCompareReadings_CraftedPodsDisagreePerReader(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want []string
	}{
		{
			name: "a serving pod in rotation",
			pod:  readablePod("p-0"),
		},
		{
			name: "the gate written this pass, PodReady not folded yet",
			pod:  gateNotFoldedPod("p-0"),
			want: []string{"fullyServing/query=true/reading=false", "readyReplicas/status=true/reading=false[not-serving]",
				"gateWritten/podreadiness=true/reading=false[not-folded]"},
		},
		{
			name: "a terminal pod whose conditions still read healthy",
			pod:  terminalPod("p-0"),
			want: []string{"servingCount/status>reading", "readyCount/status>reading", "podCount/status>reading[terminal]",
				"readyReplicas/status=true/reading=false[dead-pod]", "podReadyAndServing/coordination=false/podreadiness=true",
				"podAvailable/podreadiness=true/reading=false[not-live]", "gateWritten/podreadiness=true/reading=false[not-live]"},
		},
		{
			name: "the gate condition absent, PodReady true",
			pod:  gateAbsentPod("p-0"),
			want: []string{"readyReplicas/status=true/reading=false[not-serving]", "promotable/query=true/reading=false[not-serving]",
				"podReadyAndServing/coordination=true/podreadiness=false", "podAvailable/podreadiness=true/reading=false[not-serving]"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := TickView{
				Now:    traceStart.Add(time.Minute),
				Pods:   map[int32][]*corev1.Pod{0: {tc.pod}},
				Routed: map[string]string{"p-0": "svc-rev-a"},
				Spec:   TickSpec{Replicas: 1, PodsPerInstance: 1},
			}
			got := CompareReadings(view)
			if classes := readingClasses(got); !reflect.DeepEqual(classes, tc.want) {
				t.Fatalf("classes %v, want %v\n%+v", classes, tc.want, got)
			}
			for _, f := range got {
				if f.Index != 0 || f.Reader == "" || !strings.HasPrefix(f.Detail, "index 0: ") {
					t.Fatalf("finding names no index, reader or detail: %+v", f)
				}
			}
		})
	}
}

// TestCompareReadings_HandsTheRowToTheComparison proves the readers of
// the persisted serving and available counts are compared against the
// row the view carries for the Instance, and that an Instance with a row
// and no pods is left out.
func TestCompareReadings_HandsTheRowToTheComparison(t *testing.T) {
	failedRow := func(index, serving int32) types.InstanceStatus {
		return types.InstanceStatus{Index: index, Phase: types.InstancePhaseFailed, PodCount: 1, ServingPodCount: serving}
	}
	view := TickView{
		Now: traceStart.Add(time.Minute),
		Rows: []types.InstanceStatus{
			failedRow(0, 0),
			{Index: 1, Phase: types.InstancePhaseReady, PodCount: 1, ServingPodCount: 1, AvailablePodCount: 1},
			failedRow(2, 0),
		},
		Pods: map[int32][]*corev1.Pod{
			0: {readablePod("p-0")},
			1: {readablePod("p-1")},
		},
		Routed: map[string]string{"p-0": "svc-rev-a", "p-1": "svc-rev-a"},
		Spec:   TickSpec{Replicas: 3, PodsPerInstance: 1},
	}
	got := CompareReadings(view)
	want := []string{"recreateOfDarkRow/row=true/reading=false", "persistedServingCount/row<reading", "persistedAvailableCount/row<reading"}
	if classes := readingClasses(got); !reflect.DeepEqual(classes, want) {
		t.Fatalf("classes %v, want %v\n%+v", classes, want, got)
	}
	for _, f := range got {
		if f.Index != 0 {
			t.Fatalf("a finding on an Instance whose row and pods agree, or which has no pods: %+v", f)
		}
	}
}

// TestCompareReadings_ListsInstancesInOrder holds the findings to Instance
// order, whatever order the pod map iterates in, with the reader named.
func TestCompareReadings_ListsInstancesInOrder(t *testing.T) {
	view := TickView{
		Now: traceStart.Add(time.Minute),
		Pods: map[int32][]*corev1.Pod{
			7: {terminalPod("p-7")},
			2: {gateNotFoldedPod("p-2")},
			4: {readablePod("p-4")},
		},
		Spec: TickSpec{Replicas: 3, PodsPerInstance: 1},
	}
	got := CompareReadings(view)
	var indexes []int32
	var readers []string
	for _, f := range got {
		indexes = append(indexes, f.Index)
		readers = append(readers, f.Reader)
	}
	if want := []int32{2, 2, 2, 7, 7, 7, 7, 7, 7, 7}; !reflect.DeepEqual(indexes, want) {
		t.Fatalf("indexes %v, want %v\n%+v", indexes, want, got)
	}
	if want := []string{compare.ReaderPodSetFullyServing, compare.ReaderReadyReplicas, compare.ReaderIsServing,
		compare.ReaderCountServingPods, compare.ReaderCountReadyPods, compare.ReaderPodCount, compare.ReaderReadyReplicas,
		compare.ReaderPodReadyAndServing, compare.ReaderIsPodAvailable, compare.ReaderIsServing}; !reflect.DeepEqual(readers, want) {
		t.Fatalf("readers %v, want %v", readers, want)
	}
	if got := CompareReadings(TickView{}); got != nil {
		t.Fatalf("an empty view made findings: %+v", got)
	}
}

// TestRotationOf keeps a pod in the rotation set only when a routed
// Service publishes it.
func TestRotationOf(t *testing.T) {
	got := rotationOf(map[string]string{"a": "svc-rev-a", "b": ""})
	if _, ok := got["a"]; !ok || len(got) != 1 {
		t.Fatalf("rotation %v, want {a}", got)
	}
}
