package gangpack

import (
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

// podGroupLabel is the scheduler-plugins standard pod label naming a pod's
// PodGroup. It is an ecosystem standard (sigs.k8s.io/scheduler-plugins), not an
// OME convention, so it is used as-is — not made configurable.
const podGroupLabel = "scheduling.x-k8s.io/pod-group"

// gangInfo is everything the plugin needs to place and gate a gang, all declared
// by the workload (never global config): the gang's key, how many members must
// land together, which node label denotes its domain, and how long a member may
// wait at the gate for its siblings.
type gangInfo struct {
	key         string // namespace/podgroup-name
	uid         string // immutable PodGroup identity; prevents stale-name pin reuse
	minMember   int
	topologyKey string
	timeout     time.Duration
}

// podGroupReader resolves a PodGroup into its placement facts (minMember, the
// declared topology key, and the gate timeout). It is an interface so the plugin
// logic is unit-testable without a live PodGroup informer.
type podGroupReader interface {
	get(namespace, name string) (minMember int, topologyKey string, timeout time.Duration, uid string, found bool)
}

// podGroupNameOf returns the (namespace, name) of a pod's PodGroup — namespace
// is the pod's own, name is the standard pod-group label value — and false when
// the pod carries no pod-group label (not a gang member).
func podGroupNameOf(pod *v1.Pod) (namespace, name string, ok bool) {
	if pod == nil {
		return "", "", false
	}
	name = pod.Labels[podGroupLabel]
	if name == "" {
		return "", "", false
	}
	return pod.Namespace, name, true
}

// splitGangKey splits a gang key ("namespace/podgroup-name") back into its parts.
// ok is false for a malformed key (no separator, or an empty half).
func splitGangKey(key string) (namespace, name string, ok bool) {
	i := strings.IndexByte(key, '/')
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	return key[:i], key[i+1:], true
}

// resolveGang assembles a pod's gangInfo from its PodGroup. It returns false —
// meaning "not a domain-pinned gang, leave placement unconstrained" — when the
// pod is not a gang member, its PodGroup is absent, its minMember is
// non-positive, or the PodGroup declares no topology key (nothing to pin to).
func resolveGang(pod *v1.Pod, r podGroupReader) (gangInfo, bool) {
	ns, name, ok := podGroupNameOf(pod)
	if !ok {
		return gangInfo{}, false
	}
	minMember, topologyKey, timeout, uid, found := r.get(ns, name)
	if !found || minMember <= 0 || topologyKey == "" {
		return gangInfo{}, false
	}
	return gangInfo{key: ns + "/" + name, uid: uid, minMember: minMember, topologyKey: topologyKey, timeout: timeout}, true
}
