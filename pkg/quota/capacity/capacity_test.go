package capacity

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var accelerators = Options{AcceleratorResources: []string{
	"google.com/tpu", "nvidia.com/gpu", "amd.com/gpu",
}}

type nodeOpt func(*corev1.Node)

func ready(n *corev1.Node) {
	n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
}

func notReady(n *corev1.Node) {
	n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
}

func cordoned(n *corev1.Node) { n.Spec.Unschedulable = true }

func labels(kv map[string]string) nodeOpt {
	return func(n *corev1.Node) { n.Labels = kv }
}

func alloc(kv map[string]string) nodeOpt {
	return func(n *corev1.Node) {
		n.Status.Allocatable = corev1.ResourceList{}
		for k, v := range kv {
			n.Status.Allocatable[corev1.ResourceName(k)] = resource.MustParse(v)
		}
	}
}

// tainted carries the taint an accelerator pool routinely wears. Availability
// must ignore it, so every fixture that models a real pool sets it.
func tainted(n *corev1.Node) {
	n.Spec.Taints = []corev1.Taint{{
		Key: "google.com/tpu", Value: "present", Effect: corev1.TaintEffectNoSchedule,
	}}
}

func node(name string, opts ...nodeOpt) corev1.Node {
	n := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ready(&n)
	for _, o := range opts {
		o(&n)
	}
	return n
}

func qty(s string) resource.Quantity { return resource.MustParse(s) }

var (
	tpu7x = Flavor{Name: "tpu7x", NodeLabels: map[string]string{"accelerator": "tpu7x"}}
	gb300 = Flavor{Name: "gb300", NodeLabels: map[string]string{"accelerator": "gb300"}}
)

