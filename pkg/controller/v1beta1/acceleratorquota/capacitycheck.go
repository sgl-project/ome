package acceleratorquota

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/tree"
)

// capacityViolations reports nodes whose budget claims more of a (resource,
// flavor) pair than this cluster has.
//
// The comparison is against the high-water mark rather than the instantaneous
// allocatable, because capacity dips for reasons that say nothing about
// entitlement — a drain, a rolling reboot, a device plugin restarting. The mark
// already has the hysteresis band folded into it, so the band is not applied a
// second time here.
//
// It is a whole-tree check for the same reason containment is: a budget can
// become unaffordable without anyone editing it, by the fleet shrinking
// underneath it, so it has to be re-checked by something that sees the cluster
// rather than only at admission.
//
// capacity is the reserved root's observed capacity. Nil or empty means nothing
// has been measured yet and NO violation is reported — an unmeasured cluster
// must not read as a cluster with no hardware, which would freeze every tenant
// on the first pass after a restart.
func capacityViolations(nodes []*tree.Node, capacity []v1beta1.AcceleratorCapacityStatus) tree.Violations {
	if len(capacity) == 0 {
		return nil
	}

	marks := make(map[string]resource.Quantity, len(capacity))
	for _, c := range capacity {
		marks[budgetKey(c.ResourceName, c.ResourceFlavor)] = c.HighWaterMark
	}

	var vs tree.Violations
	for _, n := range nodes {
		for _, b := range n.Quota.Spec.Budgets {
			key := budgetKey(b.ResourceName, b.ResourceFlavor)
			mark, measured := marks[key]
			if !measured {
				// The pair is budgeted but contributes no measured capacity:
				// either no node carries that flavor, or the resource is absent
				// from the configured accelerator list. Both mean the budget is
				// written against hardware this cluster cannot account for, and
				// saying so is the whole point of naming resources in full.
				vs = append(vs, tree.Violation{
					Node:    n.Name(),
					Reason:  v1beta1.AcceleratorQuotaReasonCapacityExceeded,
					Subject: key,
					Message: fmt.Sprintf("budget %s is %s but no capacity is measured for that pair",
						key, b.Nominal.String()),
				})
				continue
			}
			if b.Nominal.Cmp(mark) > 0 {
				vs = append(vs, tree.Violation{
					Node:    n.Name(),
					Reason:  v1beta1.AcceleratorQuotaReasonCapacityExceeded,
					Subject: key,
					Message: fmt.Sprintf("budget %s is %s but the cluster's peak capacity is %s",
						key, b.Nominal.String(), mark.String()),
				})
			}
		}
	}
	return vs
}

// budgetKey renders a (resource, flavor) pair. It mirrors the tree package's
// own unexported key by hand, because Violation.Subject is what makes two
// violations comparable across builds: a capacity violation and a containment
// violation on the same budget have to name that budget identically or a reader
// cannot tell they are about the same thing.
func budgetKey(resourceName, flavor string) string {
	return resourceName + " on " + flavor
}
