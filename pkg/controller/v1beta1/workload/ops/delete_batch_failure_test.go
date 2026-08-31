package ops

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clocktesting "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

type deleteFailureClient struct {
	client.Client
	statusPatchErrorPod string
	statusPatchErr      error
	deleteHook          func(context.Context, client.Object, ...client.DeleteOption) error
	deleteCalls         []string
}

func (c *deleteFailureClient) Status() client.SubResourceWriter {
	return &deleteFailureStatusWriter{
		SubResourceWriter: c.Client.Status(),
		podName:           c.statusPatchErrorPod,
		err:               c.statusPatchErr,
	}
}

func (c *deleteFailureClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Pod); ok {
		c.deleteCalls = append(c.deleteCalls, obj.GetName())
	}
	if c.deleteHook != nil {
		return c.deleteHook(ctx, obj, opts...)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

type deleteFailureStatusWriter struct {
	client.SubResourceWriter
	podName string
	err     error
}

func (w *deleteFailureStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if pod, ok := obj.(*corev1.Pod); ok && pod.Name == w.podName {
		return w.err
	}
	return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
}

type deleteFailureReader struct {
	client.Reader
	endpointSliceListErr error
	serviceGetErr        error
	endpointSliceLists   int
	serviceGets          int
}

func (r *deleteFailureReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*discoveryv1.EndpointSliceList); ok {
		r.endpointSliceLists++
		if r.endpointSliceListErr != nil {
			return r.endpointSliceListErr
		}
	}
	return r.Reader.List(ctx, list, opts...)
}

func (r *deleteFailureReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		r.serviceGets++
		if r.serviceGetErr != nil {
			return r.serviceGetErr
		}
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func newDeleteFailureBaseClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := discoveryv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&corev1.Pod{}).
		WithObjects(objects...).
		Build()
}

func deleteFailurePod(index int32, serving bool, revision string) *corev1.Pod {
	pod := deleteBatchPod(index)
	pod.UID = types.UID(fmt.Sprintf("pod-%d-uid", index))
	pod.Spec.Containers = []corev1.Container{{Name: "main", Image: "example.com/model:latest"}}
	if revision != "" {
		pod.Labels[query.LabelRevisionHash] = revision
	}
	if serving {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   podreadiness.ConditionType,
			Status: corev1.ConditionTrue,
		})
	}
	return pod
}

func deleteFailureSlice(namespace, name, service string, endpoints ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    map[string]string{discoveryv1.LabelServiceName: service},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   endpoints,
	}
}

func deleteFailureEndpoint(pod *corev1.Pod, ready bool) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{
		Addresses: []string{"192.0.2.1"},
		Conditions: discoveryv1.EndpointConditions{
			Ready: ptr.To(ready),
		},
		TargetRef: &corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
	}
}

func TestDeleteBatchAdmissionAPIErrorsFailBeforeExternalEffects(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "413", err: apierrors.NewRequestEntityTooLargeError("status object too large")},
		{name: "429", err: apierrors.NewTooManyRequests("status write throttled", 1)},
		{name: "5xx", err: apierrors.NewServiceUnavailable("status storage unavailable")},
		{name: "context cancellation", err: context.Canceled},
		{name: "generic write error", err: errors.New("injected status write failure")},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := deleteBatchOwner()
			statuses := []workload.InstanceStatus{{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady}}
			pod := deleteFailurePod(0, true, "rev-a")
			base := newDeleteFailureBaseClient(t, pod)
			c := &deleteFailureClient{Client: base}
			reads := &deleteFailureReader{Reader: base}
			finalized := 0
			adapterCalls := 0
			input := deleteBatchInput(owner, statuses)
			input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
				finalized++
				return true, nil
			}
			input.ApplyInstanceMutationsWithRetryBlock = func(context.Context, []workload.InstanceMutation, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
				adapterCalls++
				return test.err
			}

			result, err := DeleteBatch(context.Background(), workload.Deps{
				Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
			}, input, deleteBatchPlan(), []int32{0}, map[int32][]*corev1.Pod{0: {pod}})
			if err == nil || !errors.Is(err, test.err) {
				t.Fatalf("result/error = %+v/%v, want wrapped %v", result, err, test.err)
			}
			if adapterCalls != 1 || finalized != 0 || len(c.deleteCalls) != 0 {
				t.Fatalf("calls = adapter:%d finalizer:%d deletes:%v", adapterCalls, finalized, c.deleteCalls)
			}
			if reads.endpointSliceLists != 0 || reads.serviceGets != 0 {
				t.Fatalf("failed admission reached drain reads: slices=%d services=%d", reads.endpointSliceLists, reads.serviceGets)
			}
			got := &corev1.Pod{}
			if err := base.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil {
				t.Fatalf("failed admission removed Pod: %v", err)
			}
			if !podreadiness.IsServing(got) {
				t.Fatalf("failed admission changed readiness: %+v", got.Status.Conditions)
			}
		})
	}
}

