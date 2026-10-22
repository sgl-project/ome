package testfixture_test

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func condition(pod *corev1.Pod, t corev1.PodConditionType) *corev1.PodCondition {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == t {
			return &pod.Status.Conditions[i]
		}
	}
	return nil
}

func TestServingPod_ReadyAndServingForAnHourBeforeEpoch(t *testing.T) {
	pod := fixture.ServingPod("a")
	if pod.Status.Phase != corev1.PodRunning {
		t.Fatalf("phase = %q, want Running", pod.Status.Phase)
	}
	readySince := fixture.Epoch.Add(-time.Hour)
	for _, typ := range []corev1.PodConditionType{corev1.ContainersReady, podreadiness.ConditionType, corev1.PodReady} {
		c := condition(pod, typ)
		if c == nil || c.Status != corev1.ConditionTrue || !c.LastTransitionTime.Time.Equal(readySince) {
			t.Errorf("condition %s = %+v, want True since %s", typ, c, readySince)
		}
	}
	if cs := pod.Status.ContainerStatuses; len(cs) != 1 || !cs[0].Ready || cs[0].Name != constants.MainContainerName {
		t.Errorf("container statuses = %+v, want one ready %s", cs, constants.MainContainerName)
	}
	if got := pod.Labels[query.LabelRevisionHash]; got != fixture.RevisionHashA {
		t.Errorf("revision label = %q, want %q", got, fixture.RevisionHashA)
	}
	if got := pod.Labels[query.LabelRunner]; got != types.RunnerDefault {
		t.Errorf("runner label = %q, want %q", got, types.RunnerDefault)
	}
	if !pod.CreationTimestamp.Time.Equal(fixture.Epoch.Add(-2 * time.Hour)) {
		t.Errorf("created %s, want two hours before Epoch", pod.CreationTimestamp.Time)
	}
}

func TestPod_AppliesTheOptionsInOrder(t *testing.T) {
	at := fixture.Epoch.Add(-time.Minute)
	pod := fixture.Pod("a", fixture.UnreadySince(fixture.Epoch.Add(-10*time.Minute)), fixture.ReadySince(at))
	if c := condition(pod, corev1.PodReady); c.Status != corev1.ConditionTrue || !c.LastTransitionTime.Time.Equal(at) {
		t.Errorf("PodReady = %+v, want True since %s (the later option wins)", c, at)
	}
	if c := condition(pod, corev1.ContainersReady); c.Status != corev1.ConditionFalse {
		t.Errorf("ContainersReady = %+v, want False (left by the first option)", c)
	}
	if pods := fixture.Pods(pod, fixture.ServingPod("b")); len(pods) != 2 || pods[1].Name != "b" {
		t.Errorf("Pods = %v, want the two pods in order", pods)
	}
}

