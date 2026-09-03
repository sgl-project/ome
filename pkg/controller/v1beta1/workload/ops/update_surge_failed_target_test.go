package ops

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

type failedCreateContainerSurgeFixture struct {
	isvc         *v1beta1.InferenceService
	client       client.Client
	recording    *gracefulDeleteRecordingClient
	expectations *workload.Expectations
	input        workload.ReconcileInput
	plan         workload.ComponentPlan
	target       *appsv1.ControllerRevision
	source       *corev1.Pod
	failed       *corev1.Pod
}

func newFailedCreateContainerSurgeFixture(t *testing.T, excludedNodes []string) failedCreateContainerSurgeFixture {
	t.Helper()
	legacyResetExpectations(t)

	isvc, _ := surgeISVCReady("llama-70b", "prod", 1)
	sourceRevision := "llama-70b-engine-rev-sourcehash"
	targetRevision := "llama-70b-engine-rev-targethash"
	failedName := query.PodName(isvc.Name, workload.ComponentEngine, 0, "default", 1)
	ir := legacyInstanceIR(isvc, workload.ComponentEngine, v1beta1.OMENativeInstanceStatus{
		Index:           0,
		Incarnation:     1,
		Phase:           v1beta1.OMENativeInstanceFailed,
		RunningRevision: sourceRevision,
		TargetRevision:  targetRevision,
		ActiveOrdinal:   0,
		LastFailure: &v1beta1.InstanceTermination{
			PodName: failedName,
			Reason:  createContainerErrorReason,
		},
	})

	source := surgePodAtOrdinal(isvc, 0, 1, 0, true, true)
	source.UID = k8stypes.UID("source-uid")
	source.Spec.NodeName = "node-source"
	source.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(sourceRevision)

	failed := surgePodAtOrdinal(isvc, 0, 1, 1, false, false)
	failed.UID = k8stypes.UID("failed-target-uid")
	failed.Spec.NodeName = "node-target"
	failed.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(targetRevision)
	failed.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "main",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason:  createContainerErrorReason,
			Message: "runtime mount unavailable",
		}},
	}}

	base := legacyNewFakeClient(t, isvc, ir, source, failed)
	recording := &gracefulDeleteRecordingClient{Client: base}
	target := makeCR(t, recording, isvc, targetRevision)
	input := legacyTestInput(isvc, recording, workload.ComponentEngine)
	input.ObservedState.UpdateRevision = targetRevision
	input.ObservedState.InstanceStatuses[0].LastFailure = &workload.InstanceTermination{
		PodName: failedName,
		Reason:  createContainerErrorReason,
	}
	plan := surgePlan()
	plan.Instances[0].ExcludedNodes = append([]string(nil), excludedNodes...)

	return failedCreateContainerSurgeFixture{
		isvc:         isvc,
		client:       base,
		recording:    recording,
		expectations: workload.NewExpectations(),
		input:        input,
		plan:         plan,
		target:       target,
		source:       source,
		failed:       failed,
	}
}

func (f failedCreateContainerSurgeFixture) deps() workload.Deps {
	return workload.Deps{Client: f.recording, Expectations: f.expectations}
}

