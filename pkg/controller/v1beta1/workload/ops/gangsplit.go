package ops

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// gangSplitRiskSeen tracks which (owner, Component) pairs have already
// received the GangSplitRisk Warning so the create path does not re-emit
// it on every pass. Keyed "<namespace>/<name>/<component>".
// Process-scoped — controller restart re-fires once, which is fine since
// operators re-read events on restart. Mirrors maybeNoGangSchedulerSeen.
var gangSplitRiskSeen sync.Map

// maybeWarnGangSplitRisk emits a one-shot Warning when a multi-node gang
// WORKER pod is about to be created with no co-location podAffinity at
// all. Such a gang's pods may land in separate network / NVLink / TPU
// topology domains, which breaks the tightly-coupled collectives a
// multi-node runtime needs (NCCL/RCCL/NIXL all-reduce, multi-host TPU
// sessions). Advisory only — it never blocks the create.
//
// It reads the ALREADY-RENDERED pod, so it inherits injectGangDomainAffinity's
// exact decision instead of re-deriving it (no logic drift):
//   - a resolved Component topologyKey means the injector added a required
//     term -> not at risk;
//   - a hand-written operator podAffinity is preserved on the pod → not
//     at risk;
//   - only a worker left with zero required podAffinity trips the warning.
//
// Accelerator-agnostic by construction: it inspects podAffinity, never the
// requested resource kind, so GPU/RDMA/TPU/any future gang are covered the
// same way. Dedup'd per (owner, Component) per process. nil-safe.
func maybeWarnGangSplitRisk(rec record.EventRecorder, input workload.ReconcileInput, plan workload.ComponentPlan, inst workload.InstancePlan, runner workload.RunnerPlan, pod *corev1.Pod) {
	// Only a multi-node gang WORKER can split. The leader is the domain
	// anchor (carries no co-location term by design); single-pod Instances
	// have nothing to co-locate.
	if runner.Name != "worker" || !instanceHasLeader(inst) {
		return
	}
	// A required podAffinity term — OME-injected or operator-supplied —
	// means the gang is already pinned to one domain.
	if pod == nil || hasAnyRequiredPodAffinity(pod) {
		return
	}
	target := eventTarget(input)
	if target == nil {
		return
	}
	key := fmt.Sprintf("%s/%s/%s", target.GetNamespace(), target.GetName(), plan.Component)
	if _, already := gangSplitRiskSeen.LoadOrStore(key, struct{}{}); already {
		return
	}
	if rec == nil {
		return
	}
	rec.Eventf(target, corev1.EventTypeWarning, string(workload.EventReasonGangSplitRisk),
		"OMENative component=%s is a multi-node gang but its worker pods have no co-location podAffinity: "+
			"the gang may be scheduled across separate network/NVLink/TPU topology domains, breaking the "+
			"tightly-coupled collectives a multi-node runtime needs (NCCL/RCCL/NIXL all-reduce, multi-host "+
			"TPU sessions). Set %s.topologyKey to the node label that identifies the required topology "+
			"domain, or declare a worker podAffinity.",
		plan.Component, plan.Component)
}

// hasAnyRequiredPodAffinity reports whether pod declares at least one
// required (hard) podAffinity term — the signal that some co-location
// constraint, OME-injected or operator-supplied, is in force.
func hasAnyRequiredPodAffinity(pod *corev1.Pod) bool {
	return pod.Spec.Affinity != nil &&
		pod.Spec.Affinity.PodAffinity != nil &&
		len(pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0
}

// resetGangSplitRiskSeen clears the per-process dedup map. Test-only
// helper (lowercase) so dedup tests can reset between cases.
func resetGangSplitRiskSeen() {
	gangSplitRiskSeen.Range(func(k, _ any) bool {
		gangSplitRiskSeen.Delete(k)
		return true
	})
}