func TestPodOptions_ChangeOneThingEach(t *testing.T) {
	at := fixture.Epoch.Add(-3 * time.Minute)
	cases := map[string]struct {
		option fixture.PodOption
		check  func(*corev1.Pod) bool
	}{
		"Failed":    {fixture.Failed, func(p *corev1.Pod) bool { return p.Status.Phase == corev1.PodFailed }},
		"Succeeded": {fixture.Succeeded, func(p *corev1.Pod) bool { return p.Status.Phase == corev1.PodSucceeded }},
		"Terminating": {fixture.Terminating, func(p *corev1.Pod) bool {
			return p.DeletionTimestamp != nil && p.DeletionTimestamp.Time.Equal(fixture.Epoch)
		}},
		"Pending":             {fixture.Pending, func(p *corev1.Pod) bool { return p.Status.Phase == corev1.PodPending && len(p.Status.Conditions) == 0 }},
		"AsWorker":            {fixture.AsWorker, func(p *corev1.Pod) bool { return p.Labels[query.LabelRunner] == types.RunnerWorker }},
		"AsLeader":            {fixture.AsLeader, func(p *corev1.Pod) bool { return p.Labels[query.LabelRunner] == types.RunnerLeader }},
		"NoRevision":          {fixture.NoRevision, func(p *corev1.Pod) bool { _, has := p.Labels[query.LabelRevisionHash]; return !has }},
		"OnRevision":          {fixture.OnRevision(fixture.RevisionHashB), func(p *corev1.Pod) bool { return p.Labels[query.LabelRevisionHash] == fixture.RevisionHashB }},
		"WithPairingProtocol": {fixture.WithPairingProtocol("v2"), func(p *corev1.Pod) bool { return p.Labels[query.LabelPairingProtocol] == "v2" }},
		"GateAbsent":          {fixture.GateAbsent, func(p *corev1.Pod) bool { return condition(p, podreadiness.ConditionType) == nil }},
		"GateNotFolded": {fixture.GateNotFolded, func(p *corev1.Pod) bool {
			return condition(p, podreadiness.ConditionType).Status == corev1.ConditionTrue && condition(p, corev1.PodReady).Status == corev1.ConditionFalse
		}},
		"GateHeldBy": {fixture.GateHeldBy("writer", "key"), func(p *corev1.Pod) bool {
			gate := condition(p, podreadiness.ConditionType)
			return gate.Status == corev1.ConditionFalse && gate.Message == `[{"userAgent":"writer","key":"key"}]` && condition(p, corev1.PodReady).Status == corev1.ConditionFalse
		}},
		"NodeLost": {fixture.NodeLost, func(p *corev1.Pod) bool {
			return condition(p, corev1.PodReady).Status == corev1.ConditionFalse && condition(p, corev1.ContainersReady).Status == corev1.ConditionTrue
		}},
		"UnreadySince": {fixture.UnreadySince(at), func(p *corev1.Pod) bool {
			return condition(p, corev1.PodReady).Status == corev1.ConditionFalse && condition(p, corev1.ContainersReady).LastTransitionTime.Time.Equal(at) &&
				!p.Status.ContainerStatuses[0].Ready && condition(p, podreadiness.ConditionType).Status == corev1.ConditionTrue
		}},
		"NeverReady": {fixture.NeverReady, func(p *corev1.Pod) bool {
			return condition(p, podreadiness.ConditionType) == nil && condition(p, corev1.PodReady).LastTransitionTime.Time.Equal(p.CreationTimestamp.Time)
		}},
		"ContainerWaiting": {fixture.ContainerWaiting("ImagePullBackOff"), func(p *corev1.Pod) bool {
			w := p.Status.ContainerStatuses[0].State.Waiting
			return w != nil && w.Reason == "ImagePullBackOff" && condition(p, corev1.PodReady).LastTransitionTime.Time.Equal(fixture.Epoch.Add(-10*time.Minute))
		}},
		"ContainersUnready": {fixture.ContainersUnready, func(p *corev1.Pod) bool {
			return condition(p, corev1.ContainersReady).Status == corev1.ConditionFalse && condition(p, corev1.PodReady).Status == corev1.ConditionTrue
		}},
		"RestartedAt": {fixture.RestartedAt(at), func(p *corev1.Pod) bool {
			cs := p.Status.ContainerStatuses[0]
			return cs.RestartCount == 1 && cs.LastTerminationState.Terminated.FinishedAt.Time.Equal(at) && cs.State.Running.StartedAt.Time.Equal(at)
		}},
		"ExitedAt": {fixture.ExitedAt(at), func(p *corev1.Pod) bool {
			cs := p.Status.ContainerStatuses[0]
			return cs.State.Terminated != nil && cs.State.Terminated.FinishedAt.Time.Equal(at) && condition(p, corev1.PodReady).Status == corev1.ConditionFalse
		}},
		"CreatedAt": {fixture.CreatedAt(at), func(p *corev1.Pod) bool { return p.CreationTimestamp.Time.Equal(at) }},
		"ReadyWithoutTransitionTime": {fixture.ReadyWithoutTransitionTime, func(p *corev1.Pod) bool {
			return condition(p, corev1.PodReady).Status == corev1.ConditionTrue && condition(p, corev1.PodReady).LastTransitionTime.IsZero()
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if pod := fixture.Pod("a", tc.option); !tc.check(pod) {
				t.Errorf("%s did not shape the pod as its name says: %+v", name, pod.Status)
			}
		})
	}
}

func TestRunningNotReadyPod_RunsWithTheKubeletsUnreadyReason(t *testing.T) {
	since := fixture.Epoch.Add(-time.Minute)
	pod := fixture.RunningNotReadyPod("a", since)
	c := condition(pod, corev1.ContainersReady)
	if pod.Status.Phase != corev1.PodRunning || c == nil || c.Status != corev1.ConditionFalse || c.Reason != "ContainersNotReady" || !c.LastTransitionTime.Time.Equal(since) {
		t.Fatalf("pod = %+v, want Running with ContainersReady False (ContainersNotReady) since %s", pod.Status, since)
	}
	if pod.Status.ContainerStatuses[0].Ready || pod.Status.ContainerStatuses[0].State.Running == nil {
		t.Errorf("container = %+v, want running and not ready", pod.Status.ContainerStatuses[0])
	}
	if condition(pod, corev1.PodReady) != nil || condition(pod, podreadiness.ConditionType) != nil {
		t.Errorf("conditions = %+v, want neither PodReady nor the gate", pod.Status.Conditions)
	}
}

func TestUnschedulablePod_PendingWithTheSchedulersMessage(t *testing.T) {
	since := fixture.Epoch.Add(-time.Minute)
	pod := fixture.UnschedulablePod("a", "0/3 nodes are available", since)
	c := condition(pod, corev1.PodScheduled)
	if pod.Status.Phase != corev1.PodPending || c == nil || c.Status != corev1.ConditionFalse ||
		c.Reason != types.WaitingReasonUnschedulable || c.Message != "0/3 nodes are available" || !c.LastTransitionTime.Time.Equal(since) {
		t.Fatalf("pod = %+v, want Pending with PodScheduled False (%s) since %s", pod.Status, types.WaitingReasonUnschedulable, since)
	}
}
