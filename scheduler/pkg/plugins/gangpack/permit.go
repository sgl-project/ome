package gangpack

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kube-scheduler/framework"
)

// sameGang reports whether a pod belongs to the given gang key (namespace/name).
func sameGang(pod *v1.Pod, gangKey string) bool {
	ns, name, ok := podGroupNameOf(pod)
	return ok && ns+"/"+name == gangKey
}

// Permit is the all-or-nothing gang gate. Each gang member waits here instead of
// binding immediately; when the arriving member makes the count reach minMember,
// every waiting sibling is released together and this member proceeds. Members
// wait up to the gang's timeout (its PodGroup ScheduleTimeoutSeconds); on timeout
// the framework rejects the pod and Unreserve unwinds the rest. A pod
// that is not a domain-pinned gang member is not gated.
func (g *GangPack) Permit(_ context.Context, state framework.CycleState, pod *v1.Pod, _ string) (*framework.Status, time.Duration) {
	pin := readPin(state)
	if pin == nil {
		if _, _, member := podGroupNameOf(pod); member {
			return framework.NewStatus(framework.Unschedulable,
				"labeled gang pod reached Permit without validated PreFilter state"), 0
		}
		return framework.NewStatus(framework.Success), 0
	}
	gang := pin.gang
	if gang.timeout <= 0 {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable,
			"gang permit timeout is not configured"), 0
	}
	g.rememberAttempt(pod, pin.commitment)
	g.activateGangMembers(gang)

	// Reserve has already converted this member's claim into assumed occupancy.
	// The commitment reaches zero exactly when minMember slots, including any
	// members adopted as already bound, have arrived. Waiting pods are assumed in
	// the scheduler snapshot, so combining a snapshot count with the waiting set
	// would count them twice and open the gate early.
	if !g.pins.CommittedIf(gang.key, pin.commitment) {
		gangGateTotal.WithLabelValues("wait").Inc()
		return framework.NewStatus(framework.Wait), gang.timeout
	}

	// The gang is complete: release every waiting sibling; this member proceeds.
	g.handle.IterateOverWaitingPods(func(wp framework.WaitingPod) {
		if sameGang(wp.GetPod(), gang.key) && g.attemptFor(wp.GetPod()) == pin.commitment {
			wp.Allow(Name)
		}
	})
	gangGateTotal.WithLabelValues("admit").Inc()
	return framework.NewStatus(framework.Success), 0
}

// activateGangMembers closes the queueing-hint race where a sibling Add event
// occurs while an earlier member is still in flight and that earlier member
// subsequently becomes Unschedulable waiting for the full template set. Once any
// member reaches Permit, the informer has the gang; force-activating its siblings
// makes the earlier member retry immediately instead of waiting for the periodic
// unschedulable flush.
//
// Unconditional and per-member: whenever any gang member reaches Permit, every
// live member is activated, regardless of whether the gate then admits or
// waits. This is also what lets an affinity-ordered gang (a worker whose
// Filter depends on its leader already being assumed) converge promptly on an
// otherwise quiet cluster, since an assumed-but-unbound pod produces no
// cluster event of its own for that worker to react to.
func (g *GangPack) activateGangMembers(gang gangInfo) {
	lister, ok := g.podLister.(gangMemberLister)
	if !ok || g.handle == nil {
		return
	}
	ns, name, ok := splitGangKey(gang.key)
	if !ok {
		return
	}
	pods, err := lister.gangPods(ns, name)
	if err != nil {
		return
	}
	activate := make(map[string]*v1.Pod, len(pods))
	for _, pod := range pods {
		if pod != nil {
			activate[pod.Namespace+"/"+pod.Name] = pod
		}
	}
	g.handle.Activate(klog.Background(), activate)
}