func TestDeleteBatchAdmissionOwnerGoneReplansBeforeExternalEffects(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady}}
	pod := deleteFailurePod(0, true, "rev-a")
	base := newDeleteFailureBaseClient(t, pod)
	c := &deleteFailureClient{Client: base}
	reads := &deleteFailureReader{Reader: base}
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = func(context.Context, []workload.InstanceMutation, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		return workload.ErrStatusOwnerGone
	}

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
	}, input, deleteBatchPlan(), []int32{0}, map[int32][]*corev1.Pod{0: {pod}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || len(c.deleteCalls) != 0 || reads.endpointSliceLists != 0 || reads.serviceGets != 0 {
		t.Fatalf("owner-gone result/effects = %+v deletes:%v reads:%d/%d",
			result, c.deleteCalls, reads.endpointSliceLists, reads.serviceGets)
	}
	got := &corev1.Pod{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(pod), got); err != nil || !podreadiness.IsServing(got) {
		t.Fatalf("owner-gone admission changed Pod: err=%v conditions=%+v", err, got.Status.Conditions)
	}
}

func TestDeleteBatchReadinessFailureStopsWholeWaveBeforeDrainOrDelete(t *testing.T) {
	for _, test := range []struct {
		name      string
		failedPod int32
	}{
		{name: "first Pod", failedPod: 2},
		{name: "middle Pod", failedPod: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := deleteBatchOwner()
			started := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
			statuses := []workload.InstanceStatus{
				deleteOwnedStatus(2, started),
				deleteOwnedStatus(1, started),
				deleteOwnedStatus(0, started),
			}
			pods := map[int32][]*corev1.Pod{}
			objects := make([]client.Object, 0, len(statuses))
			for _, status := range statuses {
				pod := deleteFailurePod(status.Index, true, "rev-a")
				pods[status.Index] = []*corev1.Pod{pod}
				objects = append(objects, pod)
			}
			base := newDeleteFailureBaseClient(t, objects...)
			c := &deleteFailureClient{
				Client:              base,
				statusPatchErrorPod: pods[test.failedPod][0].Name,
				statusPatchErr:      errors.New("injected serving-gate patch failure"),
			}
			reads := &deleteFailureReader{Reader: base}
			store := newDeleteMutationStore(owner, statuses)
			input := deleteBatchInput(owner, statuses)
			input.ApplyInstanceMutationsWithRetryBlock = store.apply

			_, err := DeleteBatch(context.Background(), workload.Deps{
				Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
			}, input, deleteBatchPlan(), nil, pods)
			if err == nil || !strings.Contains(err.Error(), "mark not serving") {
				t.Fatalf("error = %v, want readiness failure", err)
			}
			if len(c.deleteCalls) != 0 {
				t.Fatalf("readiness failure issued Pod deletes: %v", c.deleteCalls)
			}
			if reads.endpointSliceLists != 0 || reads.serviceGets != 0 {
				t.Fatalf("readiness failure reached drain reads: slices=%d services=%d", reads.endpointSliceLists, reads.serviceGets)
			}
		})
	}
}

