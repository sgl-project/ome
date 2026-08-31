package acceleratorquota

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/tree"
)

func capStatus(resourceName, flavor, mark string) v1beta1.AcceleratorCapacityStatus {
	return v1beta1.AcceleratorCapacityStatus{
		ResourceName:   resourceName,
		ResourceFlavor: flavor,
		HighWaterMark:  resource.MustParse(mark),
	}
}

func gpuBudget(flavor, nominal string) v1beta1.AcceleratorBudget {
	return v1beta1.AcceleratorBudget{
		ResourceName:   "nvidia.com/gpu",
		ResourceFlavor: flavor,
		Nominal:        resource.MustParse(nominal),
	}
}

// nodesFor builds a tree and returns its nodes, so the check is exercised over
// the same shape the reconciler hands it.
func nodesFor(t *testing.T, quotas ...*v1beta1.AcceleratorQuota) []*tree.Node {
	t.Helper()
	items := make([]v1beta1.AcceleratorQuota, 0, len(quotas))
	for _, q := range quotas {
		items = append(items, *q)
	}
	built, _, err := tree.Build(items, tree.Options{RootName: rootName})
	if err != nil {
		t.Fatalf("tree.Build() error = %v", err)
	}
	return built.Nodes()
}

// A budget can become unaffordable without anyone editing it, by the fleet
// shrinking underneath it, so this is re-checked every pass. Each row is a way
// the check could be wrong in a direction nobody would notice: silently
// tolerating an over-budget tenant, or freezing a whole fleet over capacity
// that was simply never measured.
func TestCapacityViolations(t *testing.T) {
	tests := []struct {
		name     string
		quotas   []*v1beta1.AcceleratorQuota
		capacity []v1beta1.AcceleratorCapacityStatus
		want     []string
	}{
		{
			name: "a budget within the mark is fine",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, ""),
				leaf("team-a", rootName, gpuBudget("a100", "8")),
			},
			capacity: []v1beta1.AcceleratorCapacityStatus{capStatus("nvidia.com/gpu", "a100", "16")},
			want:     []string{},
		},
		{
			// Equal is affordable. Off-by-one here would freeze a fleet that is
			// exactly fully allocated, which is the normal end state.
			name: "a budget exactly at the mark is fine",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, ""),
				leaf("team-a", rootName, gpuBudget("a100", "16")),
			},
			capacity: []v1beta1.AcceleratorCapacityStatus{capStatus("nvidia.com/gpu", "a100", "16")},
			want:     []string{},
		},
		{
			name: "a budget above the mark is reported",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, ""),
				leaf("team-a", rootName, gpuBudget("a100", "24")),
			},
			capacity: []v1beta1.AcceleratorCapacityStatus{capStatus("nvidia.com/gpu", "a100", "16")},
			want:     []string{"team-a"},
		},
		{
			// The promise made by the --accelerator-resources flag help and by
			// values.yaml: a pair nothing measures is a budget against hardware
			// this cluster cannot account for, and it must not pass silently.
			name: "a budget for an unmeasured pair is reported",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, ""),
				leaf("team-a", rootName, gpuBudget("no-such-flavor", "8")),
			},
			capacity: []v1beta1.AcceleratorCapacityStatus{capStatus("nvidia.com/gpu", "a100", "16")},
			want:     []string{"team-a"},
		},
		{
			// Nothing measured yet -- a restart, or derivation switched off.
			// Treating that as "no hardware" would freeze every tenant on the
			// first pass, which is the worst possible reading of missing data.
			name: "no measured capacity reports nothing",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, ""),
				leaf("team-a", rootName, gpuBudget("a100", "9999")),
			},
			capacity: nil,
			want:     []string{},
		},
		{
			name: "every over-budget node is named, not just the first",
			quotas: []*v1beta1.AcceleratorQuota{
				cohort(rootName, "", gpuBudget("a100", "64")),
				leaf("team-a", rootName, gpuBudget("a100", "32")),
				leaf("team-b", rootName, gpuBudget("a100", "32")),
			},
			capacity: []v1beta1.AcceleratorCapacityStatus{capStatus("nvidia.com/gpu", "a100", "16")},
			want:     []string{rootName, "team-a", "team-b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := capacityViolations(nodesFor(t, tc.quotas...), tc.capacity)

			names := []string{}
			for _, v := range got {
				if v.Reason != v1beta1.AcceleratorQuotaReasonCapacityExceeded {
					t.Errorf("reason = %q, want %q", v.Reason,
						v1beta1.AcceleratorQuotaReasonCapacityExceeded)
				}
				if v.Subject == "" {
					t.Errorf("violation on %q has no subject; two violations on one node "+
						"are indistinguishable without it", v.Node)
				}
				names = append(names, v.Node)
			}
			sort.Strings(names)

			if diff := cmp.Diff(tc.want, names); diff != "" {
				t.Errorf("capacityViolations() nodes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
