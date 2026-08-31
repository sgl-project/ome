package inferencereplica

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

func TestPublicationObservationMatchesInlineV1Projection(t *testing.T) {
	ir := baselineIR("model-engine", "default", 10)
	ready := podForIR(ir, 0, "default", 0, true, true)
	ready.Spec.NodeName = "node-b"
	notReady := podForIR(ir, 1, "default", 0, false, false)
	notReady.Spec.NodeName = "node-a"
	gated := podForIR(ir, 2, "default", 0, false, false)
	gated.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "example-gate"}}
	terminating := podForIR(ir, 3, "default", 0, true, true)
	terminating.Spec.NodeName = "node-c"
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	foreignOwner := podForIR(ir, 4, "default", 0, true, true)
	foreignOwner.OwnerReferences = []metav1.OwnerReference{{UID: "different-owner", Controller: boolPointer(true)}}
	malformed := podForIR(ir, 5, "default", 0, true, true)
	malformed.Labels[query.LabelInstanceIdx] = "invalid"
	liveOnly := podForIR(ir, 99, "default", 0, true, true)

	gangPods := make([]*corev1.Pod, 0, 8)
	for i := 0; i < 8; i++ {
		pod := podForIR(ir, 7, "worker", int32(i), true, true)
		if i%2 == 0 {
			pod.Spec.NodeName = "node-b"
		} else {
			pod.Spec.NodeName = "node-a"
		}
		gangPods = append(gangPods, pod)
	}

	tests := []struct {
		name      string
		statuses  []v1beta1.OMENativeInstanceStatus
		pods      []*corev1.Pod
		available map[string]struct{}
		desired   map[int32]int32
	}{
		{
			name: "stale status trails a ready singleton",
			statuses: []v1beta1.OMENativeInstanceStatus{{
				Index: 0, Phase: v1beta1.OMENativeInstanceUpdating,
			}},
			pods:      []*corev1.Pod{ready},
			available: map[string]struct{}{ready.Name: {}},
			desired:   map[int32]int32{0: 1},
		},
		{
			name:     "scale-down row falls back to observed pod count",
			statuses: []v1beta1.OMENativeInstanceStatus{fullyPopulatedInstanceStatus(0)},
			pods:     []*corev1.Pod{ready},
			available: map[string]struct{}{
				ready.Name: {},
			},
			desired: map[int32]int32{9: 1},
		},
		{
			name: "stale status leads a non-ready singleton",
			statuses: []v1beta1.OMENativeInstanceStatus{{
				Index: 1, PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1,
				AvailablePodCount: 1, ScheduledPodCount: 1, Admitted: true,
				NodesOccupied: []string{"old-node"},
			}},
			pods:    []*corev1.Pod{notReady},
			desired: map[int32]int32{1: 1},
		},
		{
			name:     "zero-pod create window",
			statuses: []v1beta1.OMENativeInstanceStatus{fullyPopulatedInstanceStatus(6)},
			desired:  map[int32]int32{6: 1},
		},
		{
			name: "admission-gated pod",
			statuses: []v1beta1.OMENativeInstanceStatus{{
				Index: 2, Admitted: true,
			}},
			pods:    []*corev1.Pod{gated},
			desired: map[int32]int32{2: 1},
		},
		{
			name: "eight-pod gang with duplicate nodes",
			statuses: []v1beta1.OMENativeInstanceStatus{
				fullyPopulatedInstanceStatus(7),
				fullyPopulatedInstanceStatus(7),
			},
			pods: gangPods,
			available: map[string]struct{}{
				gangPods[0].Name: {}, gangPods[1].Name: {}, gangPods[2].Name: {}, gangPods[3].Name: {},
			},
			desired: map[int32]int32{7: 8},
		},
		{
			name: "selector membership includes terminating and foreign-owner pods",
			statuses: []v1beta1.OMENativeInstanceStatus{
				{Index: 3},
				{Index: 4},
			},
			pods:      []*corev1.Pod{terminating, foreignOwner},
			available: map[string]struct{}{foreignOwner.Name: {}},
			desired:   map[int32]int32{3: 1, 4: 1},
		},
		{
			name: "malformed and rowless pod indexes do not create rows",
			statuses: []v1beta1.OMENativeInstanceStatus{
				fullyPopulatedInstanceStatus(5),
				fullyPopulatedInstanceStatus(8),
			},
			pods:    []*corev1.Pod{malformed, liveOnly},
			desired: map[int32]int32{5: 1, 8: 1},
		},
		{
			name:     "nil status remains nil",
			statuses: nil,
			pods:     []*corev1.Pod{liveOnly},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := cloneAPIInstanceStatuses(test.statuses)
			byIndex := query.BucketPodsByInstanceIdx(test.pods)
			want := legacyInlineV1Projection(test.statuses, byIndex, test.available)
			wantCounters := legacyInlineV1ComponentCounters(want, test.desired, "rev-running")

			observation, err := workload.NewOwnedPublicationObservation(
				v1beta1convert.InstanceStatusSliceToWorkload(test.statuses),
				workload.NewCachedSelectorPodObservation(test.pods, nil),
				test.available,
			)
			if err != nil {
				t.Fatalf("NewOwnedPublicationObservation: %v", err)
			}
			materialized, gotCounters, err := observation.TakeInlineV1Publication(test.desired, "rev-running")
			if err != nil {
				t.Fatalf("TakeInlineV1Publication: %v", err)
			}
			got := v1beta1convert.InstanceStatusSliceFromWorkload(materialized)

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("publication projection mismatch\n got: %#v\nwant: %#v", got, want)
			}
			if !reflect.DeepEqual(test.statuses, original) {
				t.Fatalf("input status mutated\n got: %#v\nwant: %#v", test.statuses, original)
			}
			if gotCounters != wantCounters {
				t.Fatalf("publication counters mismatch\n got: %+v\nwant: %+v", gotCounters, wantCounters)
			}
		})
	}
}