func TestSum(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []corev1.Node
		flavors []Flavor
		opts    *Options // nil uses the accelerator suffixes
		want    Result
	}{
		{
			name: "sums allocatable across a pool",
			nodes: []corev1.Node{
				node("n1", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), tainted),
				node("n2", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), tainted),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x", Allocatable: qty("8"), Nodes: 2},
			}},
		},
		{
			// A dedicated pool is tainted so only tolerating workloads land on
			// it. Treating that as unavailable would report the whole fleet as
			// empty, which is why availability deliberately ignores taints.
			name: "a taint does not make a node unavailable",
			nodes: []corev1.Node{
				node("n1", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), tainted),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x", Allocatable: qty("4"), Nodes: 1},
			}},
		},
		{
			// Parked capacity is still installed. Sizing a budget against it
			// would over-promise, and hiding it would make a cordoned rack look
			// like hardware that never existed.
			name: "cordoned and not-Ready capacity is counted separately",
			nodes: []corev1.Node{
				node("up", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				node("cordoned", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), cordoned),
				node("down", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), notReady),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Allocatable: qty("4"), Nodes: 1,
				Unavailable: qty("8"), UnavailableNodes: 2,
			}}},
		},
		{
			// A node with no Ready condition at all has not reported in.
			name: "a node with no Ready condition is unavailable",
			nodes: []corev1.Node{
				func() corev1.Node {
					n := node("silent", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}))
					n.Status.Conditions = nil
					return n
				}(),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{{
				ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x",
				Unavailable: qty("4"), UnavailableNodes: 1,
			}}},
		},
		{
			name: "capacity is keyed by (resource, flavor)",
			nodes: []corev1.Node{
				node("t", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				node("g", labels(map[string]string{"accelerator": "gb300"}), alloc(map[string]string{"nvidia.com/gpu": "8"})),
			},
			flavors: []Flavor{tpu7x, gb300},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x", Allocatable: qty("4"), Nodes: 1},
				{ResourceName: "nvidia.com/gpu", ResourceFlavor: "gb300", Allocatable: qty("8"), Nodes: 1},
			}},
		},
		{
			name: "several listed vendors on one node",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "gb300"}),
					alloc(map[string]string{"amd.com/gpu": "2", "nvidia.com/gpu": "2"})),
			},
			flavors: []Flavor{gb300},
			want: Result{Capacities: []Capacity{
				{ResourceName: "amd.com/gpu", ResourceFlavor: "gb300", Allocatable: qty("2"), Nodes: 1},
				{ResourceName: "nvidia.com/gpu", ResourceFlavor: "gb300", Allocatable: qty("2"), Nodes: 1},
			}},
		},
		{
			// The cost of exact names over a pattern, recorded rather than
			// discovered: a vendor nobody listed is invisible here. It is not
			// silent overall — a budget against unmeasured hardware exceeds a
			// capacity of zero and says so — but this is where it starts.
			name: "an unlisted vendor contributes nothing",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "tpu7x"}),
					alloc(map[string]string{"habana.ai/gaudi": "8", "google.com/tpu": "4"})),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x", Allocatable: qty("4"), Nodes: 1},
			}},
		},
		{
			name: "non-accelerator resources are ignored",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "tpu7x"}),
					alloc(map[string]string{"cpu": "64", "memory": "256Gi", "google.com/tpu": "4"})),
			},
			flavors: []Flavor{tpu7x},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "tpu7x", Allocatable: qty("4"), Nodes: 1},
			}},
		},
		{
			name: "a node advertising zero chips contributes nothing",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "0"})),
			},
			flavors: []Flavor{tpu7x},
			want:    Result{},
		},
		{
			// Either a missing ResourceFlavor or a labelling mistake, and both
			// look like a shrinking fleet unless the node is named.
			name: "chips on a node no flavor claims are reported",
			nodes: []corev1.Node{
				node("orphan", labels(map[string]string{"accelerator": "h100"}), alloc(map[string]string{"nvidia.com/gpu": "8"})),
			},
			flavors: []Flavor{tpu7x, gb300},
			want: Result{Unattributed: []Unattributed{
				{Node: "orphan", ResourceName: "nvidia.com/gpu", Quantity: qty("8"), Reason: ReasonNoFlavor},
			}},
		},
		{
			// Counting it into both would authorize a budget twice the size of
			// the hardware, so it is counted into neither and surfaced.
			name: "a node two flavors claim equally is counted nowhere",
			nodes: []corev1.Node{
				node("both", labels(map[string]string{"accelerator": "tpu7x", "zone": "a"}),
					alloc(map[string]string{"google.com/tpu": "4"})),
			},
			flavors: []Flavor{
				{Name: "by-accelerator", NodeLabels: map[string]string{"accelerator": "tpu7x"}},
				{Name: "by-zone", NodeLabels: map[string]string{"zone": "a"}},
			},
			want: Result{Unattributed: []Unattributed{
				{Node: "both", ResourceName: "google.com/tpu", Quantity: qty("4"), Reason: ReasonAmbiguous},
			}},
		},
		{
			// Specificity breaks the tie, so a catch-all can coexist with
			// precise flavors instead of making every node ambiguous.
			name: "the more specific flavor wins",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "tpu7x", "zone": "a"}),
					alloc(map[string]string{"google.com/tpu": "4"})),
			},
			flavors: []Flavor{
				{Name: "catch-all"},
				{Name: "precise", NodeLabels: map[string]string{"accelerator": "tpu7x", "zone": "a"}},
			},
			want: Result{Capacities: []Capacity{
				{ResourceName: "google.com/tpu", ResourceFlavor: "precise", Allocatable: qty("4"), Nodes: 1},
			}},
		},
		{
			name: "a flavor with no labels catches what nothing else claims",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "h100"}), alloc(map[string]string{"nvidia.com/gpu": "8"})),
			},
			flavors: []Flavor{{Name: "catch-all"}},
			want: Result{Capacities: []Capacity{
				{ResourceName: "nvidia.com/gpu", ResourceFlavor: "catch-all", Allocatable: qty("8"), Nodes: 1},
			}},
		},
		{
			// Absent config disables rather than guessing which resources are
			// accelerators.
			name: "no configured suffixes derives nothing",
			nodes: []corev1.Node{
				node("n", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
			},
			flavors: []Flavor{tpu7x},
			opts:    &Options{},
			want:    Result{},
		},
		{
			name:    "no nodes is not an error",
			flavors: []Flavor{tpu7x},
			want:    Result{},
		},
		{
			name: "no flavors leaves every chip unattributed",
			nodes: []corev1.Node{
				node("n", alloc(map[string]string{"google.com/tpu": "4"})),
			},
			want: Result{Unattributed: []Unattributed{
				{Node: "n", ResourceName: "google.com/tpu", Quantity: qty("4"), Reason: ReasonNoFlavor},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := accelerators
			if tc.opts != nil {
				opts = *tc.opts
			}
			got, err := Sum(tc.nodes, tc.flavors, opts)
			if err != nil {
				t.Fatalf("Sum() = %v", err)
			}
			if diff := cmp.Diff(tc.want, got, cmp.Comparer(quantityEqual)); diff != "" {
				t.Errorf("Sum() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Both slices are documented as sorted, and a caller diffing two snapshots
// relies on it — an unordered result reports churn on every pass. Map iteration
// is what is being pinned, so each row needs more than one entry sharing a
// first sort key, and the input order is deliberately not the wanted order.
func TestSumOrdersItsOutput(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []corev1.Node
		flavors []Flavor
		// wantCapacities and wantUnattributed are the keys each slice should be
		// sorted by, rendered so a mismatch reads as an order rather than a
		// struct dump.
		wantCapacities   []string
		wantUnattributed []string
	}{
		{
			name: "capacities sort by resource then flavor",
			nodes: []corev1.Node{
				node("z", labels(map[string]string{"accelerator": "gb300"}), alloc(map[string]string{"nvidia.com/gpu": "8"})),
				node("a", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				// Same resource as "a", different flavor: the tie-break.
				node("m", labels(map[string]string{"accelerator": "tpu6e"}), alloc(map[string]string{"google.com/tpu": "2"})),
			},
			flavors: []Flavor{
				gb300, tpu7x,
				{Name: "tpu6e", NodeLabels: map[string]string{"accelerator": "tpu6e"}},
			},
			wantCapacities: []string{
				"google.com/tpu|tpu6e", "google.com/tpu|tpu7x", "nvidia.com/gpu|gb300",
			},
		},
		{
			name: "unattributed sorts by node then resource",
			nodes: []corev1.Node{
				node("y-orphan", alloc(map[string]string{"nvidia.com/gpu": "1"})),
				// Two resources on one node: the tie-break.
				node("b-orphan", alloc(map[string]string{"nvidia.com/gpu": "1", "google.com/tpu": "1"})),
			},
			wantUnattributed: []string{
				"b-orphan|google.com/tpu", "b-orphan|nvidia.com/gpu", "y-orphan|nvidia.com/gpu",
			},
		},
		{
			name: "both slices sort independently in one pass",
			nodes: []corev1.Node{
				node("z", labels(map[string]string{"accelerator": "gb300"}), alloc(map[string]string{"nvidia.com/gpu": "8"})),
				node("a", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				node("y-orphan", alloc(map[string]string{"nvidia.com/gpu": "1"})),
				node("b-orphan", alloc(map[string]string{"google.com/tpu": "1"})),
			},
			flavors:          []Flavor{gb300, tpu7x},
			wantCapacities:   []string{"google.com/tpu|tpu7x", "nvidia.com/gpu|gb300"},
			wantUnattributed: []string{"b-orphan|google.com/tpu", "y-orphan|nvidia.com/gpu"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Sum(tc.nodes, tc.flavors, accelerators)
			if err != nil {
				t.Fatalf("Sum() = %v", err)
			}

			var capOrder []string
			for _, c := range got.Capacities {
				capOrder = append(capOrder, c.ResourceName+"|"+c.ResourceFlavor)
			}
			if diff := cmp.Diff(tc.wantCapacities, capOrder); diff != "" {
				t.Errorf("capacity order mismatch (-want +got):\n%s", diff)
			}

			var unattOrder []string
			for _, u := range got.Unattributed {
				unattOrder = append(unattOrder, u.Node+"|"+u.ResourceName)
			}
			if diff := cmp.Diff(tc.wantUnattributed, unattOrder); diff != "" {
				t.Errorf("unattributed order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// A caller error rather than a data condition: two flavors of the same name
// would make the result ambiguous in a way no node list could explain.
func TestSumRejectsMalformedFlavors(t *testing.T) {
	tests := []struct {
		name    string
		flavors []Flavor
		wantErr string
	}{
		{
			name:    "a duplicate flavor name",
			flavors: []Flavor{{Name: "tpu7x"}, {Name: "tpu7x"}},
			wantErr: "declared more than once",
		},
		{
			name:    "a flavor with no name",
			flavors: []Flavor{{NodeLabels: map[string]string{"a": "b"}}},
			wantErr: "no name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Sum(nil, tc.flavors, accelerators)
			if err == nil {
				t.Fatalf("Sum() = nil, want an error naming %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Sum() = %q, want it to name %q", err, tc.wantErr)
			}
		})
	}
}

// Sum must not disturb the node list it was handed. Callers pass an informer's
// objects, so a Quantity accumulated in place would corrupt the shared cache —
// and the corruption compounds, because every later pass sums the already-summed
// value. The rows are the shapes where an in-place Add is easiest to write by
// accident.
func TestSumDoesNotMutateInput(t *testing.T) {
	tests := []struct {
		name  string
		nodes []corev1.Node
	}{
		{
			name: "several nodes summing into one entry",
			nodes: []corev1.Node{
				node("n1", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				node("n2", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
			},
		},
		{
			name: "mixed scales, where the sum reformats the quantity",
			nodes: []corev1.Node{
				node("n1", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "1500m"})),
				node("n2", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "2Gi"})),
			},
		},
		{
			name: "one node feeding both the available and unavailable totals",
			nodes: []corev1.Node{
				node("up", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"})),
				node("down", labels(map[string]string{"accelerator": "tpu7x"}), alloc(map[string]string{"google.com/tpu": "4"}), cordoned),
			},
		},
		{
			name: "a node whose chips go unattributed",
			nodes: []corev1.Node{
				node("orphan", alloc(map[string]string{"google.com/tpu": "4"})),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := map[string]string{}
			for i := range tc.nodes {
				q := tc.nodes[i].Status.Allocatable["google.com/tpu"]
				before[tc.nodes[i].Name] = q.String()
			}

			if _, err := Sum(tc.nodes, []Flavor{tpu7x}, accelerators); err != nil {
				t.Fatalf("Sum() = %v", err)
			}

			after := map[string]string{}
			for i := range tc.nodes {
				q := tc.nodes[i].Status.Allocatable["google.com/tpu"]
				after[tc.nodes[i].Name] = q.String()
			}
			if diff := cmp.Diff(before, after); diff != "" {
				t.Errorf("Sum mutated the caller's nodes (-before +after):\n%s", diff)
			}
		})
	}
}

// Quantity carries unexported cached state, so compare on value.
func quantityEqual(a, b resource.Quantity) bool { return a.Cmp(b) == 0 }
