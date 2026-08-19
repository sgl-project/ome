package utils

import (
	"math"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func ptrInt(i int32) *intstr.IntOrString {
	out := intstr.FromInt32(i)
	return &out
}

func TestScaledCountFromIntOrString(t *testing.T) {
	cases := []struct {
		name    string
		v       *intstr.IntOrString
		total   int32
		roundUp bool
		want    int32
	}{
		{name: "nil yields zero", v: nil, total: 10, want: 0},
		{name: "integer literal is returned as-is", v: ptrInt(3), total: 10, want: 3},
		{name: "integer above total is not clamped", v: ptrInt(25), total: 10, want: 25},
		{name: "negative integer floors at zero", v: ptrInt(-1), total: 10, want: 0},
		{name: "percent rounds up when roundUp", v: PtrIntOrStringFromString("25%"), total: 10, roundUp: true, want: 3},
		{name: "percent rounds down when not roundUp", v: PtrIntOrStringFromString("25%"), total: 10, want: 2},
		{name: "percent over 100 clamps to total", v: PtrIntOrStringFromString("150%"), total: 10, roundUp: true, want: 10},
		{name: "malformed string yields zero", v: PtrIntOrStringFromString("bogus"), total: 10, want: 0},
		{
			// Regression: the clamp compares in int. Narrowing to int32
			// first would wrap this to a negative value, bypass the clamp,
			// and return a negative count.
			name:    "percent overflowing int32 clamps to total",
			v:       PtrIntOrStringFromString("99999999999%"),
			total:   math.MaxInt32,
			roundUp: true,
			want:    math.MaxInt32,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScaledCountFromIntOrString(tc.v, tc.total, tc.roundUp)
			if got != tc.want {
				t.Errorf("ScaledCountFromIntOrString(%v, %d, %t) = %d; want %d", tc.v, tc.total, tc.roundUp, got, tc.want)
			}
			if got < 0 {
				t.Errorf("count must never be negative; got %d", got)
			}
		})
	}
}

func TestAvailabilityFloor(t *testing.T) {
	cases := []struct {
		name                   string
		total                  int32
		minAvailable           *intstr.IntOrString
		maxUnavailable         *intstr.IntOrString
		fallbackMaxUnavailable *intstr.IntOrString
		want                   int32
	}{
		{name: "non-positive total yields zero", total: 0, want: 0},
		{name: "no budget is strict", total: 10, want: 10},
		{name: "minAvailable takes precedence", total: 10, minAvailable: ptrInt(4), maxUnavailable: ptrInt(9), want: 4},
		{name: "minAvailable above total clamps to total", total: 10, minAvailable: ptrInt(25), want: 10},
		{name: "minAvailable percent resolves via ceil", total: 10, minAvailable: PtrIntOrStringFromString("25%"), want: 3},
		{name: "maxUnavailable subtracts from total", total: 10, maxUnavailable: ptrInt(3), want: 7},
		{name: "maxUnavailable at total floors at zero", total: 10, maxUnavailable: ptrInt(10), want: 0},
		{name: "maxUnavailable 100 percent floors at zero", total: 10, maxUnavailable: PtrIntOrStringFromString("100%"), want: 0},
		{name: "fallback used only when maxUnavailable is nil", total: 10, fallbackMaxUnavailable: ptrInt(2), want: 8},
		{name: "maxUnavailable wins over fallback", total: 10, maxUnavailable: ptrInt(3), fallbackMaxUnavailable: ptrInt(9), want: 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailabilityFloor(tc.total, tc.minAvailable, tc.maxUnavailable, tc.fallbackMaxUnavailable)
			if got != tc.want {
				t.Errorf("AvailabilityFloor(%d, %v, %v, %v) = %d; want %d",
					tc.total, tc.minAvailable, tc.maxUnavailable, tc.fallbackMaxUnavailable, got, tc.want)
			}
		})
	}
}
