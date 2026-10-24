package reading

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/drain"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// RoutedServices names the routed Services the pods can be members of, one
// per revision among the routable pods (query.RoutedServiceForPod), sorted
// and without repeats, so a caller can list exactly those EndpointSlices
// before building the rotation set. Workers and pods without a revision
// hash name no Service.
func RoutedServices(owner string, component types.ComponentType, pods []*corev1.Pod) []string {
	seen := map[string]struct{}{}
	for _, pod := range pods {
		if name := query.RoutedServiceForPod(owner, component, pod); name != "" {
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RotationFromEndpointSlices is the rotation set a caller passes in Context:
// the names of the pods the slices publish for new traffic
// (drain.EndpointAvailable, so a terminating endpoint is left out), across
// every slice given. Endpoints that do not point at a pod are skipped.
func RotationFromEndpointSlices(slices []discoveryv1.EndpointSlice) Rotation {
	out := Rotation{}
	for i := range slices {
		for _, ep := range slices[i].Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.Kind != "" && ep.TargetRef.Kind != "Pod" {
				continue
			}
			if !drain.EndpointAvailable(ep) {
				continue
			}
			out[ep.TargetRef.Name] = struct{}{}
		}
	}
	return out
}
