package gangpack

import (
	v1 "k8s.io/api/core/v1"
	resourcehelper "k8s.io/component-helpers/resource"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"
	"k8s.io/kube-scheduler/framework"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

// The scheduler bakes in no workload conventions and no fabric values. Gang
// membership + size come from the standard scheduler-plugins PodGroup; the domain
// label is declared per-workload on the PodGroup (see gang.go); and "is a node
// free for this gang" is inferred from the GANG POD'S OWN resource requests — the
// pod already declares what it needs, so the scheduler is never told an
// accelerator resource name.

// domainOf returns the node's domain — the value of the given topology label — or
// "" when the node has no such label (not part of any domain).
func domainOf(node *v1.Node, topologyKey string) string {
	if node == nil {
		return ""
	}
	return node.Labels[topologyKey]
}

// nodeFitsPod mirrors the resource and hard node-placement constraints that can
// invalidate a domain choice before the framework's regular filters run. The
// whole-node gang model assigns at most one forming member to each node, but the
// node still has to fit every requested resource and one more pod.
func nodeFitsPod(ni framework.NodeInfo, pod *v1.Pod) bool {
	if ni == nil || ni.Node() == nil || ni.GetAllocatable() == nil || ni.GetRequested() == nil || pod == nil {
		return false
	}
	node := ni.Node()
	if !nodeSchedulable(node, pod) {
		return false
	}
	if pod.Spec.NodeName != "" && pod.Spec.NodeName != node.Name {
		return false
	}
	match, err := nodeaffinity.GetRequiredNodeAffinity(pod).Match(node)
	if err != nil || !match {
		return false
	}
	req := schedulerframework.NewResource(resourcehelper.PodRequests(pod, resourcehelper.PodResourcesOptions{}))
	allocatable, requested := ni.GetAllocatable(), ni.GetRequested()
	if allocatable.GetMilliCPU()-requested.GetMilliCPU() < req.GetMilliCPU() ||
		allocatable.GetMemory()-requested.GetMemory() < req.GetMemory() ||
		allocatable.GetEphemeralStorage()-requested.GetEphemeralStorage() < req.GetEphemeralStorage() ||
		allocatable.GetAllowedPodNumber() <= len(ni.GetPods()) {
		return false
	}
	for name, quantity := range req.ScalarResources {
		if allocatable.GetScalarResources()[name]-requested.GetScalarResources()[name] < quantity {
			return false
		}
	}
	return true
}

// boundGangPlacement is the result of scanning the node snapshot for pods
// already placed (assumed or bound) that share the pod's PodGroup: how many
// were found and the domain they sit in (the domain of the last such node
// seen; a healthy gang is single-domain). This is how the plugin recovers a
// gang whose in-memory pin was lost (scheduler restart / leader failover):
// the truth of where a gang lives is where its members already are. The pod
// being scheduled is not yet in any node's pod list, so it never counts
// itself.
type boundGangPlacement struct {
	domain string
	count  int
	split  bool
	names  map[string]bool
}

// inspectBoundGangMembers also detects an already-split gang. Silently adopting
// one arbitrary domain would undercount the other domains and make the split
// worse after scheduler failover.
func inspectBoundGangMembers(nodeInfos []framework.NodeInfo, pod *v1.Pod, topologyKey string) boundGangPlacement {
	ns, pgName, ok := podGroupNameOf(pod)
	if !ok {
		return boundGangPlacement{}
	}
	result := boundGangPlacement{names: make(map[string]bool)}
	for _, info := range nodeInfos {
		n := info.Node()
		if n == nil {
			continue
		}
		dom := domainOf(n, topologyKey)
		for _, pi := range info.GetPods() {
			mp := pi.GetPod()
			if mp != nil && mp.Namespace == ns && mp.Labels[podGroupLabel] == pgName {
				result.names[mp.Name] = true
				if dom == "" {
					result.split = true
					result.count++
					continue
				}
				if result.domain != "" && result.domain != dom {
					result.split = true
				}
				result.domain = dom
				result.count++
			}
		}
	}
	return result
}

func boundGangMembers(nodeInfos []framework.NodeInfo, pod *v1.Pod, topologyKey string) (domain string, count int) {
	result := inspectBoundGangMembers(nodeInfos, pod, topologyKey)
	return result.domain, result.count
}

// placedInDomain counts a gang's members (namespace + pod-group name) occupying
// nodes of the given domain in the snapshot. Unlike boundGangMembers it matches a
// gang by its resolved key rather than a live pod object, so the reservation
// reconciler can count a pinned gang's real footprint without a pod in hand.
func placedInDomain(nodeInfos []framework.NodeInfo, namespace, pgName, topologyKey, domain string) int {
	count := 0
	for _, info := range nodeInfos {
		n := info.Node()
		if n == nil || domainOf(n, topologyKey) != domain {
			continue
		}
		for _, pi := range info.GetPods() {
			mp := pi.GetPod()
			if mp != nil && mp.Namespace == namespace && mp.Labels[podGroupLabel] == pgName {
				count++
			}
		}
	}
	return count
}

// nodeSchedulable reports whether the gang pod could actually be placed on this
// node right now — beyond having the accelerator free, the node must be
// schedulable: not cordoned (spec.unschedulable), and its NoSchedule/NoExecute
// taints tolerated by the pod. Without this check, freeByDomain would count a
// cordoned or tainted node as free, so Choose could pin a domain whose nodes the
// framework's own Filter (NodeUnschedulable / TaintToleration) then rejects —
// wedging the gang there (pinStale keeps the pin because the domain still looks
// "free") while genuinely schedulable domains sit unused.
func nodeSchedulable(node *v1.Node, pod *v1.Pod) bool {
	if node == nil || node.Spec.Unschedulable {
		return false
	}
	_, untolerated := v1helper.FindMatchingUntoleratedTaint(klog.Background(), node.Spec.Taints, pod.Spec.Tolerations, func(t *v1.Taint) bool {
		return t.Effect == v1.TaintEffectNoSchedule || t.Effect == v1.TaintEffectNoExecute
	}, false)
	return !untolerated
}

// freeByDomain is the homogeneous convenience projection used by focused tests
// and tracing. Planning uses feasibleByDomain with every remaining member.
func freeByDomain(nodeInfos []framework.NodeInfo, topologyKey string, pod *v1.Pod) topology.FreeByDomain {
	out := topology.FreeByDomain{}
	for _, info := range nodeInfos {
		dom := domainOf(info.Node(), topologyKey)
		if dom == "" {
			continue
		}
		out[dom] += 0 // register the domain even if all nodes are occupied
		if nodeFitsPod(info, pod) {
			out[dom]++
		}
	}
	return out
}

// feasibleByDomain returns a best-fit score only for domains that can place all
// remaining gang templates at once. Feasibility is a bipartite matching problem:
// templates may have different requests, selectors, affinities, and tolerations,
// so aggregate free-node counts are insufficient.
func feasibleByDomain(nodeInfos []framework.NodeInfo, topologyKey string, pods []*v1.Pod) topology.FreeByDomain {
	return feasibleAtLeastByDomain(nodeInfos, topologyKey, pods, len(pods))
}

// feasibleAtLeastByDomain is the minMember-aware form of feasibleByDomain. The
// first pod is the current scheduling cycle and must fit; among surplus siblings,
// any subset large enough to reach need may supply the rest of the gang.
func feasibleAtLeastByDomain(nodeInfos []framework.NodeInfo, topologyKey string, pods []*v1.Pod, need int) topology.FreeByDomain {
	byDomain := make(map[string][]framework.NodeInfo)
	for _, info := range nodeInfos {
		if domain := domainOf(info.Node(), topologyKey); domain != "" {
			byDomain[domain] = append(byDomain[domain], info)
		}
	}
	out := make(topology.FreeByDomain, len(byDomain))
	for domain, nodes := range byDomain {
		eligible := 0
		for _, node := range nodes {
			for _, pod := range pods {
				if nodeFitsPod(node, pod) {
					eligible++
					break
				}
			}
		}
		if currentCanJoinMatching(pods, nodes, need) {
			out[domain] = eligible
		} else {
			out[domain] = 0
		}
	}
	return out
}

// currentCanJoinMatching requires pods[0] to occupy one node and proves that at
// least need-1 distinct surplus members can occupy distinct remaining nodes.
func currentCanJoinMatching(pods []*v1.Pod, nodes []framework.NodeInfo, need int) bool {
	if need <= 0 || len(pods) < need || len(nodes) < need {
		return false
	}
	for i, node := range nodes {
		if !nodeFitsPod(node, pods[0]) {
			continue
		}
		remaining := make([]framework.NodeInfo, 0, len(nodes)-1)
		remaining = append(remaining, nodes[:i]...)
		remaining = append(remaining, nodes[i+1:]...)
		if maxGangMatching(pods[1:], remaining, need-1) >= need-1 {
			return true
		}
	}
	return false
}

// gangMatchesNodes reports whether every pod can be assigned to a distinct node.
func gangMatchesNodes(pods []*v1.Pod, nodes []framework.NodeInfo) bool {
	return maxGangMatching(pods, nodes, len(pods)) == len(pods)
}

// maxGangMatching returns the maximum number of pod templates that can occupy
// distinct nodes, stopping once limit matches have been found.
func maxGangMatching(pods []*v1.Pod, nodes []framework.NodeInfo, limit int) int {
	if limit <= 0 || len(pods) == 0 || len(nodes) == 0 {
		return 0
	}
	matchedPod := make([]int, len(nodes))
	for i := range matchedPod {
		matchedPod[i] = -1
	}
	var assign func(int, []bool) bool
	assign = func(podIndex int, seen []bool) bool {
		for nodeIndex, node := range nodes {
			if seen[nodeIndex] || !nodeFitsPod(node, pods[podIndex]) {
				continue
			}
			seen[nodeIndex] = true
			if matchedPod[nodeIndex] == -1 || assign(matchedPod[nodeIndex], seen) {
				matchedPod[nodeIndex] = podIndex
				return true
			}
		}
		return false
	}
	matches := 0
	for podIndex := range pods {
		if assign(podIndex, make([]bool, len(nodes))) {
			matches++
			if matches == limit {
				break
			}
		}
	}
	return matches
}
