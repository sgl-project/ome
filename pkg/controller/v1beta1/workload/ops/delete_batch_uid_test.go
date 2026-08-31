package ops

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

type gracefulDeleteRecordingClient struct {
	client.Client
	deleteCalls int
	deleteUIDs  []*types.UID
	deleteErr   error
}

func (c *gracefulDeleteRecordingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalls++
	deleteOptions := &client.DeleteOptions{}
	for _, opt := range opts {
		opt.ApplyToDelete(deleteOptions)
	}
	var uid *types.UID
	if deleteOptions.Preconditions != nil && deleteOptions.Preconditions.UID != nil {
		value := *deleteOptions.Preconditions.UID
		uid = &value
	}
	c.deleteUIDs = append(c.deleteUIDs, uid)
	if c.deleteErr != nil {
		return c.deleteErr
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestDeleteBatchGracefulDeleteUsesObservedPodUID(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	pod := deleteBatchPod(0)
	base := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod).Build()
	recording := &gracefulDeleteRecordingClient{Client: base}
	expectations := workload.NewExpectations()
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	if _, err := DeleteBatch(context.Background(), workload.Deps{
		Client: recording, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}}); err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	if recording.deleteCalls != 1 || len(recording.deleteUIDs) != 1 ||
		recording.deleteUIDs[0] == nil || *recording.deleteUIDs[0] != pod.UID {
		t.Fatalf("delete calls/UIDs = %d/%v, want one delete preconditioned on %q",
			recording.deleteCalls, recording.deleteUIDs, pod.UID)
	}
	if expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, status.Index) {
		t.Fatal("successful delete must retain its expectation until the Pod watch observes deletion")
	}
}

func TestDeleteBatchGracefulDeleteUIDConflictPreservesReplacement(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	observed := deleteBatchPod(0)
	replacement := observed.DeepCopy()
	replacement.UID = types.UID("replacement-pod-uid")
	base := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(replacement).Build()
	recording := &gracefulDeleteRecordingClient{
		Client: base,
		deleteErr: apierrors.NewConflict(
			schema.GroupResource{Resource: "pods"}, observed.Name, apierrors.NewBadRequest("UID precondition does not match")),
	}
	expectations := workload.NewExpectations()
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	if _, err := DeleteBatch(context.Background(), workload.Deps{
		Client: recording, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {observed}}); err != nil {
		t.Fatalf("UID conflict must be stale-snapshot success: %v", err)
	}
	if recording.deleteCalls != 1 || len(recording.deleteUIDs) != 1 ||
		recording.deleteUIDs[0] == nil || *recording.deleteUIDs[0] != observed.UID {
		t.Fatalf("delete calls/UIDs = %d/%v, want observed UID %q",
			recording.deleteCalls, recording.deleteUIDs, observed.UID)
	}
	if !expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, status.Index) {
		t.Fatal("UID conflict must roll back the delete expectation")
	}
	stored := &corev1.Pod{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(replacement), stored); err != nil {
		t.Fatalf("same-name replacement was deleted: %v", err)
	}
	if stored.UID != replacement.UID {
		t.Fatalf("stored Pod UID = %q, want replacement UID %q", stored.UID, replacement.UID)
	}
}

func TestDeleteBatchDrainUIDChangePreservesReplacement(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	observed := deleteBatchPod(0)
	observed.Status.Conditions = []corev1.PodCondition{{
		Type: podreadiness.ConditionType, Status: corev1.ConditionTrue,
	}}
	replacement := observed.DeepCopy()
	replacement.UID = types.UID("replacement-pod-uid")
	base := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(replacement).Build()
	recording := &gracefulDeleteRecordingClient{Client: base}
	expectations := workload.NewExpectations()
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: recording, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {observed}})
	if err != nil {
		t.Fatalf("UID change must request a fresh observation: %v", err)
	}
	if !result.ImmediateRequeue {
		t.Fatalf("UID change result = %+v, want immediate requeue", result)
	}
	if recording.deleteCalls != 0 {
		t.Fatalf("UID change issued %d delete call(s), want 0", recording.deleteCalls)
	}
	if !expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, status.Index) {
		t.Fatal("UID change must not leave a delete expectation")
	}
	stored := &corev1.Pod{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(replacement), stored); err != nil {
		t.Fatalf("get same-name replacement: %v", err)
	}
	if !podreadiness.IsServing(stored) {
		t.Fatal("stale drain marked the same-name replacement NotServing")
	}
}

func TestDeleteBatchGracefulDeleteRequiresObservedPodUID(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	pod := deleteBatchPod(0)
	pod.UID = ""
	base := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod).Build()
	recording := &gracefulDeleteRecordingClient{Client: base}
	expectations := workload.NewExpectations()
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: recording, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
	if err == nil || !strings.Contains(err.Error(), "without an observed UID") {
		t.Fatalf("DeleteBatch error = %v, want missing observed UID", err)
	}
	if recording.deleteCalls != 0 {
		t.Fatalf("missing UID issued %d delete call(s), want zero", recording.deleteCalls)
	}
	if !expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, status.Index) {
		t.Fatal("missing UID must not leave a delete expectation")
	}
	stored := &corev1.Pod{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(pod), stored); err != nil {
		t.Fatalf("missing-UID Pod changed: %v", err)
	}
}

func TestDeleteBatchGracefulDeletePreservesOtherErrors(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	pod := deleteBatchPod(0)
	base := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod).Build()
	recording := &gracefulDeleteRecordingClient{
		Client:    base,
		deleteErr: apierrors.NewServiceUnavailable("injected delete failure"),
	}
	expectations := workload.NewExpectations()
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: recording, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
	if err == nil || !apierrors.IsServiceUnavailable(err) {
		t.Fatalf("DeleteBatch error = %v, want preserved ServiceUnavailable", err)
	}
	if recording.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", recording.deleteCalls)
	}
	if !expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, status.Index) {
		t.Fatal("failed delete must roll back its expectation")
	}
}

func TestDeleteBatchResumedWaveRequiresAtomicStatusAdapter(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})

	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(),
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err == nil || !strings.Contains(err.Error(), "owner-aware atomic status adapter is required") {
		t.Fatalf("DeleteBatch error = %v, want missing atomic adapter", err)
	}
}
