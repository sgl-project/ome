package gangpack

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
	"sigs.k8s.io/ome/scheduler/pkg/topology"
)

// PostFilter unwinds a gang when every candidate failed before Reserve. Domain
// planning intentionally evaluates only resources and hard node constraints;
// other framework filters (for example volumes, ports, and inter-pod affinity)
// may still reject the domain. Without this hook no Reserve/Unreserve callback
// runs and the pin's exclusive reservation could persist indefinitely.
func (g *GangPack) PostFilter(ctx context.Context, state framework.CycleState, pod *v1.Pod, _ framework.NodeToStatusReader) (*framework.PostFilterResult, *framework.Status) {
	if readPin(state) == nil {
		return nil, framework.NewStatus(framework.Unschedulable)
	}
	g.releaseAttempt(readPin(state), pod, true)
	return nil, framework.NewStatus(framework.Unschedulable, "gang reservation released after all candidate nodes were filtered")
}

func failedDomainKey(gang gangInfo) string {
	return gang.key + "\x00" + gang.uid
}

func (g *GangPack) markFailedDomain(gang gangInfo, domain placement.Domain) {
	g.failedMu.Lock()
	defer g.failedMu.Unlock()
	if g.failedDomains == nil {
		g.failedDomains = make(map[string]sets.Set[placement.Domain])
	}
	key := failedDomainKey(gang)
	if g.failedDomains[key] == nil {
		g.failedDomains[key] = sets.New[placement.Domain]()
	}
	g.failedDomains[key].Insert(domain)
}

func (g *GangPack) clearFailedDomains(gang gangInfo) {
	g.failedMu.Lock()
	delete(g.failedDomains, failedDomainKey(gang))
	g.failedMu.Unlock()
}

// withoutFailedDomains returns a copy with failed domains removed. The caller
// retries the unfiltered map after this set is exhausted, producing a stable
// best-fit order rather than selecting the same failing domain forever.
func (g *GangPack) withoutFailedDomains(gang gangInfo, free topology.FreeByDomain) (topology.FreeByDomain, bool) {
	g.failedMu.Lock()
	failed := g.failedDomains[failedDomainKey(gang)].Clone()
	g.failedMu.Unlock()
	if len(failed) == 0 {
		return free, false
	}
	out := make(topology.FreeByDomain, len(free))
	for name, capacity := range free {
		if !failed.Has(placement.Domain{TopologyKey: gang.topologyKey, Name: name}) {
			out[name] = capacity
		}
	}
	return out, true
}
