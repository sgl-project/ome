package coordination

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// marksOf renders a plan's per-batch counts, each side's cumulative marks
// turned back into batch sizes, as "2,2,1 / 1,1,1".
func marksOf(prefill, decode []int32) string {
	sizes := func(marks []int32) string {
		out := make([]string, 0, len(marks))
		var prev int32
		for _, m := range marks {
			out = append(out, fmt.Sprintf("%d", m-prev))
			prev = m
		}
		return strings.Join(out, ",")
	}
	return sizes(prefill) + " / " + sizes(decode)
}

// worstGap is the largest gap between the two sides' shares over a plan's
// pictures, in decode Instances.
func worstGap(prefill, decode int32, prefillMarks, decodeMarks []int32) float64 {
	var worst float64
	for i := range prefillMarks {
		gap := float64(prefillMarks[i])/float64(prefill) - float64(decodeMarks[i])/float64(decode)
		if gap < 0 {
			gap = -gap
		}
		if gap*float64(decode) > worst {
			worst = gap * float64(decode)
		}
	}
	return worst
}

// TestPairedBatchPlan_WorkedShapes pins the batches the ratio rule plans
// for the reference shapes. 12 prefill beside 8 decode at
// 10 percent budgets and a 10 percent tolerance gives prefill batches
// 2,2,1,2,1,2,1,1 beside decode batches of 1, the picture reading 2:1,
// 4:2, 5:3, 7:4, 8:5, 10:6, 11:7, 12:8; at the chart's default tolerance
// of 5 the prefill side alternates 2,1. Where both steps already follow
// the ratio (8:4 at 25 percent) nothing changes. Where the ratio needs a
// step above a budget (12:4 at one each) the prefill side keeps stepping
// and the decode side pauses on the batches the ratio does not yet need.
// A tolerance of 100 is the opt-out: both sides step their full budgets.
// A tolerance of 0 asks for the picture closest to the line every batch,
// the larger batch breaking ties, and still moves at least one Instance
// per batch. With 12 prefill beside 2 decode each decode Instance is worth
// six prefill, so the decode side moves only at the points the prefill
// share reaches, and once it has finished the prefill side steps its full
// budget. Every plan ends with both sides at their desired counts, and the
// gap it keeps is the tolerance, not a fixed number of Instances: 0.67
// decode Instances off the line at 10, 0.33 at 5, 1.33 at the reference
// tolerance of 20 (2:1, 4:2, 6:3, 8:4, 9:5, 11:6, 12:7, 12:8, two
// to one for four batches), and the full 2.0 of plain steps at 100.
func TestPairedBatchPlan_WorkedShapes(t *testing.T) {
	cases := []struct {
		name                    string
		prefill, decode         int32
		prefillStep, decodeStep int32
		tolerance               int32
		wantBatches             string
		wantPictures            string
		wantGap                 float64 // the worst gap, in decode Instances
	}{
		{
			name: "12:8 at 10 percent, tolerance 10", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 1, tolerance: 10,
			wantBatches: "2,2,1,2,1,2,1,1 / 1,1,1,1,1,1,1,1", wantPictures: "2:1 4:2 5:3 7:4 8:5 10:6 11:7 12:8", wantGap: 0.67,
		},
		{
			name: "12:8 at 10 percent, tolerance 5", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 1, tolerance: 5,
			wantBatches: "2,1,2,1,2,1,2,1 / 1,1,1,1,1,1,1,1", wantPictures: "2:1 3:2 5:3 6:4 8:5 9:6 11:7 12:8", wantGap: 0.33,
		},
		{
			name: "12:8 at 10 percent, tolerance 20", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 1, tolerance: 20,
			wantBatches: "2,2,2,2,1,2,1,0 / 1,1,1,1,1,1,1,1", wantPictures: "2:1 4:2 6:3 8:4 9:5 11:6 12:7 12:8", wantGap: 1.33,
		},
		{
			name: "8:4 at 25 percent keeps its steps", prefill: 8, decode: 4, prefillStep: 2, decodeStep: 1, tolerance: 5,
			wantBatches: "2,2,2,2 / 1,1,1,1", wantPictures: "2:1 4:2 6:3 8:4", wantGap: 0,
		},
		{
			name: "12:4 at one each: the decode side pauses", prefill: 12, decode: 4, prefillStep: 1, decodeStep: 1, tolerance: 5,
			wantBatches: "1,1,1,1,1,1,1,1,1,1,1,1 / 0,1,0,0,1,0,0,1,0,0,1,0", wantPictures: "1:0 2:1 3:1 4:1 5:2 6:2 7:2 8:3 9:3 10:3 11:4 12:4", wantGap: 0.33,
		},
		{
			name: "tolerance 100 is the opt-out", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 1, tolerance: 100,
			wantBatches: "2,2,2,2,2,2,0,0 / 1,1,1,1,1,1,1,1", wantPictures: "2:1 4:2 6:3 8:4 10:5 12:6 12:7 12:8", wantGap: 2,
		},
		{
			name: "tolerance 0 hugs the line and never stalls", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 1, tolerance: 0,
			wantBatches: "2,1,2,1,2,1,2,1 / 1,1,1,1,1,1,1,1", wantPictures: "2:1 3:2 5:3 6:4 8:5 9:6 11:7 12:8", wantGap: 0.33,
		},
		{
			name: "12:2 at 2 and 1: the decode side moves as the prefill share reaches its points", prefill: 12, decode: 2, prefillStep: 2, decodeStep: 1, tolerance: 10,
			wantBatches: "1,1,2,2,1,1,2,2 / 0,0,1,0,0,0,1,0", wantPictures: "1:0 2:0 4:1 6:1 7:1 8:1 10:2 12:2", wantGap: 0.33,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefill, decode := pairedBatchPlan(tc.prefill, tc.decode, tc.prefillStep, tc.decodeStep, tc.tolerance)
			if got := marksOf(prefill, decode); got != tc.wantBatches {
				t.Errorf("batches\n got %s\nwant %s", got, tc.wantBatches)
			}
			ps := make([]string, 0, len(prefill))
			for i := range prefill {
				ps = append(ps, fmt.Sprintf("%d:%d", prefill[i], decode[i]))
			}
			if got := strings.Join(ps, " "); got != tc.wantPictures {
				t.Errorf("pictures\n got %s\nwant %s", got, tc.wantPictures)
			}
			if prefill[len(prefill)-1] != tc.prefill || decode[len(decode)-1] != tc.decode {
				t.Errorf("the plan ends at %d:%d, want %d:%d", prefill[len(prefill)-1], decode[len(decode)-1], tc.prefill, tc.decode)
			}
			if gap := worstGap(tc.prefill, tc.decode, prefill, decode); math.Abs(gap-tc.wantGap) > 0.01 {
				t.Errorf("worst gap %.2f decode Instances off the line, want %.2f", gap, tc.wantGap)
			}
		})
	}
}

