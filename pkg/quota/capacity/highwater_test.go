package capacity

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTrack(t *testing.T) {
	tests := []struct {
		name     string
		mark     string
		observed string
		band     int32
		want     string
	}{
		{
			// New hardware is usable the moment it reports; there is nothing to
			// be cautious about on the way up.
			name: "growth is believed immediately", mark: "100", observed: "120", band: 10, want: "120",
		},
		{
			name: "an unchanged observation holds the mark", mark: "100", observed: "100", band: 10, want: "100",
		},
		{
			// A drained node or a restarting device plugin. Following it down
			// would Degrade budgets every time a rack was patched.
			name: "a dip inside the band is ignored", mark: "100", observed: "95", band: 10, want: "100",
		},
		{
			// The band is inclusive, so the boundary is quiet rather than
			// flapping between two answers on a rounding difference.
			name: "a dip exactly at the band is ignored", mark: "100", observed: "90", band: 10, want: "100",
		},
		{
			// A decommission. A fleet that genuinely shrank has to stop
			// claiming capacity it no longer has.
			name: "a drop past the band is believed", mark: "100", observed: "89", band: 10, want: "89",
		},
		{
			name: "capacity going to zero is believed", mark: "100", observed: "0", band: 10, want: "0",
		},
		{
			// Absent config disables damping rather than assuming a band.
			name: "a zero band disables damping", mark: "100", observed: "99", band: 0, want: "99",
		},
		{
			name: "a negative band disables damping", mark: "100", observed: "99", band: -5, want: "99",
		},
		{
			// The reading the number invites. Treating it as "no damping" would
			// invert the knob at its extreme.
			name: "a band of 100 never lowers the mark", mark: "100", observed: "1", band: 100, want: "100",
		},
		{
			name: "a band above 100 also never lowers", mark: "100", observed: "1", band: 250, want: "100",
		},
		{
			// Growth is still believed at any band: the mark is a floor on what
			// was installed, not a ceiling on what can be.
			name: "a band of 100 still follows growth", mark: "100", observed: "150", band: 100, want: "150",
		},
		{
			name: "the first observation sets the mark", mark: "0", observed: "64", band: 10, want: "64",
		},
		{
			// Accelerator counts are small integers, so the band has to work at
			// a granularity where one chip matters.
			name: "one chip lost from eight is inside a 25% band", mark: "8", observed: "7", band: 25, want: "8",
		},
		{
			name: "three chips lost from eight is past a 25% band", mark: "8", observed: "5", band: 25, want: "5",
		},
		{
			// Whole-fleet numbers, where an int64 milli-scale intermediate
			// would be a real overflow risk.
			name: "very large quantities compare exactly", mark: "1e30", observed: "1e29", band: 10, want: "1e29",
		},
		{
			name: "fractional quantities compare exactly", mark: "1500m", observed: "1450m", band: 10, want: "1500m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Track(qty(tc.mark), qty(tc.observed), tc.band)
			if want := qty(tc.want); got.Cmp(want) != 0 {
				t.Errorf("Track(%s, %s, %d) = %s, want %s",
					tc.mark, tc.observed, tc.band, got.String(), want.String())
			}
		})
	}
}

