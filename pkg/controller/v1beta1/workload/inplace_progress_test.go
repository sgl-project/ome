package workload_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	workloadops "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/ops"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/revision"
)

type staleInPlacePodClient struct {
	client.Client
	stale bool
	pods  []*corev1.Pod
}

func (c *staleInPlacePodClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if !c.stale {
		return c.Client.List(ctx, list, opts...)
	}
	pods, ok := list.(*corev1.PodList)
	if !ok {
		return c.Client.List(ctx, list, opts...)
	}
	pods.Items = make([]corev1.Pod, 0, len(c.pods))
	for _, pod := range c.pods {
		pods.Items = append(pods.Items, *pod.DeepCopy())
	}
	return nil
}

// TestReconcile_InPlaceMetadataUpdateProgressesWithinMaxUnavailable verifies
// that a metadata-only rollout holds its MaxUnavailable slot until a fresh
// observation promotes the current Instance, then advances to the next index.
// The runtime may report a different repository alias for the unchanged image.
func TestReconcile_InPlaceMetadataUpdateProgressesWithinMaxUnavailable(t *testing.T) {
	scheme := makeScheme(t)
	spec := &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "test:v1"}}}
	runningName := "llama-70b-engine-oldhash"
	targetName := "llama-70b-engine-newhash"
	runningRaw, err := json.Marshal(revision.DataPayload{
		PodSpec: spec,
		PodMeta: &metav1.ObjectMeta{Annotations: map[string]string{"release": "one"}},
	})
	if err != nil {
		t.Fatalf("marshal running revision: %v", err)
	}
	targetRaw, err := json.Marshal(revision.DataPayload{
		PodSpec: spec,
		PodMeta: &metav1.ObjectMeta{Annotations: map[string]string{"release": "two"}},
	})
	if err != nil {
		t.Fatalf("marshal target revision: %v", err)
	}
	running := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: runningName, Namespace: "prod"}}
	running.Data.Raw = runningRaw
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetName, Namespace: "prod"}}
	target.Data.Raw = targetRaw

	labelsFor := func(index string) map[string]string {
		return map[string]string{
			constants.InferenceServicePodLabelKey: "llama-70b",
			constants.OMEComponentLabel:           string(workload.ComponentEngine),
			query.LabelManagedBy:                  query.ManagedByOMENative,
			query.LabelInstanceIdx:                index,
			query.LabelInstanceIncarnation:        "1",
			query.LabelRunner:                     "default",
			query.LabelPodOrdinal:                 "0",
			query.LabelRevisionHash:               "oldhash",
		}
	}
	readyConditions := []corev1.PodCondition{
		{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
		{Type: query.ServingConditionType, Status: corev1.ConditionTrue},
	}
	podFor := func(index string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "llama-70b-engine-" + index + "-default-0",
				Namespace:   "prod",
				Labels:      labelsFor(index),
				Annotations: map[string]string{"release": "one"},
			},
			Spec: *spec.DeepCopy(),
			Status: corev1.PodStatus{
				Conditions:        append([]corev1.PodCondition(nil), readyConditions...),
				ContainerStatuses: []corev1.ContainerStatus{{Name: "main", Image: "mirror.example.com/test:v1"}},
			},
		}
	}
	pod0 := podFor("0")
	pod1 := podFor("1")
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b-engine", Namespace: "prod"},
		Status: v1beta1.InferenceReplicaStatus{InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: runningName},
			{Index: 1, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: runningName},
		}},
	}
	live := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		WithObjects(running, target, ir, pod0, pod1).
		Build()
	cached := &staleInPlacePodClient{
		Client: live,
		stale:  true,
		pods:   []*corev1.Pod{pod0.DeepCopy(), pod1.DeepCopy()},
	}
	deps := workload.Deps{Client: cached, APIReader: live}
	markNotReady := false
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  2,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		InstanceReadyTimeout: 30 * time.Minute,
		UpdateStrategy: workload.UpdateStrategy{
			Type: workload.UpdateStrategyInPlaceIfPossible,
			InPlaceUpdateStrategy: &workload.InPlaceUpdateStrategy{
				MarkNotReadyDuringLifecycle: &markNotReady,
			},
			RollingUpdate: &workload.RollingUpdate{MaxUnavailable: intOrStringInt(1)},
		},
	}

	inputWithStatuses := func(statuses []workload.InstanceStatus) workload.ReconcileInput {
		input := minimalInput(t)
		input.DesiredSpec.Replicas = 2
		input.DesiredSpec.PodSpec = spec
		input.ObservedState.InstanceStatuses = append([]workload.InstanceStatus(nil), statuses...)
		isvc := input.OwnerObject.(*v1beta1.InferenceService)
		input.MutateInstance = roundTripMutateInstance(live, isvc, workload.ComponentEngine)
		return input
	}
	staleStatuses := make([]workload.InstanceStatus, 0, len(ir.Status.InstanceStatuses))
	for _, status := range ir.Status.InstanceStatuses {
		staleStatuses = append(staleStatuses, v1beta1convert.InstanceStatusToWorkload(status))
	}
	staleInput := func() workload.ReconcileInput {
		return inputWithStatuses(staleStatuses)
	}
	inputFromAPI := func() workload.ReconcileInput {
		current := &v1beta1.InferenceReplica{}
		if err := live.Get(context.Background(), client.ObjectKeyFromObject(ir), current); err != nil {
			t.Fatalf("get InferenceReplica: %v", err)
		}
		statuses := make([]workload.InstanceStatus, 0, len(current.Status.InstanceStatuses))
		for _, status := range current.Status.InstanceStatuses {
			statuses = append(statuses, v1beta1convert.InstanceStatusToWorkload(status))
		}
		return inputWithStatuses(statuses)
	}
	assertStatus := func(index int32, phase v1beta1.OMENativeInstancePhase, runningRevision string) {
		t.Helper()
		status := instanceStatusByIndex(live, inputFromAPI().OwnerObject.(*v1beta1.InferenceService), v1beta1.EngineComponent, index)
		if status == nil {
			t.Fatalf("instance %d status not found", index)
		}
		if status.Phase != phase || status.RunningRevision != runningRevision {
			t.Errorf("instance %d: phase=%q runningRevision=%q, want phase=%q runningRevision=%q",
				index, status.Phase, status.RunningRevision, phase, runningRevision)
		}
	}
	assertPodRevision := func(pod *corev1.Pod, annotation, hash string) {
		t.Helper()
		got := &corev1.Pod{}
		if err := live.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
			t.Fatalf("get pod %s: %v", pod.Name, err)
		}
		if got.Annotations["release"] != annotation || got.Labels[query.LabelRevisionHash] != hash {
			t.Errorf("pod %s: release=%q hash=%q, want release=%q hash=%q",
				pod.Name, got.Annotations["release"], got.Labels[query.LabelRevisionHash], annotation, hash)
		}
	}

	result, err := workload.Reconcile(context.Background(), deps, staleInput(), plan, target)
	if err != nil {
		t.Fatalf("first Reconcile: %v", err)
	}
	if result.RequeueAfter != workloadops.UpdateRequeueInterval {
		t.Errorf("first Reconcile requeue: got %v want %v", result.RequeueAfter, workloadops.UpdateRequeueInterval)
	}
	assertStatus(0, v1beta1.OMENativeInstanceUpdating, runningName)
	assertStatus(1, v1beta1.OMENativeInstanceReady, runningName)
	assertPodRevision(pod0, "two", "newhash")
	assertPodRevision(pod1, "one", "oldhash")

	result, err = workload.Reconcile(context.Background(), deps, staleInput(), plan, target)
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if result.RequeueAfter <= 0 {
		t.Errorf("second Reconcile must retain a follow-up requeue, got %v", result.RequeueAfter)
	}
	assertStatus(0, v1beta1.OMENativeInstanceReady, targetName)
	assertStatus(1, v1beta1.OMENativeInstanceReady, runningName)
	assertPodRevision(pod1, "one", "oldhash")

	cached.stale = false
	if _, err := workload.Reconcile(context.Background(), deps, inputFromAPI(), plan, target); err != nil {
		t.Fatalf("third Reconcile: %v", err)
	}
	assertStatus(1, v1beta1.OMENativeInstanceUpdating, runningName)
	assertPodRevision(pod1, "two", "newhash")

	if _, err := workload.Reconcile(context.Background(), deps, inputFromAPI(), plan, target); err != nil {
		t.Fatalf("fourth Reconcile: %v", err)
	}
	assertStatus(1, v1beta1.OMENativeInstanceReady, targetName)
}