// TestPairedBatchPlan_PlansNothingWithoutASteppingSide pins that a side
// with no step, or with nothing to roll, leaves both sides on their own
// steps: the plan is empty rather than a stall.
func TestPairedBatchPlan_PlansNothingWithoutASteppingSide(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		prefill, decode         int32
		prefillStep, decodeStep int32
	}{
		{name: "prefill step zero", prefill: 12, decode: 8, prefillStep: 0, decodeStep: 1},
		{name: "decode step zero", prefill: 12, decode: 8, prefillStep: 2, decodeStep: 0},
		{name: "no decode", prefill: 12, decode: 0, prefillStep: 2, decodeStep: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p, d := pairedBatchPlan(tc.prefill, tc.decode, tc.prefillStep, tc.decodeStep, 10); p != nil || d != nil {
				t.Fatalf("planned %v / %v, want no plan", p, d)
			}
		})
	}
}

// TestFitBatch_TakesTheLargestBatchWithinTheTolerance pins the choice on
// one picture: from 4:2 of 12:8 with full steps 2 and 1, the full batch
// reads 6:3 and sits 12.5 points off, so at tolerance 10 the batch shrinks
// to 1 and 1 (5:3, 4 points off); at tolerance 13 the full batch fits and
// is taken; at tolerance 3 nothing fits and the batch closest to the line,
// 1 and 1, is taken rather than the smallest step.
func TestFitBatch_TakesTheLargestBatchWithinTheTolerance(t *testing.T) {
	for _, tc := range []struct {
		tolerance    int32
		wantP, wantD int32
	}{
		{tolerance: 10, wantP: 1, wantD: 1},
		{tolerance: 13, wantP: 2, wantD: 1},
		{tolerance: 3, wantP: 1, wantD: 1},
	} {
		p, d := fitBatch(12, 8, 4, 2, 2, 1, tc.tolerance)
		if p != tc.wantP || d != tc.wantD {
			t.Errorf("tolerance %d: batch %d and %d, want %d and %d", tc.tolerance, p, d, tc.wantP, tc.wantD)
		}
	}
}
