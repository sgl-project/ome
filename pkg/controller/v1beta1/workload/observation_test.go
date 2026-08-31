package workload_test

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

func TestPublicationObservationReusesOwnedRowsWithoutMutatingSource(t *testing.T) {
	operation := &workload.InstanceOperation{ID: "operation-4", Type: workload.InstanceOperationUpdate, Step: "Drain"}
	persisted := []workload.InstanceStatus{
		{
			Index: 4, Incarnation: 3, Phase: workload.InstancePhaseUpdating,
			RunningRevision: "revision-a", TargetRevision: "revision-b",
			PodCount: 8, ReadyPodCount: 7, ServingPodCount: 6, AvailablePodCount: 5,
			ScheduledPodCount: 8, Admitted: true, NodesOccupied: []string{"old-node"},
			Operation: operation,
		},
		{
			Index: 9, Incarnation: 2, Phase: workload.InstancePhaseRestarting,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, AvailablePodCount: 1,
			ScheduledPodCount: 1, Admitted: true, NodesOccupied: []string{"stale-node"},
		},
		{
			Index: 4, Incarnation: 4, Phase: workload.InstancePhaseMigrating,
			RunningRevision: "revision-c", NodesOccupied: []string{"other-old-node"},
		},
	}
	apiPersisted := v1beta1convert.InstanceStatusSliceFromWorkload(persisted)
	wantAPI := v1beta1convert.InstanceStatusSliceFromWorkload(
		v1beta1convert.InstanceStatusSliceToWorkload(apiPersisted),
	)
	owned := v1beta1convert.InstanceStatusSliceToWorkload(apiPersisted)
	ownedBacking := &owned[0]

	podA := observationPod("pod-a", "node-b", true, true, false)
	podB := observationPod("pod-b", "node-a", true, false, true)
	podC := observationPod("pod-c", "", false, false, false)
	byInstance := map[int32][]*corev1.Pod{
		4:  {podA, podB, podC},
		99: {observationPod("orphan-row-pod", "node-z", true, true, false)},
	}
	observation, err := workload.NewOwnedPublicationObservation(
		owned,
		workload.NewCachedSelectorPodObservation(nil, byInstance),
		map[string]struct{}{podA.Name: {}},
	)
	if err != nil {
		t.Fatalf("NewOwnedPublicationObservation: %v", err)
	}
	if !reflect.DeepEqual(observation.PersistedStatuses(), persisted) {
		t.Fatal("publication observation did not retain the durable rows")
	}

	got, err := observation.TakeInlineV1Statuses()
	if err != nil {
		t.Fatalf("TakeInlineV1Statuses: %v", err)
	}
	if &got[0] != ownedBacking {
		t.Fatal("publication materialization allocated another status row slice")
	}
	if len(got) != 3 {
		t.Fatalf("status rows = %d, want 3", len(got))
	}
	if got[0].Index != 4 || got[1].Index != 9 || got[2].Index != 4 {
		t.Fatalf("row order = [%d %d %d], want [4 9 4]", got[0].Index, got[1].Index, got[2].Index)
	}

	for _, row := range []workload.InstanceStatus{got[0], got[2]} {
		if row.PodCount != 3 || row.ReadyPodCount != 2 || row.ServingPodCount != 1 ||
			row.AvailablePodCount != 1 || row.ScheduledPodCount != 2 || row.Admitted {
			t.Fatalf("index 4 current fields = %+v", row)
		}
		if !reflect.DeepEqual(row.NodesOccupied, []string{"node-a", "node-b"}) {
			t.Fatalf("index 4 nodes = %v, want [node-a node-b]", row.NodesOccupied)
		}
	}
	if got[0].Operation == nil || got[0].Operation.ID != operation.ID || got[0].Phase != workload.InstancePhaseUpdating {
		t.Fatalf("lifecycle fields were not preserved: %+v", got[0])
	}
	if got[1].PodCount != 0 || got[1].ReadyPodCount != 0 || got[1].ScheduledPodCount != 0 ||
		got[1].Admitted || got[1].NodesOccupied != nil {
		t.Fatalf("empty index current fields = %+v", got[1])
	}
	if !reflect.DeepEqual(apiPersisted, wantAPI) {
		t.Fatalf("source CRD rows mutated\n got: %#v\nwant: %#v", apiPersisted, wantAPI)
	}
	current, availabilityObserved := observation.CurrentCounters(4)
	if !availabilityObserved || current.AvailablePodCount != 1 {
		t.Fatalf("publication availability = current %+v observed %t", current, availabilityObserved)
	}
	extra, _ := observation.CurrentCounters(99)
	if extra.PodCount != 1 || len(got) != len(persisted) {
		t.Fatalf("Pod-only index changed row membership: current %+v rows %d", extra, len(got))
	}
	if _, err := observation.TakeInlineV1Statuses(); err == nil {
		t.Fatal("publication observation was consumed twice")
	}
	if _, _, err := observation.TakeInlineV1Publication(nil, ""); err == nil {
		t.Fatal("publication counters accepted a consumed observation")
	}
}

