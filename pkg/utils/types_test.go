package utils

import "testing"

func TestPtrStrEqual_UnsetEmptyOrSetPointers_TrueOnlyWhenBothUnsetOrEqual(t *testing.T) {
	s := func(v string) *string { return &v }
	tests := []struct {
		name string
		a, b *string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil b set", nil, s(""), false},
		{"a set b nil", s(""), nil, false},
		{"both empty", s(""), s(""), true},
		{"equal values", s("x"), s("x"), true},
		{"different values", s("x"), s("y"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PtrStrEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("PtrStrEqual = %v, want %v", got, tc.want)
			}
		})
	}
}
