package shares

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Quantities carry unexported state and remember how they were written, so two
// that mean the same number are compared on Cmp rather than on their fields.
var cmpQuantity = cmp.Comparer(func(a, b resource.Quantity) bool { return a.Cmp(b) == 0 })

func w(cluster, capacity string) Weight {
	return Weight{Cluster: cluster, Capacity: resource.MustParse(capacity)}
}

func s(cluster, nominal string) Share {
	return Share{Cluster: cluster, Nominal: resource.MustParse(nominal)}
}

// The split has to land on exact numbers, not merely close ones, and it has to
// land on the SAME numbers every time. Both are asserted here against whole
// results rather than sums, so a split that totals correctly while handing the
// wrong cluster the remainder still fails.
func TestProportional(t *testing.T) {
	tests := []struct {
		name    string
		total   string
		weights []Weight
		want    []Share
	}{
		{
			name:    "an even basis splits evenly",
			total:   "100",
			weights: []Weight{w("a", "1"), w("b", "1")},
			want:    []Share{s("a", "50"), s("b", "50")},
		},
		{
			name:    "shares follow the basis, not the cluster count",
			total:   "120",
			weights: []Weight{w("a", "3"), w("b", "1")},
			want:    []Share{s("a", "90"), s("b", "30")},
		},
		{
			// 10/3 each. Floors are 3,3,3 and one unit is left over, so exactly
			// one cluster gets 4 — the sum is 10, never 9 and never 12.
			name:    "a unit that division cannot place goes to one cluster",
			total:   "10",
			weights: []Weight{w("a", "1"), w("b", "1"), w("c", "1")},
			want:    []Share{s("a", "4"), s("b", "3"), s("c", "3")},
		},
		{
			// Remainders are 2/3, 2/3, 2/3 — a three-way tie. The lexicographic
			// tiebreak is what makes the winner predictable; without it this
			// depends on slice order.
			name:    "an exact tie is broken by cluster name",
			total:   "11",
			weights: []Weight{w("c", "1"), w("b", "1"), w("a", "1")},
			want:    []Share{s("a", "4"), s("b", "4"), s("c", "3")},
		},
		{
			// The larger remainder wins outright: a gets 8.57 -> 8 r4, b gets
			// 3.43 -> 3 r3 (over a basis of 7), so a takes the spare unit.
			name:    "the largest remainder takes the spare unit",
			total:   "12",
			weights: []Weight{w("a", "5"), w("b", "2")},
			want:    []Share{s("a", "9"), s("b", "3")},
		},
		{
			name:    "a single cluster takes the whole total",
			total:   "64",
			weights: []Weight{w("only", "1")},
			want:    []Share{s("only", "64")},
		},
		{
			// Present at zero rather than omitted: a cluster with no hardware is
			// still part of the fleet and its projection should say zero, not
			// go missing.
			name:    "a cluster with no capacity gets a share of zero",
			total:   "10",
			weights: []Weight{w("a", "1"), w("idle", "0")},
			want:    []Share{s("a", "10"), s("idle", "0")},
		},
		{
			name:    "a total of zero splits into zeroes",
			total:   "0",
			weights: []Weight{w("a", "3"), w("b", "1")},
			want:    []Share{s("a", "0"), s("b", "0")},
		},
		{
			// The budget's own unit decides. A whole chip cannot be subdivided,
			// so one cluster gets it and the others get none -- a third of an
			// accelerator each would admit nothing anywhere.
			name:    "a whole unit is never split into fractions",
			total:   "1",
			weights: []Weight{w("a", "1"), w("b", "1"), w("c", "1")},
			want:    []Share{s("a", "1"), s("b", "0"), s("c", "0")},
		},
		{
			// The same total written in milli, which is how cpu is written, and
			// now thirds are exactly what is wanted.
			name:    "a budget written in milli splits into thousandths",
			total:   "1000m",
			weights: []Weight{w("a", "1"), w("b", "1"), w("c", "1")},
			want:    []Share{s("a", "334m"), s("b", "333m"), s("c", "333m")},
		},
		{
			// Large enough that MilliValue would overflow int64, which is why
			// the arithmetic is on big integers.
			name:    "binary quantities far past an int64 of milli-units",
			total:   "16Pi",
			weights: []Weight{w("a", "1"), w("b", "1")},
			want:    []Share{s("a", "8Pi"), s("b", "8Pi")},
		},
		{
			// The basis is finer than the total and does not drag the answer
			// down with it: the weights set the 3:1 ratio, the budget's own unit
			// sets the granularity, so the result is whole numbers.
			name:    "a finer basis sets the ratio, not the unit",
			total:   "10",
			weights: []Weight{w("a", "300m"), w("b", "100m")},
			want:    []Share{s("a", "8"), s("b", "2")},
		},
		{
			name:    "output is ordered by cluster, whatever order the input came in",
			total:   "3",
			weights: []Weight{w("z", "1"), w("m", "1"), w("a", "1")},
			want:    []Share{s("a", "1"), s("m", "1"), s("z", "1")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Proportional(resource.MustParse(tc.total), tc.weights)
			if err != nil {
				t.Fatalf("Proportional() error = %v", err)
			}
			if diff := cmp.Diff(tc.want, got, cmpQuantity); diff != "" {
				t.Errorf("Proportional() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// What cannot be resolved must say so. Each of these is a case where inventing
// an answer would be worse than refusing: a silent even split, a share for a
// cluster named twice, or a negative allowance would all materialize into Kueue
// and admit work against a number nobody authorized.
func TestProportionalRejects(t *testing.T) {
	tests := []struct {
		name    string
		total   string
		weights []Weight
		wantErr string
	}{
		{
			name:    "no clusters at all",
			total:   "10",
			weights: nil,
			wantErr: "no clusters",
		},
		{
			// An even split is a different policy. Guessing it here would give
			// an admin who asked for Proportional something they did not ask
			// for, on the pass where their fleet reported no capacity.
			name:    "a basis that is entirely zero",
			total:   "10",
			weights: []Weight{w("a", "0"), w("b", "0")},
			wantErr: "nothing to be proportional to",
		},
		{
			name:    "the same cluster twice",
			total:   "10",
			weights: []Weight{w("a", "1"), w("a", "2")},
			wantErr: "appears twice",
		},
		{
			name:    "a cluster with no name",
			total:   "10",
			weights: []Weight{w("", "1")},
			wantErr: "no cluster name",
		},
		{
			name:    "a negative total",
			total:   "-10",
			weights: []Weight{w("a", "1")},
			wantErr: "is negative",
		},
		{
			name:    "a negative capacity",
			total:   "10",
			weights: []Weight{w("a", "1"), w("b", "-1")},
			wantErr: "negative capacity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Proportional(resource.MustParse(tc.total), tc.weights)
			if err == nil {
				t.Fatalf("Proportional() = %v, want an error naming %q", got, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Proportional() = %q, want it to name %q", err, tc.wantErr)
			}
		})
	}
}

// The properties the split is for, over inputs chosen to stress the arithmetic
// rather than to read well. A table of expected values cannot state "sums to
// the total" — only checking it over many shapes can.
func TestProportionalProperties(t *testing.T) {
	cases := []struct {
		name    string
		total   string
		weights []Weight
	}{
		{name: "prime total over a prime basis", total: "97", weights: []Weight{w("a", "7"), w("b", "11"), w("c", "13")}},
		{name: "total smaller than the cluster count", total: "2", weights: []Weight{w("a", "1"), w("b", "1"), w("c", "1"), w("d", "1")}},
		{name: "one dominant cluster", total: "1000", weights: []Weight{w("big", "999"), w("small", "1")}},
		{name: "wildly mixed scales", total: "7", weights: []Weight{w("a", "1m"), w("b", "1Ki"), w("c", "2")}},
		{name: "many equal clusters", total: "101", weights: []Weight{
			w("a", "1"), w("b", "1"), w("c", "1"), w("d", "1"), w("e", "1"),
			w("f", "1"), w("g", "1"), w("h", "1"), w("i", "1"), w("j", "1"),
		}},
		{name: "milli total over a binary basis", total: "1500m", weights: []Weight{w("a", "3Gi"), w("b", "1Gi")}},
		{name: "zeroes alongside real capacity", total: "50", weights: []Weight{w("a", "0"), w("b", "5"), w("c", "0"), w("d", "5")}},
		{name: "a total that divides exactly", total: "64", weights: []Weight{w("a", "1"), w("b", "1"), w("c", "1"), w("d", "1")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total := resource.MustParse(tc.total)

			// Copied before the call so a mutation of the caller's slice shows
			// up as a difference afterwards.
			before := append([]Weight(nil), tc.weights...)

			got, err := Proportional(total, tc.weights)
			if err != nil {
				t.Fatalf("Proportional() error = %v", err)
			}

			if sum := Sum(got); sum.Cmp(total) != 0 {
				t.Errorf("shares sum to %s, want exactly %s (%+v)", sum.String(), total.String(), got)
			}

			if len(got) != len(tc.weights) {
				t.Errorf("got %d shares, want one per cluster (%d)", len(got), len(tc.weights))
			}
			for _, share := range got {
				if share.Nominal.Sign() < 0 {
					t.Errorf("cluster %s got a negative share %s", share.Cluster, share.Nominal.String())
				}
			}
			for i := 1; i < len(got); i++ {
				if got[i-1].Cluster >= got[i].Cluster {
					t.Errorf("shares are not ordered by cluster: %s before %s", got[i-1].Cluster, got[i].Cluster)
				}
			}

			// Same inputs, same answer — including which cluster took the
			// remainder. A split that reshuffled would rewrite every projected
			// CR in the fleet on a pass that changed nothing.
			again, err := Proportional(total, tc.weights)
			if err != nil {
				t.Fatalf("second Proportional() error = %v", err)
			}
			if diff := cmp.Diff(got, again, cmpQuantity); diff != "" {
				t.Errorf("the split is not deterministic (-first +second):\n%s", diff)
			}

			if diff := cmp.Diff(before, tc.weights, cmpQuantity); diff != "" {
				t.Errorf("the caller's weights were mutated (-before +after):\n%s", diff)
			}
		})
	}
}

// A cluster with more capacity is never handed less quota than one with less.
// Largest-remainder can move a single unit between near-equal clusters, so this
// is asserted as an ordering over the basis rather than as exact values.
func TestProportionalIsMonotonic(t *testing.T) {
	weights := []Weight{w("small", "1"), w("medium", "5"), w("large", "20")}
	got, err := Proportional(resource.MustParse("260"), weights)
	if err != nil {
		t.Fatalf("Proportional() error = %v", err)
	}

	byCluster := map[string]resource.Quantity{}
	for _, share := range got {
		byCluster[share.Cluster] = share.Nominal
	}
	small, medium, large := byCluster["small"], byCluster["medium"], byCluster["large"]
	if small.Cmp(medium) > 0 {
		t.Errorf("small (%s) got more than medium (%s)", small.String(), medium.String())
	}
	if medium.Cmp(large) > 0 {
		t.Errorf("medium (%s) got more than large (%s)", medium.String(), large.String())
	}
}