func TestComponentObservationProvenanceAndEpochGuard(t *testing.T) {
	persisted := []workload.InstanceStatus{{Index: 0, AvailablePodCount: 7}}
	observation, err := workload.NewDecisionObservation(
		persisted,
		workload.NewAPIReaderSelectorPodObservation(nil, map[int32][]*corev1.Pod{}),
	)
	if err != nil {
		t.Fatalf("NewDecisionObservation: %v", err)
	}

	if observation.Epoch() != workload.ObservationEpochDecision ||
		observation.PodSource() != workload.PodObservationSourceAPIReader ||
		observation.PodScope() != workload.PodObservationScopeSelector {
		t.Fatalf("unexpected provenance: epoch=%v source=%v scope=%v",
			observation.Epoch(), observation.PodSource(), observation.PodScope())
	}
	current, availabilityObserved := observation.CurrentCounters(0)
	if availabilityObserved {
		t.Fatal("decision observation must not claim EndpointSlice availability")
	}
	if current.AvailablePodCount != 0 || observation.PersistedStatuses()[0].AvailablePodCount != 7 {
		t.Fatalf("persisted and current availability were conflated: current %+v persisted %+v",
			current, observation.PersistedStatuses()[0])
	}
	if _, err := observation.InlineV1Statuses(); err == nil {
		t.Fatal("decision observation materialized publication rows")
	}
	if _, _, err := observation.TakeInlineV1Publication(nil, ""); err == nil {
		t.Fatal("decision observation produced publication counters")
	}
	if !reflect.DeepEqual(observation.PersistedStatuses(), persisted) {
		t.Fatalf("persisted status projection changed")
	}
	if _, err := workload.NewDecisionObservation(persisted, workload.PodObservation{}); err == nil {
		t.Fatal("decision observation accepted unknown Pod provenance")
	}
}

func TestPublicationObservationRejectsUnsupportedProvenance(t *testing.T) {
	tests := []struct {
		name string
		pods workload.PodObservation
	}{
		{name: "unknown", pods: workload.PodObservation{}},
		{name: "API reader selector", pods: workload.NewAPIReaderSelectorPodObservation(nil, nil)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := workload.NewOwnedPublicationObservation(nil, test.pods, nil); err == nil {
				t.Fatal("publication observation accepted unsupported provenance")
			}
		})
	}
}

func TestPublicationObservationCopiesNestedDurableState(t *testing.T) {
	surgeIndex := int32(9)
	exitCode := int32(143)
	owned := []workload.InstanceStatus{{
		Index:         2,
		NodesOccupied: []string{"node-a"},
		Conditions:    []metav1.Condition{{Type: "Ready", Message: "original"}},
		Operation: &workload.InstanceOperation{
			ID: "operation-2", SurgeIndex: &surgeIndex, HintTargetNodes: []string{"node-b"},
		},
		LastFailure: &workload.InstanceTermination{PodName: "pod-2", ExitCode: &exitCode},
	}}
	observation, err := workload.NewOwnedPublicationObservation(
		owned,
		workload.NewCachedSelectorPodObservation(nil, nil),
		nil,
	)
	if err != nil {
		t.Fatalf("NewOwnedPublicationObservation: %v", err)
	}

	first, err := observation.InlineV1Statuses()
	if err != nil {
		t.Fatalf("InlineV1Statuses: %v", err)
	}
	first[0].NodesOccupied = []string{"mutated"}
	first[0].Conditions[0].Message = "mutated"
	first[0].Operation.HintTargetNodes[0] = "mutated"
	*first[0].Operation.SurgeIndex = 99
	*first[0].LastFailure.ExitCode = 1

	second, err := observation.InlineV1Statuses()
	if err != nil {
		t.Fatalf("InlineV1Statuses after output mutation: %v", err)
	}
	if second[0].Conditions[0].Message != "original" ||
		second[0].Operation.HintTargetNodes[0] != "node-b" ||
		*second[0].Operation.SurgeIndex != 9 ||
		*second[0].LastFailure.ExitCode != 143 {
		t.Fatalf("materialized rows alias durable state: %+v", second[0])
	}
	persisted := observation.PersistedStatuses()
	persisted[0].NodesOccupied[0] = "mutated-again"
	persisted[0].Conditions[0].Message = "mutated-again"
	again := observation.PersistedStatuses()[0]
	if again.NodesOccupied[0] != "node-a" || again.Conditions[0].Message != "original" {
		t.Fatal("PersistedStatuses returned aliased nested state")
	}
}

func TestPublicationObservationPreservesNilStatusSlice(t *testing.T) {
	observation, err := workload.NewOwnedPublicationObservation(
		nil,
		workload.NewCachedSelectorPodObservation(nil, nil),
		nil,
	)
	if err != nil {
		t.Fatalf("NewOwnedPublicationObservation: %v", err)
	}
	got, err := observation.TakeInlineV1Statuses()
	if err != nil {
		t.Fatalf("InlineV1Statuses: %v", err)
	}
	if got != nil {
		t.Fatalf("nil persisted rows materialized as %#v", got)
	}
}

