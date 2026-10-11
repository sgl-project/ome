package reading_test

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

const podName = "engine-0-default-0"

// base is the Context most cases read under: the pass's clock, a one-minute
// unready grace, no availability window, the pod in rotation, no pairing
// transition.
func base() reading.Context {
	return reading.Context{
		Now:          now,
		UnreadyGrace: time.Minute,
		Rotation:     reading.Rotation{podName: {}},
	}
}

type levelCase struct {
	name string
	pod  *corev1.Pod
	ctx  reading.Context
	want bool
}

func runLevel(t *testing.T, level string, get func(reading.PodReading) bool, cases []levelCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := get(reading.ReadPod(tc.pod, tc.ctx)); got != tc.want {
				t.Fatalf("%s = %v, want %v", level, got, tc.want)
			}
		})
	}
}

func TestPodReading_Live(t *testing.T) {
	runLevel(t, "Live", func(p reading.PodReading) bool { return p.Live }, []levelCase{
		{name: "serving pod", pod: shape(podName), ctx: base(), want: true},
		{name: "pod still coming up", pod: shape(podName, neverReady), ctx: base(), want: true},
		{name: "pending pod", pod: shape(podName, pending), ctx: base(), want: true},
		{name: "being deleted", pod: shape(podName, terminating), ctx: base(), want: false},
		{name: "failed", pod: shape(podName, failed), ctx: base(), want: false},
		{name: "succeeded", pod: shape(podName, succeeded), ctx: base(), want: false},
	})
}

func TestPodReading_RuntimeReady(t *testing.T) {
	runLevel(t, "RuntimeReady", func(p reading.PodReading) bool { return p.RuntimeReady }, []levelCase{
		{name: "containers ready", pod: shape(podName), ctx: base(), want: true},
		{name: "containers ready, gate not written yet", pod: shape(podName, gateAbsent, gateNotFolded), ctx: base(), want: true},
		{name: "containers unready", pod: shape(podName, unreadySince(now.Add(-time.Second))), ctx: base(), want: false},
		{name: "being deleted with containers ready", pod: shape(podName, terminating), ctx: base(), want: false},
		{name: "failed with stale conditions", pod: shape(podName, failed), ctx: base(), want: false},
	})
}