func TestPublicationObservationMatchesInlineV1ProjectionAtScale(t *testing.T) {
	tests := []struct {
		name                   string
		instances              int
		podsPerInstance        int
		desiredPodsPerInstance int
		readyPodsPerInstance   int
		assertReadyReplicas    bool
		wantReadyReplicas      int32
	}{
		{name: "empty", instances: 0, podsPerInstance: 1},
		{name: "singleton", instances: 1, podsPerInstance: 1},
		{
			name: "eight-pod gang with one surge pod", instances: 1, podsPerInstance: 9,
			desiredPodsPerInstance: 8, readyPodsPerInstance: 8,
			assertReadyReplicas: true, wantReadyReplicas: 1,
		},
		{name: "one hundred single-pod instances", instances: 100, podsPerInstance: 1},
		{name: "two thousand single-pod instances", instances: 2000, podsPerInstance: 1},
		{name: "one hundred eight-pod gangs", instances: 100, podsPerInstance: 8},
		{name: "two thousand eight-pod gangs", instances: 2000, podsPerInstance: 8},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desiredPods := test.desiredPodsPerInstance
			if desiredPods == 0 {
				desiredPods = test.podsPerInstance
			}
			ir := baselineIR("model-engine", "default", int32(test.instances))
			statuses := make([]v1beta1.OMENativeInstanceStatus, test.instances)
			pods := make([]*corev1.Pod, 0, test.instances*test.podsPerInstance+2)
			available := make(map[string]struct{})
			desired := make(map[int32]int32, test.instances)
			for index := 0; index < test.instances; index++ {
				statuses[index] = fullyPopulatedInstanceStatus(int32(index))
				desired[int32(index)] = int32(desiredPods)
				for ordinal := 0; ordinal < test.podsPerInstance; ordinal++ {
					ready := (index+ordinal)%3 != 0
					if test.readyPodsPerInstance > 0 {
						ready = ordinal < test.readyPodsPerInstance
					}
					serving := ready && (index+ordinal)%2 == 0
					runner := "default"
					if test.podsPerInstance > 1 {
						runner = "worker"
					}
					pod := podForIR(ir, int32(index), runner, int32(ordinal), ready, serving)
					if ordinal%5 != 0 {
						pod.Spec.NodeName = fmt.Sprintf("node-%02d", ordinal%4)
					}
					if (index+ordinal)%17 == 0 {
						pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "example-gate"}}
					}
					if ready && (index+ordinal)%4 != 0 {
						available[pod.Name] = struct{}{}
					}
					pods = append(pods, pod)
				}
			}
			rowless := podForIR(ir, int32(test.instances+10), "default", 0, true, true)
			malformed := podForIR(ir, int32(test.instances+11), "default", 0, true, true)
			malformed.Labels[query.LabelInstanceIdx] = "invalid"
			pods = append(pods, rowless, malformed)

			byIndex := query.BucketPodsByInstanceIdx(pods)
			want := legacyInlineV1Projection(statuses, byIndex, available)
			wantCounters := legacyInlineV1ComponentCounters(want, desired, "rev-running")
			observation, err := workload.NewOwnedPublicationObservation(
				v1beta1convert.InstanceStatusSliceToWorkload(statuses),
				workload.NewCachedSelectorPodObservation(pods, nil),
				available,
			)
			if err != nil {
				t.Fatalf("NewOwnedPublicationObservation: %v", err)
			}
			materialized, gotCounters, err := observation.TakeInlineV1Publication(desired, "rev-running")
			if err != nil {
				t.Fatalf("TakeInlineV1Publication: %v", err)
			}
			got := v1beta1convert.InstanceStatusSliceFromWorkload(materialized)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%d x %d projection differed", test.instances, test.podsPerInstance)
			}
			if gotCounters != wantCounters {
				t.Fatalf("%d x %d counters differed: got %+v want %+v", test.instances, test.podsPerInstance, gotCounters, wantCounters)
			}
			if test.assertReadyReplicas && gotCounters.ReadyReplicas != test.wantReadyReplicas {
				t.Fatalf("ReadyReplicas = %d, want %d", gotCounters.ReadyReplicas, test.wantReadyReplicas)
			}
		})
	}
}

