package topology

import "testing"

// TestBestFit pins the packing policy: among domains that can hold the whole
// gang, pick the FULLEST — the one with the fewest free nodes — so
// already-busy domains fill up and empty domains stay whole for future large
// gangs. Ties break on domain name for determinism.
func TestBestFit(t *testing.T) {
	cases := []struct {
		name string
		free FreeByDomain
		gang int
		want string
		ok   bool
	}{
		{"no domains", FreeByDomain{}, 4, "", false},
		{"none fit", FreeByDomain{"a": 2, "b": 3}, 4, "", false},
		{"single fit", FreeByDomain{"a": 8}, 4, "a", true},
		{"fullest that fits wins", FreeByDomain{"a": 18, "b": 6, "c": 4}, 4, "c", true},
		{"skip too-small, pick fullest fitting", FreeByDomain{"a": 18, "b": 5, "c": 2}, 4, "b", true},
		{"exact fit preferred over roomy", FreeByDomain{"big": 18, "exact": 4}, 4, "exact", true},
		{"tie breaks on domain name", FreeByDomain{"z": 6, "a": 6, "m": 6}, 4, "a", true},
		{"gang of one", FreeByDomain{"a": 3, "b": 1}, 1, "b", true},
		{"whole-domain gang", FreeByDomain{"a": 18}, 18, "a", true},
		{"invalid gang size zero", FreeByDomain{"a": 8}, 0, "", false},
		{"invalid gang size negative", FreeByDomain{"a": 8}, -3, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := BestFit(tc.free, tc.gang)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("BestFit(%v, %d) = (%q, %v), want (%q, %v)",
					tc.free, tc.gang, got, ok, tc.want, tc.ok)
			}
		})
	}
}
