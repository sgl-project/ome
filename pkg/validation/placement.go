package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ValidatePlacement checks spec.placement at admission — the parts the CRD
// schema cannot express. The mode enum (Single|All|Split) is already enforced by
// the CRD, so this covers:
//
//   - the requirement / cluster-selector strings must be valid label selectors.
//     A malformed selector is otherwise not caught until reconcile, where it
//     silently yields no candidates and the ISVC sits Pending with no obvious
//     cause; failing fast at admission turns that into a clear error.
//   - the Split knobs must be self-consistent (non-negative, replicas >= 1 when
//     set, and minReplicasPerCluster not above a set maxReplicasPerCluster —
//     otherwise a home could never reach the anti-sliver floor).
//
// No-op when spec.placement is unset (single-cluster / annotation-driven ISVCs).
func ValidatePlacement(spec *v1beta1.InferenceServiceSpec) error {
	p := spec.Placement
	if p == nil {
		return nil
	}
	for _, sel := range []struct{ field, value string }{
		{"requirements", p.Requirements},
		{"clusterSelector", p.ClusterSelector},
	} {
		if sel.value == "" {
			continue
		}
		if _, err := labels.Parse(sel.value); err != nil {
			return fmt.Errorf("spec.placement.%s is not a valid label selector %q: %w", sel.field, sel.value, err)
		}
	}
	s := p.Split
	if s == nil {
		return nil
	}
	if s.Replicas != nil && *s.Replicas < 1 {
		return fmt.Errorf("spec.placement.split.replicas must be >= 1 when set, got %d", *s.Replicas)
	}
	if s.MaxReplicasPerCluster < 0 {
		return fmt.Errorf("spec.placement.split.maxReplicasPerCluster must be >= 0, got %d", s.MaxReplicasPerCluster)
	}
	if s.MinReplicasPerCluster < 0 {
		return fmt.Errorf("spec.placement.split.minReplicasPerCluster must be >= 0, got %d", s.MinReplicasPerCluster)
	}
	if s.MaxReplicasPerCluster > 0 && s.MinReplicasPerCluster > s.MaxReplicasPerCluster {
		return fmt.Errorf("spec.placement.split.minReplicasPerCluster (%d) must not exceed maxReplicasPerCluster (%d)",
			s.MinReplicasPerCluster, s.MaxReplicasPerCluster)
	}
	return nil
}