var benchmarkObservationRows []workload.InstanceStatus
var benchmarkComponentCounters workload.ComponentCounters

func BenchmarkPublicationObservationMaterialization(b *testing.B) {
	for _, podsPerInstance := range []int{1, 8} {
		persisted := make([]workload.InstanceStatus, 2000)
		byInstance := make(map[int32][]*corev1.Pod, len(persisted))
		available := make(map[string]struct{}, len(persisted)*podsPerInstance)
		desired := make(map[int32]int32, len(persisted))
		for index := range persisted {
			persisted[index] = workload.InstanceStatus{
				Index: int32(index), Incarnation: 1, Phase: workload.InstancePhaseReady,
				RunningRevision: "revision-a",
			}
			desired[int32(index)] = int32(podsPerInstance)
			for ordinal := 0; ordinal < podsPerInstance; ordinal++ {
				name := fmt.Sprintf("pod-%04d-%02d", index, ordinal)
				pod := observationPod(name, fmt.Sprintf("node-%02d", ordinal), true, true, false)
				byInstance[int32(index)] = append(byInstance[int32(index)], pod)
				available[name] = struct{}{}
			}
		}
		pods := workload.NewCachedSelectorPodObservation(nil, byInstance)

		b.Run(fmt.Sprintf("2000x%d/owned", podsPerInstance), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				owned := append([]workload.InstanceStatus(nil), persisted...)
				observation, err := workload.NewOwnedPublicationObservation(owned, pods, available)
				if err != nil {
					b.Fatal(err)
				}
				rows, err := observation.TakeInlineV1Statuses()
				if err != nil {
					b.Fatal(err)
				}
				benchmarkObservationRows = rows
			}
		})

		b.Run(fmt.Sprintf("2000x%d/owned-with-counters", podsPerInstance), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				owned := append([]workload.InstanceStatus(nil), persisted...)
				observation, err := workload.NewOwnedPublicationObservation(owned, pods, available)
				if err != nil {
					b.Fatal(err)
				}
				rows, counters, err := observation.TakeInlineV1Publication(desired, "revision-a")
				if err != nil {
					b.Fatal(err)
				}
				benchmarkObservationRows = rows
				benchmarkComponentCounters = counters
			}
		})

		b.Run(fmt.Sprintf("2000x%d/legacy-owned-with-counters", podsPerInstance), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				owned := append([]workload.InstanceStatus(nil), persisted...)
				observation, err := workload.NewOwnedPublicationObservation(owned, pods, available)
				if err != nil {
					b.Fatal(err)
				}
				rows, err := observation.TakeInlineV1Statuses()
				if err != nil {
					b.Fatal(err)
				}
				benchmarkObservationRows = rows
				benchmarkComponentCounters = legacyObservationComponentCounters(rows, desired, "revision-a")
			}
		})

		b.Run(fmt.Sprintf("2000x%d/copy", podsPerInstance), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				owned := append([]workload.InstanceStatus(nil), persisted...)
				observation, err := workload.NewOwnedPublicationObservation(owned, pods, available)
				if err != nil {
					b.Fatal(err)
				}
				rows, err := observation.InlineV1Statuses()
				if err != nil {
					b.Fatal(err)
				}
				benchmarkObservationRows = rows
			}
		})
	}
}

func legacyObservationComponentCounters(instances []workload.InstanceStatus, desiredByIdx map[int32]int32, targetRevision string) workload.ComponentCounters {
	counters := workload.ComponentCounters{Replicas: int32(len(instances))}
	target := query.RevisionFromName(targetRevision)
	for _, instance := range instances {
		desired := workload.DesiredFor(desiredByIdx, instance.Index, instance.PodCount)
		if workload.InstanceMeetsThreshold(instance.PodCount, instance.ReadyPodCount, desired) {
			counters.ReadyReplicas++
		}
		if workload.InstanceMeetsThreshold(instance.PodCount, instance.ServingPodCount, desired) {
			counters.ServingReplicas++
		}
		if workload.InstanceMeetsThreshold(instance.PodCount, instance.AvailablePodCount, desired) {
			counters.AvailableReplicas++
		}
		if targetRevision != "" && query.RevisionFromName(instance.RunningRevision).Same(target) {
			counters.UpdatedReplicas++
			if workload.InstanceMeetsThreshold(instance.PodCount, instance.ReadyPodCount, desired) {
				counters.UpdatedReadyReplicas++
			}
		}
	}
	return counters
}

func observationPod(name, node string, ready, serving, gated bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.PodSpec{NodeName: node},
	}
	if ready {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type: corev1.ContainersReady, Status: corev1.ConditionTrue,
		})
	}
	if serving {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type: podreadiness.ConditionType, Status: corev1.ConditionTrue,
		})
	}
	if gated {
		pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "example-gate"}}
	}
	return pod
}