func TestTrackAll(t *testing.T) {
	mark := func(res, flavor, alloc, hwm string) Mark {
		return Mark{
			ResourceName: res, ResourceFlavor: flavor,
			Allocatable: qty(alloc), HighWaterMark: qty(hwm),
		}
	}
	capacityOf := func(res, flavor, alloc string) Capacity {
		return Capacity{ResourceName: res, ResourceFlavor: flavor, Allocatable: qty(alloc)}
	}
	// parked models a cordoned or NotReady node: out of allocatable, still in
	// the rack, and therefore still counted by the mark.
	parked := func(res, flavor, alloc, unavailable string) Capacity {
		return Capacity{
			ResourceName: res, ResourceFlavor: flavor,
			Allocatable: qty(alloc), Unavailable: qty(unavailable),
		}
	}

	tests := []struct {
		name     string
		previous []Mark
		observed []Capacity
		band     int32
		want     []Mark
	}{
		{
			name:     "a first reading sets every mark",
			observed: []Capacity{capacityOf("google.com/tpu", "tpu7x", "128")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
		},
		{
			name:     "a dip inside the band keeps the mark and reports the dip",
			previous: []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
			observed: []Capacity{capacityOf("google.com/tpu", "tpu7x", "120")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "120", "128")},
		},
		{
			// The case the whole mechanism exists for. Draining half a fleet
			// moves allocatable far past any sane band, so a mark tracking
			// allocatable would follow it down and Degrade every budget during
			// routine maintenance. Installed capacity is unchanged — the
			// hardware is still in the rack — so the mark does not move.
			name:     "a cordon does not move the mark, however large",
			previous: []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
			observed: []Capacity{parked("google.com/tpu", "tpu7x", "64", "64")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "64", "128")},
		},
		{
			// Whereas hardware physically leaving does move it: installed falls
			// too, and past the band.
			name:     "hardware leaving the fleet does move the mark",
			previous: []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
			observed: []Capacity{capacityOf("google.com/tpu", "tpu7x", "64")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "64", "64")},
		},
		{
			// A pair whose mark has decayed to nothing is dropped rather than
			// carried forever: a renamed flavor would otherwise leave a dead
			// entry in status for the life of the cluster.
			name:     "a pair with a zero mark is pruned",
			previous: []Mark{mark("google.com/tpu", "tpu7x-old", "0", "0")},
			observed: []Capacity{capacityOf("google.com/tpu", "tpu7x", "8")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "8", "8")},
		},
		{
			// The case the mark exists for. Losing the entry would erase the
			// only record that the pair was ever larger, so the fleet would
			// look like it never had the hardware rather than like it lost it.
			name:     "a pair that stops reporting keeps its mark at zero allocatable",
			previous: []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
			observed: nil,
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "0", "128")},
		},
		{
			name: "pairs are tracked independently",
			previous: []Mark{
				mark("google.com/tpu", "tpu7x", "128", "128"),
				mark("nvidia.com/gpu", "gb300", "64", "64"),
			},
			observed: []Capacity{
				capacityOf("google.com/tpu", "tpu7x", "120"), // inside the band
				capacityOf("nvidia.com/gpu", "gb300", "8"),   // past it
			},
			band: 10,
			want: []Mark{
				mark("google.com/tpu", "tpu7x", "120", "128"),
				mark("nvidia.com/gpu", "gb300", "8", "8"),
			},
		},
		{
			name:     "a newly appearing pair is added",
			previous: []Mark{mark("google.com/tpu", "tpu7x", "128", "128")},
			observed: []Capacity{
				capacityOf("nvidia.com/gpu", "gb300", "8"),
				capacityOf("google.com/tpu", "tpu7x", "128"),
			},
			band: 10,
			want: []Mark{
				mark("google.com/tpu", "tpu7x", "128", "128"),
				mark("nvidia.com/gpu", "gb300", "8", "8"),
			},
		},
		{
			// Output order must not depend on input order, or a status write
			// would churn on every pass.
			name: "output is sorted regardless of input order",
			observed: []Capacity{
				capacityOf("nvidia.com/gpu", "gb300", "8"),
				capacityOf("google.com/tpu", "tpu7x", "4"),
				capacityOf("google.com/tpu", "tpu6e", "2"),
			},
			band: 10,
			want: []Mark{
				mark("google.com/tpu", "tpu6e", "2", "2"),
				mark("google.com/tpu", "tpu7x", "4", "4"),
				mark("nvidia.com/gpu", "gb300", "8", "8"),
			},
		},
		{
			name: "nothing observed and nothing remembered is empty",
			band: 10,
			want: []Mark{},
		},
		{
			// The CRD's listMapKey stops a duplicate arriving through the API,
			// but this takes a plain slice, so first-wins keeps it total and
			// deterministic rather than letting map order decide.
			name: "a duplicated previous entry takes the first",
			previous: []Mark{
				mark("google.com/tpu", "tpu7x", "128", "128"),
				mark("google.com/tpu", "tpu7x", "64", "64"),
			},
			observed: []Capacity{capacityOf("google.com/tpu", "tpu7x", "120")},
			band:     10,
			want:     []Mark{mark("google.com/tpu", "tpu7x", "120", "128")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TrackAll(tc.previous, tc.observed, tc.band)
			if diff := cmp.Diff(tc.want, got, cmp.Comparer(quantityEqual)); diff != "" {
				t.Errorf("TrackAll() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