func TestSurgeUpdate_RecyclesFailedCreateContainerTargetThenRetries(t *testing.T) {
	f := newFailedCreateContainerSurgeFixture(t, []string{"node-target"})

	done, err := surgeUpdate(context.Background(), f.deps(), f.input, f.plan, f.plan.Instances[0], f.target,
		[]*corev1.Pod{f.source, f.failed})
	if err != nil {
		t.Fatalf("recycle failed target: %v", err)
	}
	if done {
		t.Fatal("failed-target cleanup must not report rollout completion")
	}
	if f.recording.deleteCalls != 1 || len(f.recording.deleteUIDs) != 1 ||
		f.recording.deleteUIDs[0] == nil || *f.recording.deleteUIDs[0] != f.failed.UID {
		t.Fatalf("delete calls/UIDs = %d/%v, want one delete preconditioned on %q",
			f.recording.deleteCalls, f.recording.deleteUIDs, f.failed.UID)
	}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.failed), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("failed target still exists after recycle: %v", err)
	}
	storedSource := &corev1.Pod{}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.source), storedSource); err != nil {
		t.Fatalf("serving source was deleted: %v", err)
	}
	if !podreadiness.IsServing(storedSource) {
		t.Fatal("serving source left rotation during failed-target cleanup")
	}
	status := legacyInstanceStatusesOnIR(f.client, f.isvc, workload.ComponentEngine)[0]
	if status.Phase != v1beta1.OMENativeInstanceFailed || status.Operation != nil {
		t.Fatalf("status after delete = Phase %q Operation %+v, want Failed with no operation until deletion is observed",
			status.Phase, status.Operation)
	}

	// Simulate the delete watch observation. The next pass must create a fresh
	// target in the same ordinal with the recorded node exclusion applied.
	f.expectations.ObservedDelete("prod", "llama-70b", workload.ComponentEngine, 0)
	input := legacyTestInput(f.isvc, f.recording, workload.ComponentEngine)
	input.ObservedState.UpdateRevision = f.target.Name
	input.ObservedState.InstanceStatuses[0].LastFailure = &workload.InstanceTermination{
		PodName: f.failed.Name,
		Reason:  createContainerErrorReason,
	}
	if _, err := surgeUpdate(context.Background(), f.deps(), input, f.plan, f.plan.Instances[0], f.target,
		[]*corev1.Pod{storedSource}); err != nil {
		t.Fatalf("start replacement attempt: %v", err)
	}
	replacement := &corev1.Pod{}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.failed), replacement); err != nil {
		t.Fatalf("fresh target was not created: %v", err)
	}
	if got := hostnameNotInValues(replacement); len(got) != 1 || got[0] != "node-target" {
		t.Fatalf("replacement hostname exclusions = %v, want [node-target]", got)
	}
	if !podreadiness.IsServing(storedSource) {
		t.Fatal("serving source changed while replacement was created")
	}
}

func TestSurgeUpdate_FailedCreateContainerTargetParksWithoutRelocationAuthorization(t *testing.T) {
	f := newFailedCreateContainerSurgeFixture(t, []string{"previous-node-a", "previous-node-b"})

	if _, err := surgeUpdate(context.Background(), f.deps(), f.input, f.plan, f.plan.Instances[0], f.target,
		[]*corev1.Pod{f.source, f.failed}); err != nil {
		t.Fatalf("park failed target: %v", err)
	}
	if f.recording.deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want zero without a matching relocation directive", f.recording.deleteCalls)
	}
	storedTarget := &corev1.Pod{}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.failed), storedTarget); err != nil {
		t.Fatalf("failed target was removed without authorization: %v", err)
	}
	status := legacyInstanceStatusesOnIR(f.client, f.isvc, workload.ComponentEngine)[0]
	if status.Phase != v1beta1.OMENativeInstanceFailed || status.Operation != nil {
		t.Fatalf("parked status = Phase %q Operation %+v, want Failed with no operation", status.Phase, status.Operation)
	}
}

func TestSurgeUpdate_FailedCreateContainerTargetRequiresServingSource(t *testing.T) {
	f := newFailedCreateContainerSurgeFixture(t, []string{"node-target"})
	liveSource := &corev1.Pod{}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.source), liveSource); err != nil {
		t.Fatalf("get source: %v", err)
	}
	for i := range liveSource.Status.Conditions {
		if liveSource.Status.Conditions[i].Type == query.ServingConditionType {
			liveSource.Status.Conditions[i].Status = corev1.ConditionFalse
		}
	}
	if err := f.client.Status().Update(context.Background(), liveSource); err != nil {
		t.Fatalf("mark source non-serving: %v", err)
	}

	if _, err := surgeUpdate(context.Background(), f.deps(), f.input, f.plan, f.plan.Instances[0], f.target,
		[]*corev1.Pod{liveSource, f.failed}); err != nil {
		t.Fatalf("park without a serving source: %v", err)
	}
	if f.recording.deleteCalls != 0 {
		t.Fatalf("delete calls = %d, want zero while the source is not serving", f.recording.deleteCalls)
	}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.failed), &corev1.Pod{}); err != nil {
		t.Fatalf("failed target was removed without a serving source: %v", err)
	}
}
