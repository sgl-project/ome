package types

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExpectations_SatisfiedWhenEmpty(t *testing.T) {
	e := NewExpectations()
	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("empty cache should be satisfied")
	}
}

func TestExpectations_BlocksUntilObserved(t *testing.T) {
	e := NewExpectations()
	e.ExpectCreates("ns", "isvc", ComponentEngine, 0, 2)

	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("should NOT be satisfied with 2 pending adds")
	}

	e.ObservedCreate("ns", "isvc", ComponentEngine, 0)
	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("should NOT be satisfied with 1 pending add still")
	}

	e.ObservedCreate("ns", "isvc", ComponentEngine, 0)
	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("should be satisfied after observing both creates")
	}
}

func TestExpectations_DeadlineForcesSatisfied(t *testing.T) {
	e := NewExpectations()
	e.ExpectCreates("ns", "isvc", ComponentEngine, 0, 1)
	// Manually expire the entry.
	e.mu.Lock()
	for _, v := range e.entries {
		v.Deadline = time.Now().Add(-1 * time.Second)
	}
	e.mu.Unlock()

	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("expired entry should be reported as satisfied")
	}
}

func TestExpectations_ScopedByKey(t *testing.T) {
	e := NewExpectations()
	e.ExpectCreates("ns", "isvc", ComponentEngine, 0, 1)
	// Different instance index — should be unaffected.
	if !e.Satisfied("ns", "isvc", ComponentEngine, 1) {
		t.Fatal("instance 1 should still be satisfied")
	}
	// Different component — should be unaffected.
	if !e.Satisfied("ns", "isvc", ComponentDecoder, 0) {
		t.Fatal("decoder instance 0 should still be satisfied")
	}
}

func TestExpectations_Forget(t *testing.T) {
	e := NewExpectations()
	e.ExpectCreates("ns", "isvc", ComponentEngine, 0, 3)
	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("should not be satisfied before Forget")
	}
	e.Forget("ns", "isvc", ComponentEngine, 0)
	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("Forget should clear the entry")
	}
}

func TestExpectations_DeletesTracked(t *testing.T) {
	e := NewExpectations()
	e.ExpectDeletes("ns", "isvc", ComponentEngine, 0, 1)
	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("should not be satisfied with pending delete")
	}
	e.ObservedDelete("ns", "isvc", ComponentEngine, 0)
	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("delete observation should satisfy")
	}
}

func int32p(v int32) *int32 { return &v }

func podWith(name string, phase corev1.PodPhase, css ...corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.PodStatus{Phase: phase, ContainerStatuses: css},
	}
}

func TestPodTermination_NilPod(t *testing.T) {
	if got := PodTermination(nil, metav1.Now()); got != nil {
		t.Fatalf("PodTermination(nil) = %+v, want nil", got)
	}
}

// A non-zero terminated exit code is the highest-precedence signal — the
// canonical crash the operator most wants to see (OOMKilled, exit 137).
func TestPodTermination_NonZeroTerminatedWins(t *testing.T) {
	pod := podWith("engine-0", corev1.PodFailed, corev1.ContainerStatus{
		Name: "main",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason:   "OOMKilled",
			ExitCode: 137,
			Message:  "out of memory",
		}},
	})
	got := PodTermination(pod, metav1.Now())
	if got == nil {
		t.Fatal("PodTermination = nil, want a record")
	}
	if got.PodName != "engine-0" || got.ContainerName != "main" || got.Reason != "OOMKilled" {
		t.Errorf("identity: %+v", got)
	}
	if got.ExitCode == nil || *got.ExitCode != 137 {
		t.Errorf("ExitCode: got %v want 137", got.ExitCode)
	}
	if got.Message != "out of memory" {
		t.Errorf("Message: got %q", got.Message)
	}
	if got.ShortString() != "pod engine-0 container main failed (OOMKilled, exit 137)" {
		t.Errorf("ShortString: %q", got.ShortString())
	}
}

// CrashLoopBackOff surfaces the crash in LastTerminationState while the
// live State is Waiting — the extractor must read LastTerminationState's
// non-zero exit code in preference to the bare waiting reason.
func TestPodTermination_CrashLoopReadsLastTerminationState(t *testing.T) {
	pod := podWith("engine-0", corev1.PodRunning, corev1.ContainerStatus{
		Name:  "main",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason:   "Error",
			ExitCode: 1,
		}},
	})
	got := PodTermination(pod, metav1.Now())
	if got == nil || got.Reason != "Error" || got.ExitCode == nil || *got.ExitCode != 1 {
		t.Fatalf("want Error/exit 1 from LastTerminationState, got %+v", got)
	}
}