func TestDeleteBatchDrainReadFailureStopsBeforeDelete(t *testing.T) {
	for _, test := range []struct {
		name        string
		configure   func(*deleteFailureReader)
		wantMessage string
		wantLists   int
		wantGets    int
	}{
		{
			name: "EndpointSlice list",
			configure: func(reader *deleteFailureReader) {
				reader.endpointSliceListErr = errors.New("injected EndpointSlice read failure")
			},
			wantMessage: "list EndpointSlices",
			wantLists:   1,
		},
		{
			name: "empty-slice Service lookup",
			configure: func(reader *deleteFailureReader) {
				reader.serviceGetErr = errors.New("injected Service read failure")
			},
			wantMessage: "get service",
			wantLists:   1,
			wantGets:    1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := deleteBatchOwner()
			statuses := []workload.InstanceStatus{deleteOwnedStatus(0, time.Now())}
			pod := deleteFailurePod(0, false, "rev-a")
			base := newDeleteFailureBaseClient(t, pod)
			c := &deleteFailureClient{Client: base}
			reads := &deleteFailureReader{Reader: base}
			test.configure(reads)
			store := newDeleteMutationStore(owner, statuses)
			input := deleteBatchInput(owner, statuses)
			input.ApplyInstanceMutationsWithRetryBlock = store.apply

			_, err := DeleteBatch(context.Background(), workload.Deps{
				Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
			}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
			if err == nil || !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("error = %v, want %q", err, test.wantMessage)
			}
			if len(c.deleteCalls) != 0 {
				t.Fatalf("drain read failure issued Pod deletes: %v", c.deleteCalls)
			}
			if reads.endpointSliceLists != test.wantLists || reads.serviceGets != test.wantGets {
				t.Fatalf("drain reads = slices:%d services:%d, want %d/%d",
					reads.endpointSliceLists, reads.serviceGets, test.wantLists, test.wantGets)
			}
		})
	}
}

func TestDeleteBatchDeferredMembersDoNotBlockEligiblePeer(t *testing.T) {
	owner := deleteBatchOwner()
	started := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	statuses := []workload.InstanceStatus{
		deleteOwnedStatus(2, started),
		deleteOwnedStatus(1, started),
		deleteOwnedStatus(0, started),
	}
	pod2 := deleteFailurePod(2, false, "rev-a")
	pod1 := deleteFailurePod(1, false, "rev-a")
	pod0 := deleteFailurePod(0, false, "rev-a")
	service := query.PerRevisionServiceName("llama", workload.ComponentEngine, "rev-a")
	slice := deleteFailureSlice("prod", "rev-a-slice", service,
		deleteFailureEndpoint(pod2, true),
		deleteFailureEndpoint(pod1, false),
		deleteFailureEndpoint(pod0, false),
	)
	base := newDeleteFailureBaseClient(t, pod2, pod1, pod0, slice)
	c := &deleteFailureClient{Client: base}
	reads := &deleteFailureReader{Reader: base}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 1, 1)
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: c, APIReader: reads, Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{
		2: {pod2}, 1: {pod1}, 0: {pod0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.InProgress || result.ImmediateRequeue {
		t.Fatalf("result = %+v", result)
	}
	for _, pod := range []*corev1.Pod{pod2, pod1} {
		if err := base.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{}); err != nil {
			t.Fatalf("deferred Pod %s was deleted: %v", pod.Name, err)
		}
	}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(pod0), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("eligible peer Pod was not deleted: %v", err)
	}
	if len(c.deleteCalls) != 1 || c.deleteCalls[0] != pod0.Name {
		t.Fatalf("delete calls = %v, want [%s]", c.deleteCalls, pod0.Name)
	}
	if reads.endpointSliceLists != 1 {
		t.Fatalf("EndpointSlice lists = %d, want one shared read", reads.endpointSliceLists)
	}
}

