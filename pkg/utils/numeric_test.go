package utils

import "testing"

func TestDerefInt32(t *testing.T) {
	if got := DerefInt32(nil); got != 0 {
		t.Fatalf("DerefInt32(nil) = %d, want 0", got)
	}
	v := int32(7)
	if got := DerefInt32(&v); got != 7 {
		t.Fatalf("DerefInt32(&7) = %d, want 7", got)
	}
	z := int32(0)
	if got := DerefInt32(&z); got != 0 {
		t.Fatalf("DerefInt32(&0) = %d, want 0", got)
	}
	n := int32(-3)
	if got := DerefInt32(&n); got != -3 {
		t.Fatalf("DerefInt32(&-3) = %d, want -3", got)
	}
}

func TestClampInt32(t *testing.T) {
	tests := []struct {
		name      string
		v, lo, hi int32
		want      int32
	}{
		{"within", 5, 0, 10, 5},
		{"below lo", -1, 0, 10, 0},
		{"above hi", 12, 0, 10, 10},
		{"at lo", 0, 0, 10, 0},
		{"at hi", 10, 0, 10, 10},
		{"negative range", -5, -10, -1, -5},
		{"clamp to negative hi", 0, -10, -1, -1},
		// canary's clamp(v, max) == ClampInt32(v, 0, max)
		{"canary clamp negative", -2, 0, 4, 0},
		{"canary clamp over", 9, 0, 4, 4},
		// degenerate lo > hi: hi wins (applied last).
		{"lo>hi hi wins", 5, 10, 3, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClampInt32(tc.v, tc.lo, tc.hi); got != tc.want {
				t.Fatalf("ClampInt32(%d,%d,%d) = %d, want %d", tc.v, tc.lo, tc.hi, got, tc.want)
			}
		})
	}
}