func TestPodReading_Serving(t *testing.T) {
	runLevel(t, "Serving", func(p reading.PodReading) bool { return p.Serving }, []levelCase{
		{name: "PodReady and the gate", pod: shape(podName), ctx: base(), want: true},
		{name: "gate condition absent, PodReady true", pod: shape(podName, gateAbsent), ctx: base(), want: false},
		{name: "gate written, PodReady not folded yet", pod: shape(podName, gateNotFolded), ctx: base(), want: false},
		{name: "gate held by a drain", pod: shape(podName, gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1")), ctx: base(), want: false},
		{name: "node lost under a serving pod", pod: shape(podName, nodeLost), ctx: base(), want: false},
		{name: "being deleted", pod: shape(podName, terminating), ctx: base(), want: false},
		{name: "failed with stale conditions", pod: shape(podName, failed), ctx: base(), want: false},
	})
}

func TestPodReading_Routed(t *testing.T) {
	noRotation := base()
	noRotation.Rotation = nil
	runLevel(t, "Routed", func(p reading.PodReading) bool { return p.Routed }, []levelCase{
		{name: "routable pod in rotation", pod: shape(podName), ctx: base(), want: true},
		{name: "Ready but not in rotation", pod: shape(podName), ctx: noRotation, want: false},
		{name: "worker in the set", pod: shape(podName, asWorker), ctx: base(), want: false},
		{name: "no revision hash", pod: shape(podName, noRevision), ctx: base(), want: false},
		{name: "being deleted but still listed", pod: shape(podName, terminating), ctx: base(), want: false},
		{name: "in rotation while the gate is held", pod: shape(podName, gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1")), ctx: base(), want: true},
	})
}

func TestPodReading_Available(t *testing.T) {
	window := func(d time.Duration) reading.Context {
		ctx := base()
		ctx.AvailabilityWindow = d
		return ctx
	}
	runLevel(t, "Available", func(p reading.PodReading) bool { return p.Available }, []levelCase{
		{name: "no window: same as Serving", pod: shape(podName), ctx: window(0), want: true},
		{name: "no window, not serving", pod: shape(podName, nodeLost), ctx: window(0), want: false},
		{name: "Ready for an hour, ten-minute window", pod: shape(podName), ctx: window(10 * time.Minute), want: true},
		{name: "Ready for a minute, ten-minute window", pod: shape(podName, readySince(now.Add(-time.Minute))), ctx: window(10 * time.Minute), want: false},
		{name: "Ready exactly a window ago", pod: shape(podName, readySince(now.Add(-10*time.Minute))), ctx: window(10 * time.Minute), want: true},
		{name: "Ready with no transition time", pod: shape(podName, readyWithoutTransitionTime), ctx: window(10 * time.Minute), want: false},
		{name: "serving gate absent", pod: shape(podName, gateAbsent), ctx: window(10 * time.Minute), want: false},
	})
}

func TestPodReading_AvailableMatchesPodreadinessRule(t *testing.T) {
	for _, window := range []time.Duration{0, 30 * time.Minute, 2 * time.Hour} {
		for _, pod := range pods(
			shape(podName),
			shape(podName, readyWithoutTransitionTime),
			shape(podName, nodeLost),
			shape(podName, readySince(now.Add(-time.Hour))),
		) {
			ctx := base()
			ctx.AvailabilityWindow = window
			got := reading.ReadPod(pod, ctx).Available
			available, _ := podreadiness.IsPodAvailable(pod, int32(window/time.Second), now)
			if want := podreadiness.ReadyAndServing(pod) && available; got != want {
				t.Fatalf("window %s: Available = %v, podreadiness rule says %v", window, got, want)
			}
		}
	}
}

func TestPodReading_Pairable(t *testing.T) {
	pairing := func(inFlight bool, peer map[string]int32) reading.Context {
		ctx := base()
		ctx.Pairing = reading.Pairing{TransitionInFlight: inFlight, PeerServingByProtocol: peer}
		return ctx
	}
	runLevel(t, "Pairable", func(p reading.PodReading) bool { return p.Pairable }, []levelCase{
		{name: "no transition in flight", pod: shape(podName, protocol("v1")), ctx: pairing(false, nil), want: true},
		{name: "transition, pod has no protocol", pod: shape(podName), ctx: pairing(true, map[string]int32{"v2": 1}), want: true},
		{name: "transition, peer serves the same cohort", pod: shape(podName, protocol("v1")), ctx: pairing(true, map[string]int32{"v1": 1}), want: true},
		{name: "transition, peer serves the empty cohort", pod: shape(podName, protocol("v1")), ctx: pairing(true, map[string]int32{"": 2}), want: true},
		{name: "transition, peer serves another cohort only", pod: shape(podName, protocol("v1")), ctx: pairing(true, map[string]int32{"v2": 1}), want: false},
		{name: "transition, peer cohort count is zero", pod: shape(podName, protocol("v1")), ctx: pairing(true, map[string]int32{"v1": 0}), want: false},
		{name: "transition, no peer serving", pod: shape(podName, protocol("v1")), ctx: pairing(true, nil), want: false},
		{name: "not serving", pod: shape(podName, protocol("v1"), nodeLost), ctx: pairing(false, nil), want: false},
	})
}

func TestPodReading_KeepsTheShape(t *testing.T) {
	p := reading.ReadPod(shape(podName, asLeader, protocol("v1"), onRevision(hashB)), base())
	if p.Name != podName || p.Protocol != "v1" || p.RevisionHash != hashB || p.Runner != types.RunnerLeader || p.Worker {
		t.Fatalf("labels not kept: %+v", p)
	}
	if p.Terminal || p.Terminating || p.SurgeDraining || p.ParkedIn != "" {
		t.Fatalf("healthy pod reads a lifecycle flag: %+v", p)
	}
	if w := reading.ReadPod(shape(podName, asWorker), base()); !w.Worker {
		t.Fatalf("worker not read as one: %+v", w)
	}
	if f := reading.ReadPod(shape(podName, failed), base()); !f.Terminal || f.Terminating {
		t.Fatalf("failed pod: %+v", f)
	}
	if d := reading.ReadPod(shape(podName, terminating), base()); !d.Terminating || d.Terminal {
		t.Fatalf("deleting pod: %+v", d)
	}
	if s := reading.ReadPod(shape(podName, gateHeldBy(podreadiness.WriterUpdateSurgeDrain, "0-1")), base()); !s.SurgeDraining {
		t.Fatalf("surge-drained source: %+v", s)
	}
	if o := reading.ReadPod(shape(podName, gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1")), base()); o.SurgeDraining {
		t.Fatalf("in-place drain read as the surge drain: %+v", o)
	}
	if c := reading.ReadPod(shape(podName, parkedIn("CrashLoopBackOff")), base()); c.ParkedIn != "CrashLoopBackOff" {
		t.Fatalf("parked pod: %+v", c)
	}
}

func TestPodReading_OutOfService(t *testing.T) {
	noGrace := base()
	noGrace.UnreadyGrace = 0
	for _, tc := range []struct {
		name          string
		pod           *corev1.Pod
		ctx           reading.Context
		out           bool
		unready, dark bool
	}{
		{name: "serving", pod: shape(podName), ctx: base()},
		{name: "parked in CrashLoopBackOff", pod: shape(podName, parkedIn("CrashLoopBackOff")), ctx: base(), out: true, unready: true, dark: true},
		{name: "parked, no grace configured", pod: shape(podName, parkedIn("ImagePullBackOff")), ctx: noGrace, out: true},
		{name: "served, then unready past the grace", pod: shape(podName, unreadySince(now.Add(-2*time.Minute))), ctx: base(), out: true, unready: true, dark: true},
		{name: "served, then unready inside the grace", pod: shape(podName, unreadySince(now.Add(-30*time.Second))), ctx: base()},
		{name: "unready past the grace, no grace configured", pod: shape(podName, unreadySince(now.Add(-2*time.Minute))), ctx: noGrace},
		{name: "never promoted, unready past the grace", pod: shape(podName, neverReady), ctx: base(), dark: true},
		{name: "drained, then unready past the grace", pod: shape(podName, gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1"), unreadySince(now.Add(-2*time.Minute))), ctx: base(), dark: true},
		{name: "node lost", pod: shape(podName, nodeLost), ctx: base()},
		{name: "parked but being deleted", pod: shape(podName, parkedIn("CrashLoopBackOff"), terminating), ctx: base()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := reading.ReadPod(tc.pod, tc.ctx)
			if p.OutOfService() != tc.out || p.UnreadyPastGrace != tc.unready || p.ContainersUnreadyPastGrace != tc.dark {
				t.Fatalf("OutOfService=%v UnreadyPastGrace=%v ContainersUnreadyPastGrace=%v, want %v %v %v",
					p.OutOfService(), p.UnreadyPastGrace, p.ContainersUnreadyPastGrace, tc.out, tc.unready, tc.dark)
			}
		})
	}
}

func TestRead_Counts(t *testing.T) {
	ctx := base()
	ctx.AvailabilityWindow = 10 * time.Minute
	ctx.Rotation = reading.Rotation{"a": {}, "b": {}, "e": {}}
	ctx.Pairing = reading.Pairing{TransitionInFlight: true, PeerServingByProtocol: map[string]int32{"v1": 1}}
	r := reading.Read(pods(
		shape("a", protocol("v1")),                                    // every level
		shape("b", protocol("v2"), readySince(now.Add(-time.Minute))), // serving, routed, not available, not pairable
		shape("c", gateNotFolded),                                     // live, runtime ready only, not in rotation
		shape("d", neverReady),                                        // live only
		shape("e", terminating),                                       // nothing
		nil,
		shape("f", failed), // nothing
	), 2, ctx)
	want := reading.Counts{Pods: 6, Live: 4, RuntimeReady: 3, Serving: 2, Routed: 2, Available: 1, Pairable: 1}
	if r.Counts != want {
		t.Fatalf("Counts = %+v, want %+v", r.Counts, want)
	}
	if len(r.Pods) != 6 || r.Desired != 2 {
		t.Fatalf("Read kept %d readings for desired %d", len(r.Pods), r.Desired)
	}
}

type verdictCase struct {
	name    string
	pods    []*corev1.Pod
	desired int32
	want    bool
}

func runVerdict(t *testing.T, verdict string, ctx reading.Context, get func(reading.InstanceReading) bool, cases []verdictCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := get(reading.Read(tc.pods, tc.desired, ctx)); got != tc.want {
				t.Fatalf("%s = %v, want %v", verdict, got, tc.want)
			}
		})
	}
}

func TestInstanceReading_FullyServing(t *testing.T) {
	surgeSource := shape("engine-0-default-1", gateHeldBy(podreadiness.WriterUpdateSurgeDrain, "0-1"))
	runVerdict(t, "FullyServing", base(), reading.InstanceReading.FullyServing, []verdictCase{
		{name: "two serving pods, two desired", pods: pods(shape("a"), shape("b")), desired: 2, want: true},
		{name: "two serving pods, three desired", pods: pods(shape("a"), shape("b")), desired: 3},
		{name: "one pod not serving", pods: pods(shape("a"), shape("b", nodeLost)), desired: 2},
		{name: "gate written, PodReady not folded yet", pods: pods(shape("a", gateNotFolded)), desired: 1},
		{name: "terminal pod beside a serving one", pods: pods(shape("a"), shape("b", failed)), desired: 1},
		{name: "deleting pod beside a serving one", pods: pods(shape("a"), shape("b", terminating)), desired: 1, want: true},
		{name: "surge source draining beside its serving replacement", pods: pods(shape(podName), surgeSource), desired: 1, want: true},
		{name: "in-place drained pod beside a serving one", pods: pods(shape("a"), shape("b", gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1"))), desired: 1},
		{name: "serving worker beside a serving leader", pods: pods(shape("a", asLeader), shape("b", asWorker)), desired: 2, want: true},
		{name: "desired zero", pods: pods(shape("a")), desired: 0},
		{name: "no pods", pods: nil, desired: 1},
	})
}

func TestInstanceReading_ServesNothing(t *testing.T) {
	parked := shape("a", parkedIn("CrashLoopBackOff"))
	runVerdict(t, "ServesNothing", base(), reading.InstanceReading.ServesNothing, []verdictCase{
		{name: "parked pod", pods: pods(parked), desired: 1, want: true},
		{name: "terminal pod alone", pods: pods(shape("a", failed)), desired: 1, want: true},
		{name: "parked beside terminal", pods: pods(parked, shape("b", failed)), desired: 2, want: true},
		{name: "served, then unready past the grace", pods: pods(shape("a", unreadySince(now.Add(-2*time.Minute)))), desired: 1, want: true},
		{name: "served, then unready inside the grace", pods: pods(shape("a", unreadySince(now.Add(-30*time.Second)))), desired: 1},
		{name: "serving pod", pods: pods(shape("a")), desired: 1},
		{name: "parked beside serving", pods: pods(parked, shape("b")), desired: 2},
		{name: "still coming up", pods: pods(shape("a", neverReady)), desired: 1},
		{name: "node lost under a serving pod", pods: pods(shape("a", nodeLost)), desired: 1},
		{name: "parked leader beside a serving worker", pods: pods(shape("a", asLeader, parkedIn("CrashLoopBackOff")), shape("b", asWorker)), desired: 2, want: true},
		{name: "serving leader beside a parked worker", pods: pods(shape("a", asLeader), shape("b", asWorker, parkedIn("CrashLoopBackOff"))), desired: 2},
		{name: "only a serving worker is left", pods: pods(shape("b", asWorker)), desired: 2, want: true},
		{name: "only a deleting pod", pods: pods(shape("a", parkedIn("CrashLoopBackOff"), terminating)), desired: 1},
		{name: "no pods", pods: nil, desired: 1},
	})
}

func TestInstanceReading_OutOfRotation(t *testing.T) {
	runVerdict(t, "OutOfRotation", base(), reading.InstanceReading.OutOfRotation, []verdictCase{
		{name: "still coming up", pods: pods(shape("a", neverReady)), desired: 1, want: true},
		{name: "served, then unready inside the grace", pods: pods(shape("a", unreadySince(now.Add(-time.Second)))), desired: 1, want: true},
		{name: "terminal pod alone", pods: pods(shape("a", failed)), desired: 1, want: true},
		{name: "only a serving worker is left", pods: pods(shape("b", asWorker)), desired: 2, want: true},
		{name: "serving pod", pods: pods(shape("a")), desired: 1},
		{name: "one serving beside one coming up", pods: pods(shape("a"), shape("b", neverReady)), desired: 2},
		{name: "only a deleting pod", pods: pods(shape("a", terminating)), desired: 1},
		{name: "no pods", pods: nil, desired: 1},
	})
}

func TestInstanceReading_Dark(t *testing.T) {
	darkFor := unreadySince(now.Add(-2 * time.Minute))
	runVerdict(t, "Dark", base(), reading.InstanceReading.Dark, []verdictCase{
		{name: "served, then unready past the grace", pods: pods(shape("a", darkFor)), desired: 1, want: true},
		{name: "drained, then unready past the grace", pods: pods(shape("a", gateHeldBy(podreadiness.WriterUpdateInPlace, "0-1"), darkFor)), desired: 1, want: true},
		{name: "never promoted, unready past the grace", pods: pods(shape("a", neverReady)), desired: 1, want: true},
		{name: "unready inside the grace", pods: pods(shape("a", unreadySince(now.Add(-time.Second)))), desired: 1},
		{name: "serving pod", pods: pods(shape("a")), desired: 1},
		{name: "one dark beside one serving", pods: pods(shape("a", darkFor), shape("b")), desired: 2},
		{name: "dark leader beside a serving worker", pods: pods(shape("a", asLeader, darkFor), shape("b", asWorker)), desired: 2, want: true},
		{name: "serving leader beside a dark worker", pods: pods(shape("a", asLeader), shape("b", asWorker, darkFor)), desired: 2},
		{name: "terminal pod alone", pods: pods(shape("a", failed)), desired: 1},
		{name: "only a worker, dark", pods: pods(shape("b", asWorker, darkFor)), desired: 2},
		{name: "no pods", pods: nil, desired: 1},
	})
}

func TestInstanceReading_UnavailableOnParkedMember(t *testing.T) {
	parked := shape("b", asWorker, parkedIn("CrashLoopBackOff"))
	runVerdict(t, "UnavailableOnParkedMember", base(), reading.InstanceReading.UnavailableOnParkedMember, []verdictCase{
		{name: "one serving, one parked, two desired", pods: pods(shape("a", asLeader), parked), desired: 2, want: true},
		{name: "one serving, one unready past the grace, two desired", pods: pods(shape("a", asLeader), shape("b", asWorker, unreadySince(now.Add(-2*time.Minute)))), desired: 2, want: true},
		{name: "one serving, one parked, one desired", pods: pods(shape("a", asLeader), parked), desired: 1},
		{name: "one serving, one coming up, two desired", pods: pods(shape("a", asLeader), shape("b", asWorker, neverReady)), desired: 2},
		{name: "one serving, one unready inside the grace", pods: pods(shape("a", asLeader), shape("b", asWorker, unreadySince(now.Add(-time.Second)))), desired: 2},
		{name: "one serving, one terminal, two desired", pods: pods(shape("a", asLeader), shape("b", asWorker, failed)), desired: 2},
		{name: "one serving, one parked but deleting", pods: pods(shape("a", asLeader), shape("b", asWorker, parkedIn("CrashLoopBackOff"), terminating)), desired: 2},
		{name: "both serving", pods: pods(shape("a", asLeader), shape("b", asWorker)), desired: 2},
	})
}

func TestInstanceReading_PairableServing(t *testing.T) {
	ctx := base()
	ctx.Pairing = reading.Pairing{TransitionInFlight: true, PeerServingByProtocol: map[string]int32{"v1": 1}}
	r := reading.Read(pods(
		shape("a", protocol("v1")),
		shape("b", protocol("v2")),
		shape("c"),
		shape("d", protocol("v1"), nodeLost),
	), 4, ctx)
	if got := r.PairableServing(); got != 2 {
		t.Fatalf("PairableServing = %d, want 2", got)
	}
	ctx.Pairing.TransitionInFlight = false
	if got := reading.Read(pods(shape("a", protocol("v1")), shape("b", protocol("v2"))), 2, ctx).PairableServing(); got != 2 {
		t.Fatalf("PairableServing with no transition = %d, want 2", got)
	}
}

// A pod with no conditions at all, as the apiserver hands out a pod the
// kubelet has not reported on yet.
func TestPodReading_NoConditions(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName}, Status: corev1.PodStatus{Phase: corev1.PodPending}}
	p := reading.ReadPod(pod, base())
	if !p.Live || p.RuntimeReady || p.Serving || p.Routed || p.Available || p.Pairable || p.OutOfService() {
		t.Fatalf("bare pending pod: %+v", p)
	}
}