func legacyInlineV1Projection(instances []v1beta1.OMENativeInstanceStatus, byIndex map[int32][]*corev1.Pod, available map[string]struct{}) []v1beta1.OMENativeInstanceStatus {
	out := cloneAPIInstanceStatuses(instances)
	for i := range out {
		pods := byIndex[out[i].Index]
		out[i].PodCount = int32(len(pods))
		out[i].ReadyPodCount = workload.CountReadyPods(pods)
		out[i].ServingPodCount = workload.CountServingPods(pods)
		out[i].AvailablePodCount = workload.CountAvailablePods(pods, available)
		out[i].ScheduledPodCount = workload.CountScheduledPods(pods)
		out[i].Admitted = legacyPodsAdmitted(pods)
		out[i].NodesOccupied = workload.UniqueNodes(pods)
	}
	return out
}

func legacyInlineV1ComponentCounters(instances []v1beta1.OMENativeInstanceStatus, desiredByIdx map[int32]int32, targetRevision string) workload.ComponentCounters {
	counters := workload.ComponentCounters{Replicas: int32(len(instances))}
	target := query.RevisionFromName(targetRevision)
	for _, instance := range instances {
		desired := legacyDesiredPodCount(desiredByIdx, instance.Index, instance.PodCount)
		if legacyInstanceMeetsThreshold(instance.PodCount, instance.ReadyPodCount, desired) {
			counters.ReadyReplicas++
		}
		if legacyInstanceMeetsThreshold(instance.PodCount, instance.ServingPodCount, desired) {
			counters.ServingReplicas++
		}
		if legacyInstanceMeetsThreshold(instance.PodCount, instance.AvailablePodCount, desired) {
			counters.AvailableReplicas++
		}
		if targetRevision != "" && query.RevisionFromName(instance.RunningRevision).Same(target) {
			counters.UpdatedReplicas++
			if legacyInstanceMeetsThreshold(instance.PodCount, instance.ReadyPodCount, desired) {
				counters.UpdatedReadyReplicas++
			}
		}
	}
	return counters
}

func legacyDesiredPodCount(desiredByIdx map[int32]int32, index, observed int32) int32 {
	if desiredByIdx != nil {
		if desired, ok := desiredByIdx[index]; ok {
			return desired
		}
	}
	return observed
}

func legacyInstanceMeetsThreshold(observed, satisfying, desired int32) bool {
	if observed == 0 {
		return false
	}
	if desired <= 0 {
		return satisfying == observed
	}
	return satisfying >= desired
}

func cloneAPIInstanceStatuses(in []v1beta1.OMENativeInstanceStatus) []v1beta1.OMENativeInstanceStatus {
	return v1beta1convert.InstanceStatusSliceFromWorkload(v1beta1convert.InstanceStatusSliceToWorkload(in))
}

func legacyPodsAdmitted(pods []*corev1.Pod) bool {
	if len(pods) == 0 {
		return false
	}
	for _, pod := range pods {
		if workload.PodAdmissionGated(pod) {
			return false
		}
	}
	return true
}

func boolPointer(value bool) *bool { return &value }