// A terminal waiting state with no terminated history (ImagePullBackOff)
// yields a reason with a nil ExitCode and a "stuck" ShortString.
func TestPodTermination_TerminalWaitingNoExitCode(t *testing.T) {
	pod := podWith("engine-0", corev1.PodPending, corev1.ContainerStatus{
		Name:  "main",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "back-off pulling"}},
	})
	got := PodTermination(pod, metav1.Now())
	if got == nil {
		t.Fatal("want a record")
	}
	if got.Reason != "ImagePullBackOff" || got.ExitCode != nil {
		t.Errorf("want ImagePullBackOff/nil exit, got %+v", got)
	}
	if got.ShortString() != "pod engine-0 container main stuck (ImagePullBackOff)" {
		t.Errorf("ShortString: %q", got.ShortString())
	}
}

// A non-terminal waiting reason (ContainerCreating) is not a failure
// signal; with no other signal and a non-Failed phase, return nil.
func TestPodTermination_TransientWaitingIgnored(t *testing.T) {
	pod := podWith("engine-0", corev1.PodPending, corev1.ContainerStatus{
		Name:  "main",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
	})
	if got := PodTermination(pod, metav1.Now()); got != nil {
		t.Fatalf("ContainerCreating must not produce a record, got %+v", got)
	}
}

// Pod phase Failed with no per-container detail falls back to a
// pod-level PodFailed record.
func TestPodTermination_PodLevelFallback(t *testing.T) {
	pod := podWith("engine-0", corev1.PodFailed)
	pod.Status.Message = "node shutdown"
	got := PodTermination(pod, metav1.Now())
	if got == nil || got.Reason != "PodFailed" || got.ContainerName != "" {
		t.Fatalf("want pod-level PodFailed, got %+v", got)
	}
	if got.Message != "node shutdown" {
		t.Errorf("Message: got %q", got.Message)
	}
	if got.ShortString() != "pod engine-0 failed (PodFailed)" {
		t.Errorf("ShortString: %q", got.ShortString())
	}
}

// A Running pod with no termination signal yields nil (no false-positive
// capture during healthy operation).
func TestPodTermination_HealthyRunningNil(t *testing.T) {
	pod := podWith("engine-0", corev1.PodRunning, corev1.ContainerStatus{
		Name:  "main",
		Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	})
	if got := PodTermination(pod, metav1.Now()); got != nil {
		t.Fatalf("healthy running pod must yield nil, got %+v", got)
	}
}

// A healthy Running pod whose init container completed cleanly must
// still yield nil. Exit 0 is the normal resting state of every
// completed init container, so an unguarded exit-0 scan would report a
// termination for the majority of healthy pods and stamp a bogus
// LastFailure on the Instance.
func TestPodTermination_CompletedInitContainerNotAFailure(t *testing.T) {
	pod := podWith("engine-0", corev1.PodRunning, corev1.ContainerStatus{
		Name:  "main",
		Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	})
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "init-model",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason:   "Completed",
			ExitCode: 0,
		}},
	}}
	if got := PodTermination(pod, metav1.Now()); got != nil {
		t.Fatalf("a cleanly-completed init container is not a failure, got %+v", got)
	}
}

// The exit-0 scan still fires once the pod itself has Failed — that is
// the restartPolicy=Never sidecar case it exists for.
func TestPodTermination_ExitZeroCapturedOncePodFailed(t *testing.T) {
	pod := podWith("engine-0", corev1.PodFailed, corev1.ContainerStatus{
		Name: "sidecar",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason:   "Completed",
			ExitCode: 0,
		}},
	})
	got := PodTermination(pod, metav1.Now())
	if got == nil || got.ContainerName != "sidecar" {
		t.Fatalf("want the sidecar record, got %+v", got)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("want exit 0, got %+v", got.ExitCode)
	}
}