func TestDeleteBatchDeleteErrorsRollbackOnlyFailedExpectations(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{deleteOwnedStatus(0, time.Now())}

	t.Run("NotFound", func(t *testing.T) {
		pod := deleteFailurePod(0, false, "")
		base := newDeleteFailureBaseClient(t)
		c := &deleteFailureClient{Client: base}
		c.deleteHook = func(context.Context, client.Object, ...client.DeleteOption) error {
			return apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, pod.Name)
		}
		expectations := workload.NewExpectations()
		store := newDeleteMutationStore(owner, statuses)
		input := deleteBatchInput(owner, statuses)
		input.ApplyInstanceMutationsWithRetryBlock = store.apply

		if _, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: expectations},
			input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}}); err != nil {
			t.Fatal(err)
		}
		if !expectations.Satisfied("prod", "llama", workload.ComponentEngine, 0) {
			t.Fatal("NotFound left a delete expectation that no watch event can satisfy")
		}
	})

	t.Run("transport failure", func(t *testing.T) {
		pod := deleteFailurePod(0, false, "")
		base := newDeleteFailureBaseClient(t)
		c := &deleteFailureClient{Client: base}
		c.deleteHook = func(context.Context, client.Object, ...client.DeleteOption) error {
			return errors.New("injected transport failure")
		}
		expectations := workload.NewExpectations()
		store := newDeleteMutationStore(owner, statuses)
		input := deleteBatchInput(owner, statuses)
		input.ApplyInstanceMutationsWithRetryBlock = store.apply

		_, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: expectations},
			input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
		if err == nil || !strings.Contains(err.Error(), "transport failure") {
			t.Fatalf("error = %v", err)
		}
		if !expectations.Satisfied("prod", "llama", workload.ComponentEngine, 0) {
			t.Fatal("failed Delete RPC left a phantom expectation")
		}
	})

	t.Run("partial success resumes across controller restarts", func(t *testing.T) {
		podA := deleteFailurePod(0, false, "")
		podA.Name = "pod-a"
		podB := deleteFailurePod(0, false, "")
		podB.Name = "pod-b"
		base := newDeleteFailureBaseClient(t, podA, podB)
		failSecond := true
		c := &deleteFailureClient{Client: base}
		c.deleteHook = func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetName() == podB.Name && failSecond {
				failSecond = false
				return errors.New("injected second-delete failure")
			}
			return base.Delete(ctx, obj, opts...)
		}
		firstExpectations := workload.NewExpectations()
		store := newDeleteMutationStore(owner, statuses)
		input := deleteBatchInput(owner, statuses)
		input.ApplyInstanceMutationsWithRetryBlock = store.apply

		_, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: firstExpectations},
			input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {podA, podB}})
		if err == nil || !strings.Contains(err.Error(), "second-delete failure") {
			t.Fatalf("first pass error = %v", err)
		}
		if firstExpectations.Satisfied("prod", "llama", workload.ComponentEngine, 0) {
			t.Fatal("successful first delete expectation was rolled back with the failed peer")
		}
		if err := base.Get(context.Background(), client.ObjectKeyFromObject(podA), &corev1.Pod{}); !apierrors.IsNotFound(err) {
			t.Fatalf("first Pod should be gone: %v", err)
		}
		if err := base.Get(context.Background(), client.ObjectKeyFromObject(podB), &corev1.Pod{}); err != nil {
			t.Fatalf("failed second Pod delete should leave Pod: %v", err)
		}

		resumedExpectations := workload.NewExpectations()
		if _, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: resumedExpectations},
			input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {podB}}); err != nil {
			t.Fatalf("post-restart resume pass: %v", err)
		}
		if err := base.Get(context.Background(), client.ObjectKeyFromObject(podB), &corev1.Pod{}); !apierrors.IsNotFound(err) {
			t.Fatalf("post-restart resume pass did not delete remaining Pod: %v", err)
		}

		completionExpectations := workload.NewExpectations()
		finalized := 0
		input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
			finalized++
			return true, nil
		}
		result, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: completionExpectations},
			input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
		if err != nil {
			t.Fatalf("post-restart completion pass: %v", err)
		}
		if !result.ImmediateRequeue || finalized != 1 || len(store.statuses) != 0 {
			t.Fatalf("completion result/finalized/statuses = %+v/%d/%v", result, finalized, store.statuses)
		}
	})
}

func TestDeleteBatchUnconfirmedCompletionKeepsExpectations(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{deleteOwnedStatus(0, time.Now())}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 0, 1)
	completionErr := errors.New("injected completion status failure")
	input := deleteBatchInput(owner, statuses)
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
	input.ApplyInstanceMutationsWithRetryBlock = func(_ context.Context, mutations []workload.InstanceMutation, _ string, _ func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		for _, mutation := range mutations {
			if mutation.Remove {
				return completionErr
			}
		}
		return nil
	}

	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: newDeleteFailureBaseClient(t), Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err == nil || !errors.Is(err, completionErr) {
		t.Fatalf("completion error = %v, want %v", err, completionErr)
	}
	if expectations.Satisfied("prod", "llama", workload.ComponentEngine, 0) {
		t.Fatal("unconfirmed status removal cleared delete expectations")
	}
}