// Init-container failures are captured with the same precedence after
// regular containers.
func TestPodTermination_InitContainerCrash(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "engine-0"},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name: "init-model",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					Reason:   "Error",
					ExitCode: 2,
				}},
			}},
		},
	}
	got := PodTermination(pod, metav1.Now())
	if got == nil || got.ContainerName != "init-model" || got.ExitCode == nil || *got.ExitCode != 2 {
		t.Fatalf("want init-model/exit 2, got %+v", got)
	}
}

// kubelet occasionally leaves the terminated Reason blank; the extractor
// must still produce a non-empty reason ("Error") so the record is
// never reason-less.
func TestPodTermination_BlankReasonFallsBackToError(t *testing.T) {
	pod := podWith("engine-0", corev1.PodFailed, corev1.ContainerStatus{
		Name:  "main",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 139}},
	})
	got := PodTermination(pod, metav1.Now())
	if got == nil || got.Reason != "Error" {
		t.Fatalf("want fallback reason Error, got %+v", got)
	}
}

// PodTerminationWithReason fills a missing reason from the override but
// keeps container/exit detail when the extractor found it.
func TestPodTerminationWithReason(t *testing.T) {
	// No per-container detail at all → override supplies the whole record.
	bare := podWith("engine-0", corev1.PodPending)
	got := PodTerminationWithReason(bare, "ImagePullBackOff", metav1.Now())
	if got == nil || got.PodName != "engine-0" || got.Reason != "ImagePullBackOff" {
		t.Fatalf("override path: got %+v", got)
	}

	// Extractor already produced detail with a reason → override does not
	// clobber it.
	rich := podWith("engine-1", corev1.PodFailed, corev1.ContainerStatus{
		Name:  "main",
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}},
	})
	got = PodTerminationWithReason(rich, "ImagePullBackOff", metav1.Now())
	if got.Reason != "OOMKilled" || got.ExitCode == nil || *got.ExitCode != 137 {
		t.Fatalf("override must not clobber richer record, got %+v", got)
	}
}

func TestInstanceTermination_ShortStringNil(t *testing.T) {
	var t0 *InstanceTermination
	if got := t0.ShortString(); got != "" {
		t.Fatalf("nil ShortString = %q, want empty", got)
	}
}

func TestInstanceTermination_ShortStringUnknownPod(t *testing.T) {
	tm := &InstanceTermination{Reason: "OOMKilled", ExitCode: int32p(137)}
	if got := tm.ShortString(); got != "pod <unknown> failed (OOMKilled, exit 137)" {
		t.Fatalf("ShortString = %q", got)
	}
}

func TestMigrationPhaseAtOrPast_ManualChain(t *testing.T) {
	chain := []MigrationPhase{
		MigrationPhaseAccepted,
		MigrationPhaseSurgePending,
		MigrationPhaseSurgeReady,
		MigrationPhaseDraining,
		MigrationPhaseCompleted,
	}
	for i, p := range chain {
		for j, target := range chain {
			want := i >= j
			if got := MigrationPhaseAtOrPast(p, target); got != want {
				t.Errorf("MigrationPhaseAtOrPast(%s, %s) = %v, want %v", p, target, got, want)
			}
		}
	}
}

// An unrecognized phase must not read as "past everything". Ranking it
// at the top of the chain reports every advancement as already done and
// wedges the record permanently; ranking it at the bottom lets the
// executor drive it forward.
func TestMigrationPhaseAtOrPast_UnknownPhaseIsNotPastEverything(t *testing.T) {
	for _, unknown := range []MigrationPhase{"", "Bogus"} {
		if MigrationPhaseAtOrPast(unknown, MigrationPhaseAccepted) {
			t.Errorf("phase %q must not report as at-or-past Accepted", unknown)
		}
		if MigrationPhaseAtOrPast(unknown, MigrationPhaseCompleted) {
			t.Errorf("phase %q must not report as at-or-past Completed", unknown)
		}
		// The same holds with the unknown value as the target.
		if MigrationPhaseAtOrPast(MigrationPhaseAccepted, unknown) {
			t.Errorf("Accepted must not report as at-or-past target %q", unknown)
		}
		if MigrationPhaseAtOrPast(MigrationPhaseCompleted, unknown) {
			t.Errorf("Completed must not report as at-or-past target %q", unknown)
		}
		if MigrationPhaseAtOrPast(unknown, unknown) {
			t.Errorf("phase %q must not report as at-or-past itself", unknown)
		}
	}
}