func TestCompleteDeleteBatchRejectsConcurrentPhaseDrift(t *testing.T) {
	owner := deleteBatchOwner()
	expected := deleteOwnedStatus(0, time.Now())
	current := cloneDeleteInstanceStatus(expected)
	current.Phase = workload.InstancePhaseFailed
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{current})
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 0, 1)
	input := deleteBatchInput(owner, []workload.InstanceStatus{expected})
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	committed, err := completeDeleteBatch(context.Background(), workload.Deps{Expectations: expectations}, input,
		[]deleteBatchCandidate{{status: expected}})
	if !errors.Is(err, workload.ErrStatusMutationPrecondition) {
		t.Fatalf("completion error = %v, want %v", err, workload.ErrStatusMutationPrecondition)
	}
	if committed {
		t.Fatal("completion committed after the Instance left Phase=Deleting")
	}
	if got := store.statuses[0]; got.Phase != workload.InstancePhaseFailed {
		t.Fatalf("concurrent status = %+v, want Phase=Failed retained", got)
	}
	if expectations.Satisfied("prod", "llama", workload.ComponentEngine, 0) {
		t.Fatal("rejected completion cleared delete expectations")
	}
}

func TestDeleteBatchStuckTerminatingEscalatesBeforeExpectationGate(t *testing.T) {
	isvc := fdISVC("batch-force-delete")
	statuses := []workload.InstanceStatus{deleteOwnedStatus(0, fdNow.Add(-20*time.Minute))}
	pod := fdTerminatingPod("batch-force-delete-pod", "dead-node", overdueTS)
	pod.Labels = map[string]string{query.LabelInstanceIdx: "0"}
	var deletes []recordedDeleteOpts
	funcs := fdDeleteRecorder(&deletes)
	c := fdFakeClient(t, &funcs, isvc, fdStoredCopy(pod))
	store := newDeleteMutationStore(isvc, statuses)
	input := deleteBatchInput(isvc, statuses)
	input.OwnerGVK = v1beta1.SchemeGroupVersion.WithKind("InferenceService")
	input.EventTarget = isvc
	input.ForceDelete = fdPolicy()
	input.Clock = clocktesting.NewFakeClock(fdNow)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, 0, 1)
	recorder := record.NewFakeRecorder(8)

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: c, APIReader: c, Expectations: expectations, Recorder: recorder,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.InProgress || result.ImmediateRequeue {
		t.Fatalf("result = %+v", result)
	}
	if len(deletes) != 1 || deletes[0].grace == nil || *deletes[0].grace != 0 ||
		deletes[0].uid == nil || *deletes[0].uid != pod.UID {
		t.Fatalf("force deletes = %+v, want one grace-0 UID-preconditioned delete", deletes)
	}
	if expectations.Satisfied(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, 0) {
		t.Fatal("escalation unexpectedly cleared the pre-existing graceful-delete expectation")
	}
	if got := fdCountEvents(fdDrainEvents(recorder), workload.EventReasonPodForceDeleted); got != 1 {
		t.Fatalf("force-delete events = %d, want 1", got)
	}
}

func TestDeleteBatchWithoutPollReturnsForceDeleteEvidenceReadError(t *testing.T) {
	owner := deleteBatchOwner()
	status := deleteOwnedStatus(0, fdNow.Add(-20*time.Minute))
	pod := fdTerminatingPod("batch-node-read-error", "unreadable-node", overdueTS)
	pod.Labels = map[string]string{query.LabelInstanceIdx: "0"}
	base := newDeleteFailureBaseClient(t)
	reader := interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Node); ok {
				return errors.New("injected Node read failure")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	store := newDeleteMutationStore(owner, []workload.InstanceStatus{status})
	input := deleteBatchInput(owner, []workload.InstanceStatus{status})
	input.ForceDelete = fdPolicy()
	input.Clock = clocktesting.NewFakeClock(fdNow)
	input.ScaleDownRequeueInterval = 0
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: base, APIReader: reader, Expectations: workload.NewExpectations(),
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{0: {pod}})
	if err == nil || !strings.Contains(err.Error(), "injected Node read failure") {
		t.Fatalf("DeleteBatch error = %v, want force-delete evidence read failure", err)
	}
	if store.writes != 0 || store.statuses[0].Phase != workload.InstancePhaseDeleting {
		t.Fatalf("evidence read failure changed status: writes=%d status=%+v", store.writes, store.statuses[0])
	}
}

func TestDeleteBatchHighScaleBudgetBoundsEffectsAndUsesOneDrainObservation(t *testing.T) {
	const replicas = int32(2000)
	const budget = int32(100)
	owner := deleteBatchOwner()
	started := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	statuses := make([]workload.InstanceStatus, 0, replicas)
	pods := make(map[int32][]*corev1.Pod, replicas)
	endpoints := make([]discoveryv1.Endpoint, 0, replicas)
	for index := int32(0); index < replicas; index++ {
		statuses = append(statuses, deleteOwnedStatus(index, started))
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace: "prod",
			Name:      fmt.Sprintf("pod-%04d", index),
			UID:       types.UID(fmt.Sprintf("pod-%04d-uid", index)),
			Labels:    map[string]string{query.LabelRevisionHash: "rev-scale"},
		}}
		pods[index] = []*corev1.Pod{pod}
		endpoints = append(endpoints, deleteFailureEndpoint(pod, false))
	}
	service := query.PerRevisionServiceName("llama", workload.ComponentEngine, "rev-scale")
	slice := deleteFailureSlice("prod", "rev-scale-slice", service, endpoints...)
	base := newDeleteFailureBaseClient(t, slice)
	c := &deleteFailureClient{Client: base}
	c.deleteHook = func(context.Context, client.Object, ...client.DeleteOption) error { return nil }
	reads := &deleteFailureReader{Reader: base}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ScaleDownPodBatchSize = ptr.To(budget)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
	}, input, deleteBatchPlan(), nil, pods)
	if err != nil {
		t.Fatal(err)
	}
	if result.SelectedPodCost != budget || result.Deferred != int(replicas-budget) || !result.InProgress {
		t.Fatalf("result = %+v", result)
	}
	if reads.endpointSliceLists != 1 || reads.serviceGets != 0 {
		t.Fatalf("drain reads = slices:%d services:%d, want 1/0", reads.endpointSliceLists, reads.serviceGets)
	}
	if got := len(c.deleteCalls); got != int(budget) {
		t.Fatalf("Pod delete calls = %d, want %d", got, budget)
	}
	for index := int32(0); index < replicas-budget; index++ {
		if slices.Contains(c.deleteCalls, fmt.Sprintf("pod-%04d", index)) {
			t.Fatalf("deferred Pod %04d received a delete effect", index)
		}
	}
	if store.writes != 0 {
		t.Fatalf("resuming an owned wave wrote status %d times", store.writes)
	}
}

func TestDeleteBatchHighScaleFreshAdmissionWritesOnceBeforeEffects(t *testing.T) {
	const replicas = int32(2000)
	const budget = int32(100)
	owner := deleteBatchOwner()
	statuses := make([]workload.InstanceStatus, 0, replicas)
	extras := make([]int32, 0, replicas-1)
	pods := make(map[int32][]*corev1.Pod, replicas)
	for index := int32(0); index < replicas; index++ {
		statuses = append(statuses, workload.InstanceStatus{Index: index, Incarnation: 1, Phase: workload.InstancePhaseReady})
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: fmt.Sprintf("pod-%04d", index)}}
		pods[index] = []*corev1.Pod{pod}
		if index > 0 {
			extras = append(extras, index)
		}
	}
	base := newDeleteFailureBaseClient(t)
	c := &deleteFailureClient{Client: base}
	reads := &deleteFailureReader{Reader: base}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ScaleDownPodBatchSize = ptr.To(budget)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: c, APIReader: reads, Expectations: workload.NewExpectations(),
	}, input, deleteBatchPlan(), extras, pods)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || result.SelectedPodCost != budget || result.Deferred != 1899 {
		t.Fatalf("result = %+v", result)
	}
	if store.writes != 1 || len(store.mutations) != 1 || store.mutations[0] != int(budget) {
		t.Fatalf("status commits/mutations = %d/%v, want one 100-Instance admission", store.writes, store.mutations)
	}
	for index := int32(0); index < replicas; index++ {
		status := store.statuses[index]
		selected := index >= replicas-budget
		if selected && (status.Phase != workload.InstancePhaseDeleting || status.Operation == nil || status.Operation.Type != workload.InstanceOperationDelete) {
			t.Fatalf("selected Instance %d was not admitted: %+v", index, status)
		}
		if !selected && (status.Phase != workload.InstancePhaseReady || status.Operation != nil) {
			t.Fatalf("retained/deferred Instance %d was mutated: %+v", index, status)
		}
	}
	if len(c.deleteCalls) != 0 || reads.endpointSliceLists != 0 || reads.serviceGets != 0 {
		t.Fatalf("admission external effects: deletes=%d slices=%d services=%d", len(c.deleteCalls), reads.endpointSliceLists, reads.serviceGets)
	}
}
